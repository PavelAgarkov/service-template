package grpc_handler

import (
	myservice2 "service-template/cmd/grps_server/pb/myservice2/pb"
	"service-template/internal/service"
	"service-template/pkg"
)

type MyService2 struct {
	myservice2.UnimplementedMyServiceServer
	srv      *service.Srv
	postgres *pkg.PostgresRepository
}

func NewMyService2(postgres *pkg.PostgresRepository, srv *service.Srv) *MyService2 {
	return &MyService2{
		srv:      srv,
		postgres: postgres,
	}
}
