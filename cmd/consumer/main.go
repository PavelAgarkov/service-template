package main

import (
	"context"
	"fmt"
	gorabbitmq "github.com/wagslane/go-rabbitmq"
	"log"
	"service-template/application"
	"service-template/config"
	"service-template/container"
	consumers "service-template/internal/consumer"
	"service-template/pkg"
)

func main() {
	logger := pkg.NewLogger(pkg.LoggerConfig{ServiceName: "consumer", LogPath: "logs/app.log"})

	father, cancel := context.WithCancel(context.Background())
	father = pkg.LoggerWithCtx(father, logger)
	defer cancel()

	cfg := config.GetConfig()

	connectionRabbitString := "amqp://user:password@localhost:5672/"
	app := application.NewApp(father, container.BuildContainerForConsumers(father, logger, cfg, connectionRabbitString), logger)
	app.Start(cancel)
	defer app.Stop()
	defer app.RegisterRecovers()()

	err := app.Container().Invoke(func(
		deps container.ConsumerDependencies,
	) {
		postgres := deps.Postgres
		connrmq := deps.Connrmq
		bg := deps.Bg
		redisClient := deps.RedisClient
		publisherZero := deps.PublisherZero
		publisherFirst := deps.PublisherFirst
		rmq := deps.Rmq
		migrations := deps.Migrations

		app.RegisterShutdown("logger", func() {
			err := logger.Sync()
			if err != nil {
				logger.Error(fmt.Sprintf("failed to sync logger: %v", err))
			}
		}, 101)

		app.RegisterShutdown("postgres", postgres.ShutdownFunc, 100)
		migrations.Migrate(father, "./migrations", "goose_db_version")

		app.RegisterShutdown("rabbitmq", func() { _ = connrmq.Close() }, 100)
		migrations.MigrateRabbitMq(father, "rabbit_migrations", []string{connectionRabbitString})

		app.RegisterShutdown(consumers.Publisher, publisherZero.Close, 9)
		app.RegisterShutdown(consumers.Publisher1, publisherFirst.Close, 9)

		bg.SetPublishers(publisherZero, publisherFirst)

		app.RegisterShutdown(
			"redis-node",
			func() {
				if err := redisClient.Client.Close(); err != nil {
					logger.Info(fmt.Sprintf("failed to close redis connection: %v", err))
				}
			},
			100,
		)

		consumer := rmq.RegisterConsumer(
			connrmq,
			"my_queue",
			gorabbitmq.WithConsumerOptionsConcurrency(2),
			gorabbitmq.WithConsumerOptionsRoutingKey("my_queue"),
			gorabbitmq.WithConsumerOptionsExchangeName("my_events"),
			//отсутвие rabbitmq.WithConsumerOptionsExchangeDeclare и других параметров неявного создания необходимо для работы, т.к. есть миграции, где нужно явно создавать exchange и queue и bind
			gorabbitmq.WithConsumerOptionsExchangeDurable,
			gorabbitmq.WithConsumerOptionsQueueDurable,
			gorabbitmq.WithConsumerOptionsQueueNoDeclare,
		)
		app.RegisterShutdown("consumer", consumer.Close, 10)

		consumer1 := rmq.RegisterConsumer(
			connrmq,
			"test_queue",
			gorabbitmq.WithConsumerOptionsConcurrency(2),
			gorabbitmq.WithConsumerOptionsRoutingKey("test_queue"),
			gorabbitmq.WithConsumerOptionsExchangeName("events"),
			gorabbitmq.WithConsumerOptionsExchangeDurable,
			gorabbitmq.WithConsumerOptionsQueueDurable,
			gorabbitmq.WithConsumerOptionsQueueNoDeclare,
		)
		app.RegisterShutdown("consumer1", consumer1.Close, 10)

		bg.Run(
			father,
			map[string]*consumers.RabbitConsumeRoute{
				"consumer": {
					Consumer: consumer,
					Handler:  bg.BlankConsumer(father),
				},
				"consumer1": {
					Consumer: consumer1,
					Handler: func(d gorabbitmq.Delivery) gorabbitmq.Action {
						log.Printf("consumed_1: %v", string(d.Body))
						return gorabbitmq.Ack
					},
				},
			})

	})

	if err != nil {
		logger.Error(fmt.Sprintf("failed to invoke container %v", err))
		return
	}

	app.Run()
}
