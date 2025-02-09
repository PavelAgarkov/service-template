package pkg

import (
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
