package http_handler

import (
	"go.uber.org/dig"
	"service-template/container"
	"service-template/internal"
	"service-template/pkg"
)

type Handlers struct {
	globalContainer *internal.Container
	dig             *dig.Container
	postgres        *pkg.PostgresRepository
	etcd            *pkg.EtcdClientService
	serializer      *pkg.Serializer
}

func NewHandlers(globalContainer *internal.Container, dig *dig.Container) *Handlers {
	h := &Handlers{
		globalContainer: globalContainer,
	}

	p, e, s := container.GetPostgresAndEtcdAndSerializerFromContainer(dig)
	h.postgres = p
	h.serializer = s
	h.etcd = e

	return h
}

func (h *Handlers) Container() *internal.Container {
	return h.globalContainer
}

func (h *Handlers) GetDigContainer() *dig.Container {
	return h.dig
}
