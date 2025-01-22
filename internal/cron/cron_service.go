package cron

import (
	"github.com/redis/go-redis/v9"
	"service-template/internal"
	"service-template/internal/repository"
	"service-template/pkg"
)

const CronSService = "cron_service"

type CronService struct {
	locator   internal.LocatorInterface
	cron      *pkg.Cron
	redis     *redis.Client
	redisRepo *repository.RedisRepository
}

func NewCronService(cron *pkg.Cron, redis *redis.Client, redisRepo *repository.RedisRepository) *CronService {
	return &CronService{
		cron:      cron,
		redis:     redis,
		redisRepo: redisRepo,
	}
}

func (cr *CronService) SetServiceLocator(container internal.LocatorInterface) {
	cr.locator = container
}

func (cr *CronService) GetServiceLocator() internal.LocatorInterface {
	return cr.locator
}
