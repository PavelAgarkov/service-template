package main

import (
	"context"
	"fmt"
	"log"
	"service-template/application"
	"service-template/config"
	"service-template/container"
	consumers "service-template/internal/consumer"
	"service-template/pkg"
	"strconv"
	"time"

	gorabbitmq "github.com/wagslane/go-rabbitmq"
)

func main() {
	logger := pkg.NewLogger(pkg.LoggerConfig{ServiceName: "background_rabbit", LogPath: "logs/app.log"})

	father, cancel := context.WithCancel(context.Background())
	father = pkg.LoggerWithCtx(father, logger)
	defer cancel()

	cfg := config.GetConfig()
	connectionRabbitString := "amqp://user:password@localhost:5672/"

	app := application.NewApp(father, container.BuildContainerForBackgroundRabbitMq(father, logger, cfg, connectionRabbitString), logger)
	app.Start(cancel)
	defer app.Stop()
	defer app.RegisterRecovers()()

	err := app.Container().Invoke(func(
		deps container.RabbitBackgroundDependencies,
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

		go push(publisherZero, publisherFirst, father)

	})

	if err != nil {
		logger.Error(fmt.Sprintf("failed to start DI: %v", err))
		return
	}

	app.Run()
}

func push(publisher, publisher1 *gorabbitmq.Publisher, father context.Context) {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	i := 0
	for {
		select {
		case <-ticker.C:
			if err := publisher.PublishWithContext(
				father,
				[]byte("my hello, world------>"+strconv.Itoa(i)),
				[]string{"my_queue"},
				gorabbitmq.WithPublishOptionsContentType("application/json"),
				gorabbitmq.WithPublishOptionsExchange("my_events"),
				gorabbitmq.WithPublishOptionsMandatory,
				gorabbitmq.WithPublishOptionsPersistentDelivery,
			); err != nil {
				log.Printf("failed to publish: %v", err)
			}
			i++

		case <-father.Done():
			log.Println("stopping publisher")
			return
		}
	}
}
