package main

import (
	"context"
	"fmt"
	"sync"
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

type PutCommand struct {
}

type GetCommand struct {
}

type DelCommand struct {
}

func (g *GetCommand) Apply(ctx context.Context, store Store, key, value string) {
	res := store.Get(key)
	fmt.Println("get", key, ":", res)
}

func (d *DelCommand) Apply(ctx context.Context, store Store, key, value string) {
	store.Del(key)
	fmt.Println("del", key)
}

func (p *PutCommand) Apply(ctx context.Context, store Store, key, value string) {
	store.Put(key, value)
	fmt.Println("put", key, ":", value)
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

// CommandPool
type CommandPool struct {
	store Store
	pool  chan struct{}
}

func NewPoolStore(store Store) *CommandPool {
	return &CommandPool{store: store, pool: make(chan struct{}, 2)}
}

func (p *CommandPool) Stop() {
	if len(p.pool) == 0 {
		p.pool = nil
	}
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
	cmd := p.Factory(command)
	select {
	case <-ctx.Done():
		fmt.Println("context done")
		return
	case p.pool <- struct{}{}:
	}
	fmt.Println("pool size:", len(p.pool), cap(p.pool))
	go func() {
		defer func() {
			select {
			case <-ctx.Done():
				fmt.Println("context done")
			case <-p.pool:
				return
			}
		}()
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
	}

	for {
		if len(commands.pool) == 0 {
			commands.Stop()
			fmt.Println("pool is empty")
			break
		}
		time.Sleep(100 * time.Millisecond)
		fmt.Println("waiting for pool to be empty")
	}
	commands.Reconnect()

	for i, key := range keys {
		commands.Apply(ctx, i%3, key, fmt.Sprintf("%s-%d", key, i))
	}

	for {
		if len(commands.pool) == 0 {
			commands.Stop()
			fmt.Println("pool is empty")
			break
		}
		time.Sleep(100 * time.Millisecond)
		fmt.Println("waiting for pool to be empty")
	}
}
