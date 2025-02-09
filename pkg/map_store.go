package pkg

import (
	"context"
	"fmt"
	"sync"
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
	GetKey() C
	GetValue() D
}

type BaseCommand[C, D String] struct {
	key   C
	value D
}

func NewPutCommand[C, D String](key C, value D) *PutCommand[C, D] {
	return &PutCommand[C, D]{
		BaseCommand: BaseCommand[C, D]{key: key, value: value},
	}
}

type PutCommand[C, D String] struct {
	BaseCommand[C, D]
}

func (p *PutCommand[C, D]) GetKey() C {
	return p.key
}

func (p *PutCommand[C, D]) GetValue() D {
	return p.value
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

func (g *GetCommand[C, D]) GetKey() C {
	return g.key
}

func (g *GetCommand[C, D]) GetValue() D {
	return g.value
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

func (d *DelCommand[C, D]) GetKey() C {
	return d.key
}

func (d *DelCommand[C, D]) GetValue() D {
	return d.value
}

func (d *DelCommand[C, D]) Apply(ctx context.Context, store Store, key C, value D) {
	store.Del(string(key))
	fmt.Println("del", key)
}
