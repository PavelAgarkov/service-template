package service

import (
	"service-template/internal"
)

const ServiceSrv = "srv"

type Service interface {
	Get() string
}

type Srv struct {
	locator internal.LocatorInterface
}

func NewSrv() *Srv {
	return &Srv{}
}

func (srv *Srv) Get() string {
	return "22"
}

func (srv *Srv) SetServiceLocator(container internal.LocatorInterface) {
	srv.locator = container
}

func (srv *Srv) GetServiceLocator() internal.LocatorInterface {
	return srv.locator
}
