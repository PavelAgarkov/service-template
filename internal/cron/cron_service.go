package cron

import (
	"github.com/redis/go-redis/v9"
	"service-template/internal/repository"
	"service-template/pkg"
)

const CronSService = "cron_service"

type CronService struct {
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
