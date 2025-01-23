package pkg

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"github.com/bsm/redislock"
	"github.com/pressly/goose/v3"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
	"time"
)

type Migrations struct {
	db     *sql.DB
	logger *zap.Logger
	redis  *redis.Client
}

func NewMigrations(db *sql.DB, logger *zap.Logger, redis *redis.Client) *Migrations {
	return &Migrations{
		db:     db,
		logger: logger,
		redis:  redis,
	}
}

func (m *Migrations) Migrate(ctx context.Context, path string, tableNameLock string) *Migrations {
	lockClient := redislock.New(m.redis)
	lock, err := lockClient.Obtain(ctx, tableNameLock, 10*time.Second, nil)
	if errors.Is(err, redislock.ErrNotObtained) {
		m.logger.Error("Не удалось установить блокировку. Она уже установлена")
		return nil
	} else if err != nil {
		fmt.Printf("Ошибка при попытке установить блокировку: %v\n", err)
		return nil
	}

	defer func() {
		if err := lock.Release(ctx); err != nil {
			fmt.Printf("Ошибка при освобождении блокировки: %v\n", err)
			return
		}
	}()

	goose.SetBaseFS(nil)
	err = goose.SetDialect("postgres")
	if err != nil {
		m.logger.Fatal(fmt.Sprintf("error set dialect: %v", err))
	}
	goose.SetTableName(tableNameLock)

	if err := goose.Up(m.db, path); err != nil {
		m.logger.Fatal(fmt.Sprintf("Ошибка при выполнении миграции: %v", err))
	}

	return m
}
