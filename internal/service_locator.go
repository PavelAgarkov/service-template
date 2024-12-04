package internal

func NewServiceLocator(container ContainerInterface, services ...string) *Container {
	localContainer := NewContainer()

	for _, service := range services {
		localContainer.Set(service, container.Get(service))
	}

	return localContainer
}
