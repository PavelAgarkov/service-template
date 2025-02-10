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
	store      Store
	inWork     atomic.Int64
	taskMu     sync.Mutex
	task       chan Command[C, D]
	size       int
	mainCtx    context.Context
	poolCtx    context.Context
	poolCancel context.CancelFunc
	wg         *sync.WaitGroup
}

func (p *CommandPool[A, B, C, D]) GetInWork() int64 {
	return p.inWork.Load()
}

func NewPoolStore[A context.Context, B Integer, C String, D String](ctx context.Context, store Store, size int) *CommandPool[A, B, C, D] {
	poolCtx, cancel := context.WithCancel(ctx)
	return &CommandPool[A, B, C, D]{
		mainCtx:    ctx,
		poolCtx:    poolCtx,
		poolCancel: cancel,
		store:      store,
		task:       make(chan Command[C, D], size),
		size:       size,
		wg:         &sync.WaitGroup{},
	}
}

func (p *CommandPool[A, B, C, D]) Stop() {
	for p.task != nil {
		time.Sleep(100 * time.Millisecond)
		fmt.Println("waiting for pool to stop")
		if p.inWork.Load() == 0 && p.task != nil && len(p.task) == 0 {
			p.poolCancel()
			close(p.task)
			p.wg.Wait()
			fmt.Println("pool stopped")
			p.poolCancel = nil
			p.poolCtx = nil
			p.inWork.Store(0)
			return
		}
	}
	return
}

func (p *CommandPool[A, B, C, D]) Reconnect() {
	fmt.Println("before reconnect", len(p.task), cap(p.task))
	poolCtx, cancel := context.WithCancel(p.mainCtx)
	p.poolCancel = cancel
	p.poolCtx = poolCtx
	p.task = make(chan Command[C, D], p.size)
	p.Start(p.mainCtx)
	fmt.Println("pool reconnected", len(p.task), cap(p.task))
}

func (p *CommandPool[A, B, C, D]) Shutdown() {
	p.Stop()
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

func (a *CommandPool[A, B, C, D]) Start(ctx context.Context) {
	for i := 1; i <= cap(a.task); i++ {
		go func() {
			a.wg.Add(1)
			defer a.wg.Done()
			t := time.NewTicker(100 * time.Millisecond)
			defer t.Stop()
			defer func() {
				if r := recover(); r != nil {
					fmt.Println("recovered in f", r)
				}
			}()
			for {
				a.taskMu.Lock()
				current := a.task
				a.taskMu.Unlock()

				// Если канал равен nil (пул остановлен), ждём и повторяем проверку
				if current == nil {
					time.Sleep(100 * time.Millisecond)
					fmt.Println("worker", "waiting (pool frozen)")
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
	fmt.Println("pool started")
}

func (p *CommandPool[A, B, C, D]) Apply(ctx A, command B, key C, value D) error {
	p.taskMu.Lock()
	currentTask := p.task
	p.taskMu.Unlock()

	if currentTask == nil {
		return errors.New("pool is stopped (frozen)")
	}
	go func(currentTask chan Command[C, D]) {
		defer func() {
			if r := recover(); r != nil {
				fmt.Println("recovered in Apply", r)
			}
		}()

		if currentTask == nil {
			fmt.Println("Cannot apply command, pool is stopped (frozen)")
			return
		}
		cmd := p.Factory(command, key, value)
		select {
		case <-p.poolCtx.Done():
			fmt.Println("in send pool context done")
			return
		case <-ctx.Done():
			fmt.Println("in send context done")
			return
		default:
			if currentTask != nil {
				currentTask <- cmd
			} else {
				fmt.Println("Cannot apply command, pool is stopped")
			}
		}
	}(currentTask)
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
