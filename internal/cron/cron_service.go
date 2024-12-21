package cron

import "service-template/internal"

const CronSService = "cron_service"

type CronService struct {
	locator internal.LocatorInterface
}

func NewCronService() *CronService {
	return &CronService{}
}

func (cr *CronService) SetServiceLocator(container internal.LocatorInterface) {
	cr.locator = container
}

func (cr *CronService) GetServiceLocator() internal.LocatorInterface {
	return cr.locator
}
