package service

import "service-template/internal"

const CronSService = "cron"

type Cron struct {
	locator internal.LocatorInterface
}

func NewCron() *Cron {
	return &Cron{}
}

func (cr *Cron) SetServiceLocator(container internal.LocatorInterface) {
	cr.locator = container
}

func (cr *Cron) GetServiceLocator() internal.LocatorInterface {
	return cr.locator
}
