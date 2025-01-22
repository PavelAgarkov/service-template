package grpc_handler

import (
	"go.uber.org/dig"
	myservice "service-template/cmd/grps_server/pb/myservice/pb"
	"service-template/internal/service"
	"service-template/pkg"
)

type MyService struct {
	myservice.UnimplementedMyServiceServer
	srv      *service.Srv
	postgres *pkg.PostgresRepository
}

func NewMyService(dig *dig.Container) *MyService {
	my := &MyService{}
	err := dig.Invoke(func(srv *service.Srv, postgres *pkg.PostgresRepository) {
		my.srv = srv
		my.postgres = postgres
	})
	if err != nil {
		panic(err)
	}
	return my
}
