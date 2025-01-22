package container

import (
	"go.uber.org/dig"
	"log"
	"service-template/pkg"
)

func GetPostgresAndEtcdAndSerializerFromContainer(dig *dig.Container) (*pkg.PostgresRepository, *pkg.EtcdClientService, *pkg.Serializer) {
	var (
		postgres   *pkg.PostgresRepository
		etcd       *pkg.EtcdClientService
		serializer *pkg.Serializer
	)
	err := dig.Invoke(func(p *pkg.PostgresRepository, e *pkg.EtcdClientService, s *pkg.Serializer) {
		postgres = p
		etcd = e
		serializer = s
	})
	if err != nil {
		log.Fatal(err)
	}
	return postgres, etcd, serializer
}
