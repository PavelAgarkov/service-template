package pkg

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

type Integer interface {
	int | int8 | int16 | int32 | int64 | uint | uint8 | uint16 | uint32 | uint64
}

type String interface {
	string
}

type CommandPool[A context.Context, B Integer, C, D String] struct {
	store Store
	size  int

	poolCtx    context.Context
	poolCancel context.CancelFunc
	wg         *sync.WaitGroup

	inWork   atomic.Int64
	taskAtom atomic.Value
	runAtom  atomic.Bool
}

func (commandPool *CommandPool[A, B, C, D]) GetInWork() int64 {
	return commandPool.inWork.Load()
}

func NewPoolStore[A context.Context, B Integer, C String, D String](ctx context.Context, store Store, size int) *CommandPool[A, B, C, D] {
	poolCtx, cancel := context.WithCancel(ctx)
	cp := &CommandPool[A, B, C, D]{
		poolCtx:    poolCtx,
		poolCancel: cancel,
		store:      store,
		size:       size,
		wg:         &sync.WaitGroup{},
	}
	cp.taskAtom.Store(make(chan Command[C, D], size))
	return cp
}

func (commandPool *CommandPool[A, B, C, D]) Stop() error {
	if !commandPool.isRun() {
		return errors.New("pool is not running")
	}

	for {
		currentTask, _ := commandPool.taskAtom.Load().(chan Command[C, D])

		if currentTask == nil {
			return errors.New("pool is stopped")
		}

		time.Sleep(100 * time.Millisecond)
		fmt.Println("waiting for pool to stop")
		if commandPool.inWork.Load() == 0 && currentTask != nil && len(currentTask) == 0 {
			commandPool.poolCancel()

			if currentTask != nil {
				close(currentTask)
				commandPool.taskAtom.Store((chan Command[C, D])(nil))
			}

			commandPool.wg.Wait()
			fmt.Println("pool stopped")
			commandPool.poolCancel = nil
			commandPool.poolCtx = nil
			commandPool.inWork.Store(0)

			commandPool.runAtom.Store(false)
			return nil
		}
	}
}

func (commandPool *CommandPool[A, B, C, D]) isRun() bool {
	return commandPool.runAtom.Load()
}

func (commandPool *CommandPool[A, B, C, D]) Reconnect(ctx context.Context, handler func(ctx context.Context, cmd Command[C, D], store Store, key C, value D)) error {
	if commandPool.isRun() {
		return errors.New("pool is already running")
	}

	fmt.Println("before reconnect")
	poolCtx, cancel := context.WithCancel(ctx)
	commandPool.poolCancel = cancel
	commandPool.poolCtx = poolCtx
	commandPool.taskAtom.Store(make(chan Command[C, D], commandPool.size))

	commandPool.Run(ctx, handler)
	fmt.Println("pool reconnected")
	return nil
}

func (commandPool *CommandPool[A, B, C, D]) Shutdown() {
	if err := commandPool.Stop(); err != nil {
		fmt.Println(err)
	}
}

func (commandPool *CommandPool[A, B, C, D]) Factory(command B, key C, value D) Command[C, D] {
	switch command {
	case 0:
		return NewPutCommand[C, D](key, value)
	case 1:
		return NewGetCommand[C, D](key, value)
	case 2:
		return NewDelCommand[C, D](key, value)
	default:
		return NewPutCommand[C, D](key, value)
	}
}

func (commandPool *CommandPool[A, B, C, D]) Run(ctx context.Context, handler func(ctx context.Context, cmd Command[C, D], store Store, key C, value D)) {

	//func (commandPool *CommandPool[A, B, C, D]) Run(ctx context.Context, handler func(context.Context, cmd Command[C, D], Store, C, D)) {
	if commandPool.isRun() {
		return
	}

	currentTask, _ := commandPool.taskAtom.Load().(chan Command[C, D])

	for i := 1; i <= cap(currentTask); i++ {
		commandPool.wg.Add(1)
		go func() {
			defer commandPool.wg.Done()
			defer func() {
				if r := recover(); r != nil {
					fmt.Println("recovered in Start", r)
				}
			}()
			for {
				current, _ := commandPool.taskAtom.Load().(chan Command[C, D])

				if current == nil {
					time.Sleep(100 * time.Millisecond)
					fmt.Println("worker", "waiting (pool frozen)")
					select {
					case <-commandPool.poolCtx.Done():
						fmt.Println("worker pool context done")
						return
					case <-ctx.Done():
						fmt.Println("worker main context done")
						return
					default:
						continue
					}
				}

				select {
				case <-commandPool.poolCtx.Done():
					fmt.Println("worker pool context done")
					return
				case <-ctx.Done():
					fmt.Println("worker main context done")
					return
				case command, ok := <-current:
					if !ok {
						fmt.Println("task channel closed")
						return
					}
					commandPool.inWork.Add(1)
					handler(ctx, command, commandPool.store, command.GetKey(), command.GetValue())
					commandPool.inWork.Add(-1)
					fmt.Println("done task", command.GetKey(), command.GetValue())
				}
			}
		}()
	}

	commandPool.runAtom.Store(true)
	fmt.Println("pool started")
}

func (commandPool *CommandPool[A, B, C, D]) Apply(ctx A, command B, key C, value D) error {
	if !commandPool.isRun() {
		return errors.New("pool is not running")
	}

	currentTask, _ := commandPool.taskAtom.Load().(chan Command[C, D])

	if currentTask == nil {
		fmt.Println("Cannot apply command, pool is stopped (frozen)")
		return errors.New("pool is stopped (frozen)")
	}

	cmd := commandPool.Factory(command, key, value)
	select {
	case <-commandPool.poolCtx.Done():
		fmt.Println("in send pool context done")
		return commandPool.poolCtx.Err()
	case <-ctx.Done():
		fmt.Println("in send context done")
		return ctx.Err()
	default:
		if currentTask != nil {
			currentTask <- cmd
		} else {
			fmt.Println("Cannot apply command, pool is stopped")
			return errors.New("pool is stopped")
		}
	}
	return nil
}

type Command[C, D String] interface {
	Apply(ctx context.Context, store Store, key C, value D)
	GetKey() C
	GetValue() D
}

type BaseCommand[C, D String] struct {
	key   C
	value D
}

func (d *BaseCommand[C, D]) GetKey() C {
	return d.key
}

func (d *BaseCommand[C, D]) GetValue() D {
	return d.value
}

func NewPutCommand[C, D String](key C, value D) *PutCommand[C, D] {
	return &PutCommand[C, D]{
		BaseCommand: BaseCommand[C, D]{key: key, value: value},
	}
}

type PutCommand[C, D String] struct {
	BaseCommand[C, D]
}

func (p *PutCommand[C, D]) Apply(ctx context.Context, store Store, key C, value D) {
	store.Put(string(key), string(value))
	fmt.Println("put", key, ":", value)
}

func NewGetCommand[C, D String](key C, value D) *GetCommand[C, D] {
	return &GetCommand[C, D]{
		BaseCommand: BaseCommand[C, D]{
			key:   key,
			value: value,
		},
	}
}

type GetCommand[C, D String] struct {
	BaseCommand[C, D]
}

func (g *GetCommand[C, D]) Apply(ctx context.Context, store Store, key C, value D) {
	res := store.Get(string(key))
	fmt.Println("get", key, ":", res)
}

func NewDelCommand[C, D String](key C, value D) *DelCommand[C, D] {
	return &DelCommand[C, D]{
		BaseCommand: BaseCommand[C, D]{
			key:   key,
			value: value,
		},
	}
}

type DelCommand[C, D String] struct {
	BaseCommand[C, D]
}

func (d *DelCommand[C, D]) Apply(ctx context.Context, store Store, key C, value D) {
	store.Del(string(key))
	fmt.Println("del", key)
}
