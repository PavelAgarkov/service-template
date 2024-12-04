package internal

import (
	"log"
	"sync"
)

type Сontainerized interface {
	SetServiceLocator(container ContainerInterface)
	GetServiceLocator() ContainerInterface
}

type ContainerInterface interface {
	Set(key string, value any, services ...string) *Container
	Get(key string) any
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

func (c *Container) Set(key string, value any, services ...string) *Container {
	containerized, ok := value.(Сontainerized)
	if ok {
		if is, service := c.allServicesAreAvailable(services...); is {
			containerized.SetServiceLocator(NewServiceLocator(c, services...))
		} else {
			log.Fatalf("Service %s not found", service)
		}
	}
	c.container[key] = value
	return c
}

func (c *Container) Get(key string) any {
	c.containerMu.RLock()
	defer c.containerMu.RUnlock()
	return c.container[key]
}

func (c *Container) allServicesAreAvailable(services ...string) (bool, string) {
	for _, service := range services {
		if _, ok := c.container[service]; !ok {
			return false, service
		}
	}
	return true, ""
}
