package main

import (
	"context"
	"fmt"
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

	rmq, rmqCloser := pkg.NewRabbitMq("amqp://user:password@localhost:5672/")
	app.RegisterShutdown("rabbitmq_server", rmqCloser, 50)

	// Объявление очереди
	queue, err := rmq.Channel.QueueDeclare(
		"test_queue", // имя очереди
		true,         // сохранять сообщения на диске
		false,        // удалять очередь при отсутствии подписчиков
		false,        // эксклюзивная очередь
		false,        // ждать подтверждения
		nil,          // дополнительные аргументы
	)
	if err != nil {
		log.Fatalf("Не удалось объявить очередь: %s", err)
	}

	background := service.NewBackgroundService()
	background.RegisterRabbitQueue(
		"test_queue",
		&queue,
		background.BlankConsumer(),
		true,
		"test_consumer",
		false,
		false,
		false,
		false,
		nil,
	)

	_ = internal.NewContainer(
		&internal.ServiceInit{Name: pkg.RabbitMqService, Service: rmq},
		&internal.ServiceInit{Name: pkg.PostgresService, Service: postgres},
	).
		Set(repository.SrvRepositoryService, repository.NewSrvRepository(), pkg.PostgresService).
		Set(service.BackgroundRabbit, background, pkg.RabbitMqService, pkg.PostgresService)

	background.RunConsumers(father)
	closers := background.GetRegisteredShutdowns()
	for queueName, closer := range closers {
		app.RegisterShutdown(queueName, closer, 10)
	}
	<-father.Done()

	fmt.Println("end")
}
