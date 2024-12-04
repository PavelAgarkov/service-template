package service

import "service-template/internal"

type Service interface {
	Get() string
}

type Srv struct {
	container internal.ContainerInterface
}

func NewSrv() *Srv {
	return &Srv{}
}

func (simple *Srv) Get() string {
	return "22"
}

func (simple *Srv) SetServiceLocator(container internal.ContainerInterface) {
	simple.container = container
}

func (simple *Srv) GetServiceLocator() internal.ContainerInterface {
	return simple.container
}
