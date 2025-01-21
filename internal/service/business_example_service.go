package service

import (
	"service-template/internal"
	"service-template/internal/repository"
	"service-template/pkg"
)

const ServiceSrv = "srv"

type Srv struct {
	locator internal.LocatorInterface
}

func NewSrv() *Srv {
	return &Srv{}
}

func (srv *Srv) SetServiceLocator(container internal.LocatorInterface) {
	srv.locator = container
}

func (srv *Srv) GetServiceLocator() internal.LocatorInterface {
	return srv.locator
}

func (srv *Srv) GetSerializerService() *pkg.Serializer {
	serializer, ok := srv.locator.Get(pkg.SerializerService).(*pkg.Serializer)
	if !ok {
		return nil
	}
	return serializer
}

func (srv *Srv) GetEtcdClientService() *pkg.EtcdClientService {
	etcdClientService, ok := srv.locator.Get(pkg.EtcdClient).(*pkg.EtcdClientService)
	if !ok {
		return nil
	}
	return etcdClientService
}

func (srv *Srv) GetPostgresRepository() *pkg.PostgresRepository {
	repo, ok := srv.locator.Get(repository.SrvRepositoryService).(*repository.SrvRepository)
	if !ok {
		return nil
	}
	pg, ok := repo.GetServiceLocator().Get(pkg.PostgresService).(*pkg.PostgresRepository)
	if !ok {
		return nil
	}
	return pg
}
