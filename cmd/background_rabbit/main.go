package main

import (
	"context"
	"fmt"
	"github.com/redis/go-redis/v9"
	"go.uber.org/dig"
	"go.uber.org/zap"
	"log"
	"os"
	"os/signal"
	"service-template/application"
	"service-template/config"
	consumers "service-template/internal/consumer"
	"service-template/internal/service"
	"service-template/pkg"
	"strconv"
	"syscall"
	"time"

	gorabbitmq "github.com/wagslane/go-rabbitmq"
)

func BuildContainer(logger *zap.Logger, cfg *config.Config, connectionRabbitString string) *dig.Container {
	container := dig.New()
	err := container.Provide(func() *zap.Logger { return logger })
	if err != nil {
		log.Fatalf("failed to provide logger %v", err)
	}
	err = container.Provide(func() *config.Config { return cfg })
	if err != nil {
		log.Fatalf("failed to provide config %v", err)
	}
	err = container.Provide(func() *pkg.PostgresRepository {
		return pkg.NewPostgres(
			logger,
			cfg.DB.Host,
			cfg.DB.Port,
			cfg.DB.Username,
			cfg.DB.Password,
			cfg.DB.Database,
			"disable",
		)
	})
	if err != nil {
		log.Fatalf("failed to provide postgres %v", err)
	}

	err = container.Provide(func() *gorabbitmq.Conn {
		r, _ := gorabbitmq.NewClusterConn(
			gorabbitmq.NewStaticResolver(
				[]string{
					connectionRabbitString,
				},
				false,
			),
			gorabbitmq.WithConnectionOptionsLogging,
		)
		return r
	})
	if err != nil {
		log.Fatalf("failed to provide rabbitmq %v", err)
	}

	err = container.Provide(func() *consumers.ConsumerRabbitService {
		return consumers.NewConsumerRabbitService(container)
	})
	if err != nil {
		log.Fatalf("failed to provide consumer %v", err)
	}

	err = container.Provide(func() *pkg.RedisClient {
		return pkg.NewRedisClient(&redis.Options{
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
	})
	if err != nil {
		log.Fatalf("failed to provide redis %v", err)
	}

	//err = container.Provide(func() *repository.SrvRepository {
	//	return repository.NewSrvRepository()
	//})
	//if err != nil {
	//	log.Fatalf("failed to provide repository %v", err)
	//}

	err = container.Provide(func() *service.Srv {
		return service.NewSrv()
	})
	if err != nil {
		log.Fatalf("failed to provide service %v", err)
	}

	err = container.Provide(func() *pkg.Serializer {
		return pkg.NewSerializer()
	})
	if err != nil {
		log.Fatalf("failed to provide serializer %v", err)
	}

	return container
}

func main() {
	logger := pkg.NewLogger(pkg.LoggerConfig{ServiceName: "background_rabbit", LogPath: "logs/app.log"})
	father, cancel := context.WithCancel(context.Background())
	father = pkg.LoggerWithCtx(father, logger)
	defer cancel()

	logger.Info("config initializing")
	cfg := config.GetConfig()

	connectionRabbitString := "amqp://user:password@localhost:5672/"
	container := BuildContainer(logger, cfg, connectionRabbitString)

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
	defer app.RegisterRecovers(logger, sig)()

	// ----------------------------------------

	//postgres := pkg.NewPostgres(
	//	logger,
	//	cfg.DB.Host,
	//	cfg.DB.Port,
	//	cfg.DB.Username,
	//	cfg.DB.Password,
	//	cfg.DB.Database,
	//	"disable",
	//)
	//app.RegisterShutdown("postgres", postgres.ShutdownFunc, 100)
	rmq := pkg.NewRabbitMQ(logger)

	err := container.Invoke(func(
		postgres *pkg.PostgresRepository,
		connrmq *gorabbitmq.Conn,
		bg *consumers.ConsumerRabbitService,
		redisClient *pkg.RedisClient,
	) {
		app.RegisterShutdown("logger", func() {
			err := logger.Sync()
			if err != nil {
				logger.Error(fmt.Sprintf("failed to sync logger: %v", err))
			}
		}, 101)

		app.RegisterShutdown("postgres", postgres.ShutdownFunc, 100)
		pkg.NewMigrations(postgres.GetDB().DB, logger).Migrate("./migrations", "goose_db_version")

		app.RegisterShutdown("rabbitmq", func() { _ = connrmq.Close() }, 100)
		pkg.NewMigrations(postgres.GetDB().DB, logger).
			Migrate("./migrations", "goose_db_version").
			MigrateRabbitMq("rabbit_migrations", []string{connectionRabbitString})

		publisher := rmq.RegisterPublisher(
			connrmq,
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
			connrmq,
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
		bg.SetPublishers(publisher, publisher1)

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

		go push(publisher, publisher1, father)

	})

	if err != nil {
		logger.Error(fmt.Sprintf("failed to start DI: %v", err))
		return
	}

	//connectionRabbitString := "amqp://user:password@localhost:5672/"
	//pkg.NewMigrations(postgres.GetDB().DB, logger).
	//	Migrate("./migrations", "goose_db_version").
	//	MigrateRabbitMq("rabbit_migrations", []string{connectionRabbitString})

	//conn, err := gorabbitmq.NewClusterConn(
	//	gorabbitmq.NewStaticResolver(
	//		[]string{
	//			connectionRabbitString,
	//		},
	//		false,
	//	),
	//	gorabbitmq.WithConnectionOptionsLogging,
	//)
	//app.RegisterShutdown("rabbitmq", func() { _ = conn.Close() }, 100)
	//
	//if err != nil {
	//	logger.Fatal(fmt.Sprintf("failed to connect to RabbitMQ: %v", err))
	//}

	//bg := consumers.NewConsumerRabbitService()

	//publisher := rmq.RegisterPublisher(
	//	conn,
	//	func(r gorabbitmq.Return) {
	//		err := bg.HandleFailedMessageFromRabbitServer(father, r)()
	//		if err != nil {
	//			logger.Info(fmt.Sprintf("failed to handle failed message: %v", err))
	//			return
	//		}
	//	},
	//	func(c gorabbitmq.Confirmation) {
	//		logger.Info(fmt.Sprintf("publisher_0 message confirmed from server. tag: %v, ack: %v", c.DeliveryTag, c.Ack))
	//	},
	//	gorabbitmq.WithPublisherOptionsExchangeName("events"),
	//	gorabbitmq.WithPublisherOptionsLogging,
	//	gorabbitmq.WithPublisherOptionsExchangeDurable,
	//)
	//app.RegisterShutdown(consumers.Publisher, publisher.Close, 9)
	//
	//publisher1 := rmq.RegisterPublisher(
	//	conn,
	//	func(r gorabbitmq.Return) {
	//		err := bg.HandleFailedMessageFromRabbitServer(father, r)()
	//		if err != nil {
	//			logger.Info(fmt.Sprintf("failed to handle failed message: %v", err))
	//			return
	//		}
	//	},
	//	func(c gorabbitmq.Confirmation) {
	//		logger.Info(fmt.Sprintf("publisher_1 message confirmed from server. tag: %v, ack: %v", c.DeliveryTag, c.Ack))
	//	},
	//	gorabbitmq.WithPublisherOptionsExchangeName("my_events"),
	//	gorabbitmq.WithPublisherOptionsLogging,
	//	gorabbitmq.WithPublisherOptionsExchangeDurable,
	//)
	//app.RegisterShutdown(consumers.Publisher1, publisher1.Close, 9)

	//redisClient := pkg.NewRedisClient(&redis.Options{
	//	Addr:     "127.0.0.1:6379",
	//	Username: "myuser",
	//	Password: "mypassword",
	//	//такое использование баз данных возможно только без кластера
	//	// каждый сервис должен использовать свою базу данных DB
	//	// всего баз в сервере 16 DB
	//	// каждое подключение может использовать только одну базу данных DB
	//	DB:           1,
	//	DialTimeout:  5 * time.Second,
	//	ReadTimeout:  3 * time.Second,
	//	WriteTimeout: 3 * time.Second,
	//
	//	// PoolSize, MinIdleConns можно настраивать при высоконагруженных сценариях.
	//	PoolSize:     10,
	//	MinIdleConns: 2,
	//},
	//	logger,
	//)
	//
	//app.RegisterShutdown(
	//	"redis-node",
	//	func() {
	//		if err := redisClient.Client.Close(); err != nil {
	//			logger.Info(fmt.Sprintf("failed to close redis connection: %v", err))
	//		}
	//	},
	//	100,
	//)

	// -------------------------

	//_ = internal.NewContainer(
	//	logger,
	//	&internal.ServiceInit{Name: pkg.PostgresService, Service: postgres},
	//	&internal.ServiceInit{Name: consumers.Publisher, Service: publisher},
	//	&internal.ServiceInit{Name: consumers.Publisher1, Service: publisher1},
	//	&internal.ServiceInit{Name: pkg.RedisClientService, Service: redisClient},
	//).
	//	Set(repository.SrvRepositoryService, repository.NewSrvRepository(), pkg.PostgresService).
	//	Set(consumers.BackgroundRabbitConsumeService, bg, pkg.PostgresService, consumers.Publisher, consumers.Publisher1)

	//consumer := rmq.RegisterConsumer(
	//	conn,
	//	"my_queue",
	//	gorabbitmq.WithConsumerOptionsConcurrency(2),
	//	gorabbitmq.WithConsumerOptionsRoutingKey("my_queue"),
	//	gorabbitmq.WithConsumerOptionsExchangeName("my_events"),
	//	//отсутвие rabbitmq.WithConsumerOptionsExchangeDeclare и других параметров неявного создания необходимо для работы, т.к. есть миграции, где нужно явно создавать exchange и queue и bind
	//	gorabbitmq.WithConsumerOptionsExchangeDurable,
	//	gorabbitmq.WithConsumerOptionsQueueDurable,
	//	gorabbitmq.WithConsumerOptionsQueueNoDeclare,
	//)
	//app.RegisterShutdown("consumer", consumer.Close, 10)
	//
	//consumer1 := rmq.RegisterConsumer(
	//	conn,
	//	"test_queue",
	//	gorabbitmq.WithConsumerOptionsConcurrency(2),
	//	gorabbitmq.WithConsumerOptionsRoutingKey("test_queue"),
	//	gorabbitmq.WithConsumerOptionsExchangeName("events"),
	//	gorabbitmq.WithConsumerOptionsExchangeDurable,
	//	gorabbitmq.WithConsumerOptionsQueueDurable,
	//	gorabbitmq.WithConsumerOptionsQueueNoDeclare,
	//)
	//app.RegisterShutdown("consumer1", consumer1.Close, 10)
	//
	//bg.Run(
	//	father,
	//	map[string]*consumers.RabbitConsumeRoute{
	//		"consumer": {
	//			Consumer: consumer,
	//			Handler:  bg.BlankConsumer(father),
	//		},
	//		"consumer1": {
	//			Consumer: consumer1,
	//			Handler: func(d gorabbitmq.Delivery) gorabbitmq.Action {
	//				log.Printf("consumed_1: %v", string(d.Body))
	//				return gorabbitmq.Ack
	//			},
	//		},
	//	})
	//
	//go push(publisher, publisher1, father)

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
