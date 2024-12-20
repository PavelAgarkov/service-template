package service

import (
	"context"
	"github.com/bsm/redislock"
	"github.com/google/uuid"
	"log"
	"service-template/pkg"
	"time"
)

func (cr *Cron) Blank(father context.Context) func() {
	return func() { // Каждую минуту
		cron := cr.GetServiceLocator().Get(pkg.CronPackage).(*pkg.Cron)
		lock, err := cron.Locker.Obtain(father, "my_cron_lock", 2*time.Second, nil)
		if err == redislock.ErrNotObtained {
			log.Println("Блокировка уже установлена, пропускаем выполнение задачи.")
			return
		} else if err != nil {
			log.Printf("Ошибка при попытке установить блокировку: %v", err)
			return
		}
		defer lock.Release(father)

		// Ваша задача
		log.Println("Выполнение задачи:", uuid.New().String())
		// Симуляция длительной работы
		time.Sleep(10 * time.Second)
	}
}
