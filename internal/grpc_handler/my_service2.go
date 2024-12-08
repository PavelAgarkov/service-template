package grpc_handler

import (
	myservice2 "service-template/cmd/grps_server/pb/myservice2/pb"
	"service-template/internal"
)

type MyService2 struct {
	myservice2.UnimplementedMyServiceServer
	globalContainer *internal.Container
}

func NewMyService2(globalContainer *internal.Container) *MyService2 {
	return &MyService2{
		globalContainer: globalContainer,
	}
}

func (ms2 *MyService2) Container() *internal.Container {
	return ms2.globalContainer
}
