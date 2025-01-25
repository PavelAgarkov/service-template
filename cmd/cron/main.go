package main

import (
	"context"
	"fmt"
	gorabbitmq "github.com/wagslane/go-rabbitmq"
	"log"
	"service-template/application"
	"service-template/config"
	"service-template/container"
	"service-template/internal/cron"
	"service-template/internal/repository"
	"service-template/pkg"
)

func main() {
	logger := pkg.NewLogger(pkg.LoggerConfig{ServiceName: "cron", LogPath: "logs/app.log"})

	father, cancel := context.WithCancel(context.Background())
	father = pkg.LoggerWithCtx(father, logger)
	defer cancel()

	logger.Info("config initializing")
	cfg := config.GetConfig()

	connectionRabbitString := "amqp://user:password@localhost:5672/"
	app := application.NewApp(father, container.BuildContainerForCron(logger, cfg, connectionRabbitString), logger)
	app.Start(cancel)
	defer app.Stop()
	defer app.RegisterRecovers()()

	err := app.Container().Invoke(func(
		postgres *pkg.PostgresRepository,
		connrmq *gorabbitmq.Conn,
		redisRepo *repository.RedisRepository,
		redisClient *pkg.RedisClient,
		cronClient *pkg.Cron,
		cronService *cron.CronService,
		migrations *pkg.Migrations,
	) {
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

		app.RegisterShutdown(
			"redis-node",
			func() {
				if err := redisClient.Client.Close(); err != nil {
					logger.Info(fmt.Sprintf("failed to close redis connection: %v", err))
				}
			},
			100,
		)

		cronClient.AddSchedule("* * * * * *", cronService.Lock(father, cronService.Blank, "blank_cron_lock"))
		cronClient.AddSchedule("* * * * * *", cronService.Lock(father, cronService.Blank, "blank_1_cron_lock"))
		cronClient.C.Start()
		app.RegisterShutdown("cron", func() { cronClient.C.Stop() }, 10)
	})

	if err != nil {
		log.Fatalf("failed to invoke %v", err)
	}

	app.Run()
}
