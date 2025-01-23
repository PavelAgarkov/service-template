package cron

import (
	"context"
	"time"
)

func (cr *CronService) Blank(father context.Context) func() {
	return func() {
		time.Sleep(10 * time.Second)
	}
}
