package pkg

import (
	"context"
	"github.com/pressly/goose/v3"
	"github.com/rabbitmq/amqp091-go"
	"log"
	"service-template/rabbit_migrations"
	"time"
)

func (m *Migrations) MigrateRabbitMq(tableName string, rabbitConnStringCluster []string) *Migrations {
	goose.SetBaseFS(nil)
	err := goose.SetDialect("postgres")
	if err != nil {
		log.Fatalf("error set dialect: %v", err)
	}
	goose.SetTableName(tableName)

	if err := m.setupRabbitMQ(rabbitConnStringCluster); err != nil {
		log.Fatalf("Ошибка при настройке RabbitMQ: %v", err)
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

	m.applyCustom(ctx, rabbit_migrations.Up202412170001(), rabbit_migrations.RabbitMqVersion202412170001, connections)

	return nil
}
