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

func (p *CommandPool[A, B, C, D]) GetInWork() int64 {
	return p.inWork.Load()
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

func (p *CommandPool[A, B, C, D]) Stop() error {
	if !p.isRun() {
		return errors.New("pool is not running")
	}

	for {
		currentTask, _ := p.taskAtom.Load().(chan Command[C, D])

		if currentTask == nil {
			return errors.New("pool is stopped")
		}

		time.Sleep(100 * time.Millisecond)
		fmt.Println("waiting for pool to stop")
		if p.inWork.Load() == 0 && currentTask != nil && len(currentTask) == 0 {
			p.poolCancel()

			if currentTask != nil {
				close(currentTask)
				p.taskAtom.Store((chan Command[C, D])(nil))
			}

			p.wg.Wait()
			fmt.Println("pool stopped")
			p.poolCancel = nil
			p.poolCtx = nil
			p.inWork.Store(0)

			p.runAtom.Store(false)
			return nil
		}
	}
}

func (p *CommandPool[A, B, C, D]) isRun() bool {
	return p.runAtom.Load()
}

func (p *CommandPool[A, B, C, D]) Reconnect(ctx context.Context) error {
	if p.isRun() {
		return errors.New("pool is already running")
	}

	fmt.Println("before reconnect")
	poolCtx, cancel := context.WithCancel(ctx)
	p.poolCancel = cancel
	p.poolCtx = poolCtx
	p.taskAtom.Store(make(chan Command[C, D], p.size))

	p.Run(ctx)
	fmt.Println("pool reconnected")
	return nil
}

func (p *CommandPool[A, B, C, D]) Shutdown() {
	if err := p.Stop(); err != nil {
		fmt.Println(err)
	}
}

func (p *CommandPool[A, B, C, D]) Factory(command B, key C, value D) Command[C, D] {
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

func (a *CommandPool[A, B, C, D]) Run(ctx context.Context) {
	if a.isRun() {
		return
	}

	currentTask, _ := a.taskAtom.Load().(chan Command[C, D])

	for i := 1; i <= cap(currentTask); i++ {
		a.wg.Add(1)
		go func() {
			defer a.wg.Done()
			defer func() {
				if r := recover(); r != nil {
					fmt.Println("recovered in Start", r)
				}
			}()
			for {
				current, _ := a.taskAtom.Load().(chan Command[C, D])

				if current == nil {
					time.Sleep(100 * time.Millisecond)
					fmt.Println("worker", "waiting (pool frozen)")
					if a.poolCtx.Err() != nil {
						fmt.Println("worker pool context done")
						return
					}
					if ctx.Err() != nil {
						fmt.Println("worker main context done")
						return
					}
					continue
				}

				select {
				case <-a.poolCtx.Done():
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
					a.inWork.Add(1)
					command.Apply(ctx, a.store, command.GetKey(), command.GetValue())
					a.inWork.Add(-1)
					fmt.Println("done task", command.GetKey(), command.GetValue())
				}
			}
		}()
	}

	a.runAtom.Store(true)
	fmt.Println("pool started")
}

func (p *CommandPool[A, B, C, D]) Apply(ctx A, command B, key C, value D) error {
	if !p.isRun() {
		return errors.New("pool is not running")
	}

	currentTask, _ := p.taskAtom.Load().(chan Command[C, D])

	if currentTask == nil {
		fmt.Println("Cannot apply command, pool is stopped (frozen)")
		return errors.New("pool is stopped (frozen)")
	}

	cmd := p.Factory(command, key, value)
	select {
	case <-p.poolCtx.Done():
		fmt.Println("in send pool context done")
		return p.poolCtx.Err()
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
