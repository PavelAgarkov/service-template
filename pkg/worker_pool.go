package pkg

import (
	"context"
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
	store  Store
	inWork atomic.Int64
	taskMu sync.Mutex
	task   chan Command[C, D]
	size   int
}

func (p *CommandPool[A, B, C, D]) GetInWork() int64 {
	return p.inWork.Load()
}

func NewPoolStore[A context.Context, B Integer, C String, D String](store Store, size int) *CommandPool[A, B, C, D] {
	return &CommandPool[A, B, C, D]{
		store: store,
		task:  make(chan Command[C, D], size),
		size:  size,
	}
}

func (p *CommandPool[A, B, C, D]) Stop() int64 {
	if p.inWork.Load() == 0 && p.task != nil {
		p.taskMu.Lock()
		defer p.taskMu.Unlock()
		p.task = nil
		fmt.Println("pool stopped")
		return 0
	}
	return p.inWork.Load()
}

func (p *CommandPool[A, B, C, D]) Shutdown() {
	p.Stop()
}

func (p *CommandPool[A, B, C, D]) Reconnect() {
	fmt.Println("before reconnect", len(p.task), cap(p.task))
	p.taskMu.Lock()
	defer p.taskMu.Unlock()
	p.task = make(chan Command[C, D], p.size)
	fmt.Println("pool reconnected", len(p.task), cap(p.task))
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
				case <-ctx.Done():
					fmt.Println("worker pool context done")
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
				case <-t.C:
					// Если за 100 мс не пришла задача, выйдем из select,
					// чтобы на следующей итерации цикла заново взять актуальное значение p.task.
					// Это нужно, чтобы не блокировать пул, если он остановлен.
					// Если пул остановлен, то p.task == nil и воркер будет ждать, пока пул не будет перезапущен.
					// Если пул запущен, то p.task != nil и воркер будет работать.
					// Если воркер не получил задачу, то он снова попадёт в этот блок и будет ждать новую итерацию.
					// Если воркер получил задачу, то он выполнит её и снова попадёт в этот блок.
					// Это необходимо для реалищации размерзая очереди задач в канале p.task, который может быть остановлен из вне и переназначен также извне.
					// При этом воркеры не должны блокировать пул, если он остановлен.

					fmt.Println("worker", "waiting (no task)")
					continue
				}
			}
		}()
	}
	fmt.Println("pool started")
}

func (p *CommandPool[A, B, C, D]) Apply(ctx A, command B, key C, value D) {
	p.taskMu.Lock()
	currentTask := p.task
	p.taskMu.Unlock()

	if currentTask == nil {
		fmt.Println("Cannot apply command, pool is stopped (frozen)")
		return
	}

	go func() {
		cmd := p.Factory(command, key, value)
		select {
		case <-ctx.Done():
			fmt.Println("in send context done")
			return
		case currentTask <- cmd:
		}
	}()
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
