package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"service-template/application"
	"service-template/config"
	"service-template/internal"
	"service-template/internal/repository"
	"service-template/internal/service"
	"service-template/pkg"
	"syscall"
	"time"

	gorabbitmq "github.com/wagslane/go-rabbitmq"
)

func main() {
	father, cancel := context.WithCancel(context.Background())
	father = pkg.LoggerWithCtx(father, pkg.GetLogger())
	defer cancel()

	pkg.LoggerFromCtx(father).Info("config initializing")
	cfg := config.GetConfig()

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGTERM, syscall.SIGINT)
	defer signal.Stop(sig)

	go func() {
		<-sig
		pkg.LoggerFromCtx(father).Info("Signal received. Shutting down server...")
		cancel()
	}()

	app := application.NewApp()
	defer func() {
		app.Stop()
		pkg.LoggerFromCtx(father).Info("app is stopped")
	}()

	postgres, postgresShutdown := pkg.NewPostgres(
		cfg.DB.Host,
		cfg.DB.Port,
		cfg.DB.Username,
		cfg.DB.Password,
		cfg.DB.Database,
		"disable",
	)
	app.RegisterShutdown("postgres", postgresShutdown, 100)

	pkg.NewMigrations(postgres.GetDB().DB).
		Migrate("./migrations", "goose_db_version").
		MigrateRabbitMq("rabbit_migrations", []string{"amqp://user:password@localhost:5672/"})

	conn, err := gorabbitmq.NewClusterConn(
		gorabbitmq.NewStaticResolver(
			[]string{
				"amqp://user:password@localhost:5672/",
			},
			false,
		),
		gorabbitmq.WithConnectionOptionsLogging,
	)
	app.RegisterShutdown("rabbitmq", func() { _ = conn.Close() }, 50)

	if err != nil {
		log.Fatalf("failed to connect to RabbitMQ: %v", err)
	}

	bg := service.NewBackgroundService()
	rmq := pkg.NewRabbitMQ()

	publisher := rmq.RegisterPublisher(
		conn,
		func(r gorabbitmq.Return) {
			log.Printf("message returned from server: %v", r)
		},
		func(c gorabbitmq.Confirmation) {
			log.Printf("message confirmed from server. tag: %v, ack: %v", c.DeliveryTag, c.Ack)
		},
		gorabbitmq.WithPublisherOptionsExchangeName("events"),
		gorabbitmq.WithPublisherOptionsLogging,
		gorabbitmq.WithPublisherOptionsExchangeDurable,
	)
	app.RegisterShutdown(service.Publisher, publisher.Close, 9)

	publisher1 := rmq.RegisterPublisher(
		conn,
		func(r gorabbitmq.Return) {
			log.Printf("message returned from server: %v", r)
		},
		func(c gorabbitmq.Confirmation) {
			log.Printf("message confirmed from server. tag: %v, ack: %v", c.DeliveryTag, c.Ack)
		},
		gorabbitmq.WithPublisherOptionsExchangeName("my_events"),
		gorabbitmq.WithPublisherOptionsLogging,
		gorabbitmq.WithPublisherOptionsExchangeDurable,
	)
	app.RegisterShutdown(service.Publisher1, publisher1.Close, 9)

	_ = internal.NewContainer(
		&internal.ServiceInit{Name: pkg.PostgresService, Service: postgres},
		&internal.ServiceInit{Name: service.Publisher, Service: publisher},
		&internal.ServiceInit{Name: service.Publisher1, Service: publisher1},
	).
		Set(repository.SrvRepositoryService, repository.NewSrvRepository(), pkg.PostgresService).
		Set(service.BackgroundRabbit, bg, pkg.PostgresService, service.Publisher, service.Publisher1)

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
		map[string]*service.Route{
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
	log.Println("application exited gracefully")
}

func push(publisher, publisher1 *gorabbitmq.Publisher, father context.Context) {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

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
				[]byte("my hello, world"),
				[]string{"my_queue"},
				gorabbitmq.WithPublishOptionsContentType("application/json"),
				gorabbitmq.WithPublishOptionsExchange("my_events"),
				gorabbitmq.WithPublishOptionsMandatory,
				gorabbitmq.WithPublishOptionsPersistentDelivery,
			); err != nil {
				log.Printf("failed to publish: %v", err)
			}
			time.Sleep(5 * time.Second)

		case <-father.Done():
			log.Println("stopping publisher")
			return
		}
	}
}
