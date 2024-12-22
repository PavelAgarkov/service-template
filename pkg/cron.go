package pkg

import (
	"fmt"
	"github.com/bsm/redislock"
	"github.com/redis/go-redis/v9"
	"github.com/robfig/cron/v3"
	"go.uber.org/zap"
)

const CronPackage = "cron_package"

type Cron struct {
	C      *cron.Cron
	Locker *redislock.Client
	logger *zap.Logger
}

func NewCronClient(rdb *redis.Client, logger *zap.Logger) *Cron {
	return &Cron{
		Locker: redislock.New(rdb),
		C:      cron.New(cron.WithSeconds()),
		logger: logger,
	}
}

func (c *Cron) AddSchedule(spec string, cmd func()) {
	_, err := c.C.AddFunc(spec, cmd)
	if err != nil {
		c.logger.Fatal(fmt.Sprintf("Ошибка при добавлении задачи в cron: %v", err))
	}
}
