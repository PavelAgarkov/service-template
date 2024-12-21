package service

import (
	"service-template/internal"
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
