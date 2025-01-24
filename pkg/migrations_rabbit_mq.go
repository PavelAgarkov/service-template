package pkg

import (
	"context"
	"errors"
	"fmt"
	"github.com/bsm/redislock"
	"github.com/pressly/goose/v3"
	"github.com/rabbitmq/amqp091-go"
	"go.uber.org/zap"
	"service-template/rabbit_migrations"
	"time"
)

func (m *Migrations) MigrateRabbitMq(ctx context.Context, tableNameLock string, rabbitConnStringCluster []string) *Migrations {
	lockClient := redislock.New(m.redis)
	lock, err := lockClient.Obtain(ctx, tableNameLock, 60*time.Second, nil)
	if errors.Is(err, redislock.ErrNotObtained) {
		m.logger.Error("Не удалось установить блокировку. Она уже установлена", zap.Error(err))
		return nil
	} else if err != nil {
		m.logger.Error("Ошибка при попытке установить блокировку", zap.Error(err))
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

	if err := m.setupRabbitMQ(rabbitConnStringCluster); err != nil {
		m.logger.Fatal(fmt.Sprintf("Ошибка при настройке RabbitMQ: %v", err))
	}

	return m
}

func (m *Migrations) setupRabbitMQ(rabbitConnStringCluster []string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	connections := make([]*amqp091.Connection, 0)
	for _, connString := range rabbitConnStringCluster {
		conn, err := amqp091.Dial(connString)
		if err != nil {
			return err
		}
		connections = append(connections, conn)
	}
	defer func() {
		for _, conn := range connections {
			conn.Close()
		}
	}()

	m.applyCustom(ctx, rabbit_migrations.Up202412170001(m.logger), rabbit_migrations.RabbitMqVersion202412170001, connections)

	return nil
}
