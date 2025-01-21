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
	"service-template/internal/cron"
	"service-template/internal/repository"
	"service-template/pkg"
	"syscall"
	"time"

	gorabbitmq "github.com/wagslane/go-rabbitmq"
)

func main() {
	logger := pkg.NewLogger(pkg.LoggerConfig{ServiceName: "cron", LogPath: "logs/app.log"})
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

	cronCl := pkg.NewCronClient(redisClient.Client, logger)
	cronService := cron.NewCronService()

	_ = internal.NewContainer(
		logger,
		&internal.ServiceInit{Name: pkg.PostgresService, Service: postgres},
		&internal.ServiceInit{Name: pkg.RedisClientService, Service: redisClient},
		&internal.ServiceInit{Name: pkg.CronPackage, Service: cronCl},
	).
		Set(repository.RedisRepositoryLabel, repository.NewRedisRepository(), pkg.RedisClientService).
		Set(cron.CronSService, cronService, pkg.CronPackage, repository.RedisRepositoryLabel)

	cronCl.AddSchedule("* * * * * *", cronService.Blank(father))
	cronCl.C.Start()
	app.RegisterShutdown("cron", func() { cronCl.C.Stop() }, 10)

	<-father.Done()
	logger.Info("application exited gracefully")
}
