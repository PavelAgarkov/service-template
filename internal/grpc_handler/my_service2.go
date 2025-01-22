package grpc_handler

import (
	"go.uber.org/dig"
	myservice2 "service-template/cmd/grps_server/pb/myservice2/pb"
	"service-template/internal/service"
	"service-template/pkg"
)

type MyService2 struct {
	myservice2.UnimplementedMyServiceServer
	srv      *service.Srv
	postgres *pkg.PostgresRepository
}

func NewMyService2(dig *dig.Container) *MyService2 {
	my := &MyService2{}
	err := dig.Invoke(func(srv *service.Srv, postgres *pkg.PostgresRepository) {
		my.srv = srv
		my.postgres = postgres
	})
	if err != nil {
		panic(err)
	}
	return my
}
