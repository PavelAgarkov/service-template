package main

import (
	"context"
	"fmt"
	"github.com/streadway/amqp"
	"os"
	"os/signal"
	"service-template/application"
	"service-template/config"
	"service-template/internal"
	"service-template/internal/repository"
	"service-template/internal/service"
	"service-template/pkg"
	"syscall"
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

	pkg.NewMigrations(postgres.GetDB().DB).Migrate("./migrations")

	rmq, rmqCloser := pkg.NewRabbitMq(
		father,
		"amqp://"+"user"+":"+"password"+"@"+"localhost"+":5672/",
		func(ctx context.Context, ret amqp.Return) error {
			log := pkg.LoggerFromCtx(ctx)
			log.Error(fmt.Sprintf("Message %s was returned", string(ret.Body)))
			return nil
		},
	)
	app.RegisterShutdown("rabbitmq_server", rmqCloser, 50)

	background := service.NewBackgroundService()

	_ = internal.NewContainer(
		&internal.ServiceInit{Name: pkg.RabbitMqService, Service: rmq},
		&internal.ServiceInit{Name: pkg.PostgresService, Service: postgres},
	).
		Set(repository.SrvRepositoryService, repository.NewSrvRepository(), pkg.PostgresService).
		Set(service.BackgroundRabbit, background, pkg.RabbitMqService, pkg.PostgresService)

	background.RegisterRabbitQueue(
		"test_queue",
		true,
		false,
		background.BlankConsumer(),
		false,
		false,
		false,
		false,
		false,
		nil,
	)

	background.RunConsumers(father)
	closers := background.GetRegisteredShutdowns()
	for queueName, closer := range closers {
		app.RegisterShutdown(queueName, closer, 10)
	}
	<-father.Done()

	fmt.Println("end")
}
