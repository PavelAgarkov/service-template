package cron

import (
	"context"
	"errors"
	"github.com/bsm/redislock"
	"github.com/redis/go-redis/v9"
	"log"
	"service-template/internal/repository"
	"service-template/pkg"
	"time"
)

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

func (cr *CronService) Lock(father context.Context, next func(father context.Context) func(), cronLockName string) func() {
	return func() {
		lock, err := cr.cron.Locker.Obtain(father, cronLockName, 10*time.Second, nil)
		if errors.Is(err, redislock.ErrNotObtained) {
			log.Println("Блокировка уже установлена, пропускаем выполнение задачи.")
			return
		}
		log.Println("Выполнение задачи:", cronLockName)
		next(father)()
		log.Println("Задача выполнена:", cronLockName)
		defer lock.Release(father)
	}
}
