package main

import (
	"context"
	"fmt"
	"github.com/redis/go-redis/v9"
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
	father, cancel := context.WithCancel(context.Background())
	father = pkg.LoggerWithCtx(father, pkg.GetLogger())
	defer cancel()
	l := pkg.LoggerFromCtx(father)

	l.Info("config initializing")
	cfg := config.GetConfig()

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGTERM, syscall.SIGINT)
	defer signal.Stop(sig)

	go func() {
		<-sig
		l.Info("Signal received. Shutting down server...")
		cancel()
	}()

	app := application.NewApp()
	defer func() {
		app.Stop()
		l.Info("app is stopped")
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

	connectionRabbitString := "amqp://user:password@localhost:5672/"
	pkg.NewMigrations(postgres.GetDB().DB).
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
		l.Fatal(fmt.Sprintf("failed to connect to RabbitMQ: %v", err))
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
	})

	app.RegisterShutdown(
		"redis-node",
		func() {
			if err := redisClient.Client.Close(); err != nil {
				l.Info(fmt.Sprintf("failed to close redis connection: %v", err))
			}
		},
		100,
	)

	cronCl := pkg.NewCronClient(redisClient.Client)
	cronService := cron.NewCronService()

	_ = internal.NewContainer(
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
	l.Info("application exited gracefully")
}
