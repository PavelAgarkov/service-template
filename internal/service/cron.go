package service

import "service-template/internal"

const CronSService = "cron"

type CronService interface {
	Get() string
}

type Cron struct {
	locator internal.LocatorInterface
}

func NewCron() *Cron {
	return &Cron{}
}

func (cr *Cron) Get() string {
	return "22"
}

func (cr *Cron) SetServiceLocator(container internal.LocatorInterface) {
	cr.locator = container
}

func (cr *Cron) GetServiceLocator() internal.LocatorInterface {
	return cr.locator
}
