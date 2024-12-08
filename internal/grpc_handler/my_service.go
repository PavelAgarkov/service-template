package grpc_handler

import (
	myservice "service-template/cmd/grps_server/pb/myservice/pb"
	"service-template/internal"
)

type MyService struct {
	myservice.UnimplementedMyServiceServer
	globalContainer *internal.Container
}

func NewMyService(globalContainer *internal.Container) *MyService {
	return &MyService{
		globalContainer: globalContainer,
	}
}

func (ms *MyService) Container() *internal.Container {
	return ms.globalContainer
}
