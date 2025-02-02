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

type Command interface {
	Apply(ctx context.Context, store Store, key, value string)
}

type PutCommand struct{}

func (p *PutCommand) Apply(ctx context.Context, store Store, key, value string) {
	store.Put(key, value)
	fmt.Println("put", key, ":", value)
}

type GetCommand struct{}

func (g *GetCommand) Apply(ctx context.Context, store Store, key, value string) {
	res := store.Get(key)
	fmt.Println("get", key, ":", res)
}

type DelCommand struct{}

func (d *DelCommand) Apply(ctx context.Context, store Store, key, value string) {
	store.Del(key)
	fmt.Println("del", key)
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

type CommandPool struct {
	store  Store
	pool   chan struct{}
	inWork atomic.Int64
}

func NewPoolStore(store Store) *CommandPool {
	return &CommandPool{store: store, pool: make(chan struct{}, 2)}
}

func (p *CommandPool) Stop() int64 {
	if p.inWork.Load() == 0 && p.pool != nil && len(p.pool) == 0 {
		p.pool = nil
		fmt.Println("pool stopped")
		return 0
	}
	return p.inWork.Load()
}

func (p *CommandPool) Reconnect() {
	p.pool = make(chan struct{}, 2)
}

func (p *CommandPool) Factory(command int) Command {
	switch command {
	case 0:
		return &PutCommand{}
	case 1:
		return &GetCommand{}
	case 2:
		return &DelCommand{}
	default:
		return &PutCommand{}
	}
}

func (p *CommandPool) Apply(ctx context.Context, command int, key, value string) {
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
	commands := NewPoolStore(NewMemStore())

	keys := []string{"a", "b", "c", "d", "e", "f", "g"}

	for i, key := range keys {
		commands.Apply(ctx, i%3, key, fmt.Sprintf("%s-%d", key, i))
		fmt.Println("inWork:", commands.inWork.Load())
	}

	for {
		if len(commands.pool) == 0 {
			active := commands.Stop()
			if active == 0 {
				fmt.Println("pool is empty")
				break
			}
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
		if len(commands.pool) == 0 {
			active := commands.Stop()
			if active == 0 {
				fmt.Println("pool is empty")
				break
			}
		}
		time.Sleep(100 * time.Millisecond)
		fmt.Println("waiting for pool to be empty")
	}
	fmt.Println("inWork:", commands.inWork.Load())
}
