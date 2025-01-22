package http_handler

import (
	"go.uber.org/dig"
	"service-template/container"
	"service-template/pkg"
)

type Handlers struct {
	postgres   *pkg.PostgresRepository
	etcd       *pkg.EtcdClientService
	serializer *pkg.Serializer
}

func NewHandlers(dig *dig.Container) *Handlers {
	p, e, s := container.GetPostgresAndEtcdAndSerializerFromContainer(dig)
	return &Handlers{
		postgres:   p,
		etcd:       e,
		serializer: s,
	}
}
