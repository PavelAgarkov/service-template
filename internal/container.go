package internal

import (
	"sync"
)

type ContainerInterface interface {
	GetGlobalContainer() *Container
}

func (c *Container) GetGlobalContainer() *Container {
	return c
}

type Container struct {
	containerMu sync.RWMutex
	container   map[string]any
}

func NewContainer() *Container {
	return &Container{
		container: make(map[string]any),
	}
}

func (c *Container) Set(key string, value any) *Container {
	c.containerMu.Lock()
	defer c.containerMu.Unlock()
	c.container[key] = value
	return c
}

func (c *Container) Get(key string) any {
	c.containerMu.RLock()
	defer c.containerMu.RUnlock()
	return c.container[key]
}
