package internal

import (
	"go.uber.org/zap"
	"log"
)

type ServiceInit struct {
	Name    string
	Service any
}

type Сontainerized interface {
	SetServiceLocator(container LocatorInterface)
	GetServiceLocator() LocatorInterface
}

type ContainerInterface interface {
	Set(key string, value any, services ...string) *Container
	Get(key string) any
}

type LocatorInterface interface {
	Get(key string) any
}

type Container struct {
	container    map[string]any
	requirements map[string]any
	logger       *zap.Logger
}

func NewContainer(logger *zap.Logger, serviceInit ...*ServiceInit) *Container {
	container := &Container{
		container:    make(map[string]any),
		requirements: make(map[string]any),
		logger:       logger,
	}
	for _, v := range serviceInit {
		container.setRequirements(v.Name, v.Service)
	}
	return container
}

func (c *Container) Set(key string, cnt any, services ...string) *Container {
	if _, ok := c.container[key]; ok {
		log.Fatalf("Service %s not found in container", key)
	}

	containerized, ok := cnt.(Сontainerized)
	if ok && containerized != nil {
		is, service := c.allServicesAreAvailableInContainerAndRequirements(services...)
		if is {
			newLocator := c.NewServiceLocator(services...)
			containerized.SetServiceLocator(newLocator)
		} else {
			log.Fatalf("Service %s not found", service)
		}
	}
	c.container[key] = containerized

	return c
}

func (c *Container) Get(key string) any {
	if _, ok := c.container[key]; !ok {
		return nil
	}
	return c.container[key]
}

func (c *Container) NewServiceLocator(services ...string) LocatorInterface {
	localContainer := NewContainer(c.logger)

	for _, serviceName := range services {
		serviceFromContainer := c.getForLocator(serviceName)
		localContainer.setForLocator(serviceName, serviceFromContainer)
	}

	return localContainer
}

func (c *Container) setRequirements(key string, value any) *Container {
	if _, ok := c.requirements[key]; ok {
		log.Fatalf("Service %s not found in requirements", key)
	}
	c.requirements[key] = value
	return c
}

func (c *Container) setForLocator(key string, cnt any) *Container {
	if _, ok := c.container[key]; ok {
		log.Fatalf("Service %s not found in container", key)
	}
	c.container[key] = cnt
	return c

}

func (c *Container) getForLocator(key string) any {
	value, okContainer := c.container[key]
	if okContainer {
		return value
	}

	value, okRequirements := c.requirements[key]
	if okRequirements {
		return value
	}

	log.Fatalf("Service %s not found", key)
	return nil
}

func (c *Container) allServicesAreAvailableInContainerAndRequirements(services ...string) (bool, string) {
	for _, service := range services {
		_, okContainer := c.container[service]
		_, okRequirements := c.requirements[service]
		if !okContainer && !okRequirements {
			return false, service
		}
	}
	return true, ""
}
