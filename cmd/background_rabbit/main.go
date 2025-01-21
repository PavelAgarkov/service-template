package main

import (
	"context"
	"fmt"
	"github.com/redis/go-redis/v9"
	"log"
	"os"
	"os/signal"
	"service-template/application"
	"service-template/config"
	"service-template/internal"
	consumers "service-template/internal/consumer"
	"service-template/internal/repository"
	"service-template/pkg"
	"strconv"
	"syscall"
	"time"

	gorabbitmq "github.com/wagslane/go-rabbitmq"
)

func main() {
	logger := pkg.NewLogger(pkg.LoggerConfig{ServiceName: "background_rabbit", LogPath: "logs/app.log"})
	father, cancel := context.WithCancel(context.Background())
	father = pkg.LoggerWithCtx(father, logger)
	defer cancel()

	logger.Info("config initializing")
	cfg := config.GetConfig()

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGTERM, syscall.SIGINT)
	defer signal.Stop(sig)

	go func() {
		<-sig
		logger.Info("Signal received. Shutting down server...")
		cancel()
	}()

	app := application.NewApp(logger)
	defer app.Stop()

	app.RegisterShutdown("logger", func() {
		err := logger.Sync()
		if err != nil {
			log.Println(fmt.Sprintf("failed to sync logger: %v", err))
		}
	}, 101)
	defer app.RegisterRecovers(logger, sig)()

	postgres, postgresShutdown := pkg.NewPostgres(
		logger,
		cfg.DB.Host,
		cfg.DB.Port,
		cfg.DB.Username,
		cfg.DB.Password,
		cfg.DB.Database,
		"disable",
	)
	app.RegisterShutdown("postgres", postgresShutdown, 100)

	connectionRabbitString := "amqp://user:password@localhost:5672/"
	pkg.NewMigrations(postgres.GetDB().DB, logger).
		Migrate("./migrations", "goose_db_version").
		MigrateRabbitMq("rabbit_migrations", []string{connectionRabbitString})

	conn, err := gorabbitmq.NewClusterConn(
		gorabbitmq.NewStaticResolver(
			[]string{
				connectionRabbitString,
			},
			false,
		),
		gorabbitmq.WithConnectionOptionsLogging,
	)
	app.RegisterShutdown("rabbitmq", func() { _ = conn.Close() }, 100)

	if err != nil {
		logger.Fatal(fmt.Sprintf("failed to connect to RabbitMQ: %v", err))
	}

	rmq := pkg.NewRabbitMQ(logger)
	bg := consumers.NewConsumerRabbitService()

	publisher := rmq.RegisterPublisher(
		conn,
		func(r gorabbitmq.Return) {
			err := bg.HandleFailedMessageFromRabbitServer(father, r)()
			if err != nil {
				logger.Info(fmt.Sprintf("failed to handle failed message: %v", err))
				return
			}
		},
		func(c gorabbitmq.Confirmation) {
			logger.Info(fmt.Sprintf("publisher_0 message confirmed from server. tag: %v, ack: %v", c.DeliveryTag, c.Ack))
		},
		gorabbitmq.WithPublisherOptionsExchangeName("events"),
		gorabbitmq.WithPublisherOptionsLogging,
		gorabbitmq.WithPublisherOptionsExchangeDurable,
	)
	app.RegisterShutdown(consumers.Publisher, publisher.Close, 9)

	publisher1 := rmq.RegisterPublisher(
		conn,
		func(r gorabbitmq.Return) {
			err := bg.HandleFailedMessageFromRabbitServer(father, r)()
			if err != nil {
				logger.Info(fmt.Sprintf("failed to handle failed message: %v", err))
				return
			}
		},
		func(c gorabbitmq.Confirmation) {
			logger.Info(fmt.Sprintf("publisher_1 message confirmed from server. tag: %v, ack: %v", c.DeliveryTag, c.Ack))
		},
		gorabbitmq.WithPublisherOptionsExchangeName("my_events"),
		gorabbitmq.WithPublisherOptionsLogging,
		gorabbitmq.WithPublisherOptionsExchangeDurable,
	)
	app.RegisterShutdown(consumers.Publisher1, publisher1.Close, 9)

	redisClient := pkg.NewRedisClient(&redis.Options{
		Addr:     "127.0.0.1:6379",
		Username: "myuser",
		Password: "mypassword",
		//такое использование баз данных возможно только без кластера
		// каждый сервис должен использовать свою базу данных DB
		// всего баз в сервере 16 DB
		// каждое подключение может использовать только одну базу данных DB
		DB:           1,
		DialTimeout:  5 * time.Second,
		ReadTimeout:  3 * time.Second,
		WriteTimeout: 3 * time.Second,

		// PoolSize, MinIdleConns можно настраивать при высоконагруженных сценариях.
		PoolSize:     10,
		MinIdleConns: 2,
	},
		logger,
	)

	app.RegisterShutdown(
		"redis-node",
		func() {
			if err := redisClient.Client.Close(); err != nil {
				logger.Info(fmt.Sprintf("failed to close redis connection: %v", err))
			}
		},
		100,
	)

	_ = internal.NewContainer(
		logger,
		&internal.ServiceInit{Name: pkg.PostgresService, Service: postgres},
		&internal.ServiceInit{Name: consumers.Publisher, Service: publisher},
		&internal.ServiceInit{Name: consumers.Publisher1, Service: publisher1},
		&internal.ServiceInit{Name: pkg.RedisClientService, Service: redisClient},
	).
		Set(repository.SrvRepositoryService, repository.NewSrvRepository(), pkg.PostgresService).
		Set(consumers.BackgroundRabbitConsumeService, bg, pkg.PostgresService, consumers.Publisher, consumers.Publisher1)

	consumer := rmq.RegisterConsumer(
		conn,
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
		conn,
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

	go push(publisher, publisher1, father)

	<-father.Done()
	logger.Info("application exited gracefully")
}

func push(publisher, publisher1 *gorabbitmq.Publisher, father context.Context) {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	i := 0
	for {
		select {
		case <-ticker.C:
			//if err := publisher1.PublishWithContext(
			//	father,
			//	[]byte("main file"),
			//	[]string{"test_queue"},
			//	gorabbitmq.WithPublishOptionsContentType("application/json"),
			//	gorabbitmq.WithPublishOptionsExchange("events"),
			//	gorabbitmq.WithPublishOptionsMandatory,
			//	gorabbitmq.WithPublishOptionsPersistentDelivery,
			//); err != nil {
			//	log.Printf("failed to publish: %v", err)
			//}

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
			//time.Sleep(1 * time.Second)

		case <-father.Done():
			log.Println("stopping publisher")
			return
		}
	}
}
