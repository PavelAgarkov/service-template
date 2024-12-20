package pkg

import (
	"github.com/bsm/redislock"
	"github.com/redis/go-redis/v9"
	"github.com/robfig/cron/v3"
	"log"
)

const CronPackage = "cron_package"

type Cron struct {
	C      *cron.Cron
	Locker *redislock.Client
}

func NewCronClient(rdb *redis.Client) *Cron {
	return &Cron{
		Locker: redislock.New(rdb),
		C:      cron.New(cron.WithSeconds()),
	}
}

func (c *Cron) AddSchedule(spec string, cmd func()) {
	_, err := c.C.AddFunc(spec, cmd)
	if err != nil {
		log.Fatalf("Ошибка при добавлении задачи в cron: %v", err)
	}
}
