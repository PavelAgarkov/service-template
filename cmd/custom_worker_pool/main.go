package main

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

type Store interface {
	Put(key, value string)
	Get(key string) string
	Del(key string)
}

type MemStore struct {
	rwMu sync.RWMutex
	data map[string]string
}

func NewMemStore() *MemStore {
	return &MemStore{data: make(map[string]string)}
}

func (m *MemStore) Put(key, value string) {
	m.rwMu.Lock()
	defer m.rwMu.Unlock()
	m.data[key] = value
}

func (m *MemStore) Get(key string) string {
	m.rwMu.RLock()
	defer m.rwMu.RUnlock()
	return m.data[key]
}

func (m *MemStore) Del(key string) {
	m.rwMu.Lock()
	defer m.rwMu.Unlock()
	delete(m.data, key)
}

type Command[C, D String] interface {
	Apply(ctx context.Context, store Store, key C, value D)
}

type PutCommand[C, D String] struct{}

func NewPutCommand[C, D String]() *PutCommand[C, D] {
	return &PutCommand[C, D]{}
}

func (p *PutCommand[C, D]) Apply(ctx context.Context, store Store, key C, value D) {
	store.Put(string(key), string(value))
	fmt.Println("put", key, ":", value)
}

type GetCommand[C, D String] struct{}

func NewGetCommand[C, D String]() *GetCommand[C, D] {
	return &GetCommand[C, D]{}
}

func (g *GetCommand[C, D]) Apply(ctx context.Context, store Store, key C, value D) {
	res := store.Get(string(key))
	fmt.Println("get", key, ":", res)
}

type DelCommand[C, D String] struct{}

func NewDelCommand[C, D String]() *DelCommand[C, D] {
	return &DelCommand[C, D]{}
}

func (d *DelCommand[C, D]) Apply(ctx context.Context, store Store, key C, value D) {
	store.Del(string(key))
	fmt.Println("del", key)
}

type Integer interface {
	int | int8 | int16 | int32 | int64 | uint | uint8 | uint16 | uint32 | uint64
}

type String interface {
	string
}

type CommandPool[A context.Context, B Integer, C, D String] struct {
	store  Store
	pool   chan struct{}
	inWork atomic.Int64
}

func NewPoolStore[A context.Context, B Integer, C String, D String](store Store, size int) *CommandPool[A, B, C, D] {
	return &CommandPool[A, B, C, D]{store: store, pool: make(chan struct{}, size)}
}

func (p *CommandPool[A, B, C, D]) Stop() int64 {
	if p.inWork.Load() == 0 && p.pool != nil && len(p.pool) == 0 {
		p.pool = nil
		fmt.Println("pool stopped")
		return 0
	}
	return p.inWork.Load()
}

func (p *CommandPool[A, B, C, D]) Reconnect() {
	p.pool = make(chan struct{}, 2)
}

func (p *CommandPool[A, B, C, D]) Factory(command B) Command[C, D] {
	switch command {
	case 0:
		//noinspection GoVetImpossibleInterfaceToInterfaceAssertionInspection
		//noinspection GoInterfaceToAnyInspection
		//noinspection GoTypeAssertionOnNonInterfaceType
		//noinspection GoTypeMismatch
		//noinspection GoTypeMismatchInAssignment
		//noinspection GoInvalidTypeAssertion
		return NewPutCommand[C, D]()
	case 1:
		//noinspection GoTypeAssertionOnNonInterfaceType
		return NewGetCommand[C, D]()
	case 2:
		return NewDelCommand[C, D]()
	default:
		// По умолчанию тоже Put
		return NewPutCommand[C, D]()
	}
}

func (p *CommandPool[A, B, C, D]) Apply(ctx A, command B, key C, value D) {
	select {
	case <-ctx.Done():
		fmt.Println("context done")
		return
	case p.pool <- struct{}{}:
		p.inWork.Add(1)
	}
	fmt.Println("pool size:", len(p.pool), cap(p.pool))
	go func() {
		defer func() {
			select {
			case <-ctx.Done():
				fmt.Println("context done")
				p.inWork.Add(-1)
				return
			case <-p.pool:
				p.inWork.Add(-1)
				return
			}
		}()

		cmd := p.Factory(command)
		select {
		case <-ctx.Done():
			fmt.Println("context done")
			return
		default:
			select {
			case <-ctx.Done():
				fmt.Println("context done")
				return
			case <-time.After(700 * time.Millisecond):
				fmt.Println("timeout")
			}
			cmd.Apply(ctx, p.store, key, value)
		}
	}()
}

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	commands := NewPoolStore[context.Context, int, string, string](NewMemStore(), 2)

	keys := []string{"a", "b", "c", "d", "e", "f", "g"}

	for i, key := range keys {
		commands.Apply(ctx, i%3, key, fmt.Sprintf("%s-%d", key, i))
		fmt.Println("inWork:", commands.inWork.Load())
	}

	for {
		active := commands.Stop()
		if active == 0 {
			fmt.Println("pool is empty")
			break
		}
		time.Sleep(100 * time.Millisecond)
		fmt.Println("waiting for pool to be empty")
	}
	commands.Reconnect()

	for i, key := range keys {
		commands.Apply(ctx, i%3, key, fmt.Sprintf("%s-%d", key, i))
		fmt.Println("inWork:", commands.inWork.Load())
	}

	for {
		active := commands.Stop()
		if active == 0 {
			fmt.Println("pool is empty")
			break
		}
		time.Sleep(100 * time.Millisecond)
		fmt.Println("waiting for pool to be empty")
	}
	fmt.Println("inWork:", commands.inWork.Load())
}
