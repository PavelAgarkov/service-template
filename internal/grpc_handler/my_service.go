package grpc_handler

import (
	myservice "service-template/cmd/grps_server/pb/myservice/pb"
	"service-template/internal/service"
	"service-template/pkg"
)

type MyService struct {
	myservice.UnimplementedMyServiceServer
	srv      *service.Srv
	postgres *pkg.PostgresRepository
}

func NewMyService(postgres *pkg.PostgresRepository, srv *service.Srv) *MyService {
	return &MyService{
		srv:      srv,
		postgres: postgres,
	}
}
