package main

import (
	"context"
	"fmt"
	"github.com/redis/go-redis/v9"
	gorabbitmq "github.com/wagslane/go-rabbitmq"
	"go.uber.org/dig"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"log"
	"os"
	"os/signal"
	"service-template/application"
	myservice "service-template/cmd/grps_server/pb/myservice/pb"
	myservice2 "service-template/cmd/grps_server/pb/myservice2/pb"
	"service-template/config"
	"service-template/internal/grpc_handler"
	"service-template/internal/service"
	"service-template/pkg"
	"syscall"
	"time"

	"service-template/server"
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
	logger := pkg.NewLogger(pkg.LoggerConfig{ServiceName: "grpc_server", LogPath: "logs/app.log"})
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

	err := container.Invoke(func(
		postgres *pkg.PostgresRepository,
		connrmq *gorabbitmq.Conn,
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

		app.RegisterShutdown(
			"redis-node",
			func() {
				if err := redisClient.Client.Close(); err != nil {
					logger.Info(fmt.Sprintf("failed to close redis connection: %v", err))
				}
			},
			100,
		)
	})

	if err != nil {
		log.Fatalf("failed to invoke %v", err)
	}

	gRPCShutdown := server.CreateGRPCServer(
		func(s *grpc.Server) {
			myservice.RegisterMyServiceServer(s, grpc_handler.NewMyService(container))
			myservice2.RegisterMyServiceServer(s, grpc_handler.NewMyService2(container))
		},
		":50051",
		logger,
	)
	app.RegisterShutdown("gRPC server", gRPCShutdown, 1)

	//openssl req \
	//-x509 \
	//-nodes \
	//-newkey rsa:4096 \
	//-keyout server.key \
	//-out server.crt \
	//-days 365 \
	//-subj "/CN=localhost" \
	//-addext "subjectAltName=DNS:localhost"
	gRPCSShutdown := server.CreateGRPCServerTLS(
		"./server.crt", // Путь к сертификату
		"./server.key", // Путь к ключу
		func(s *grpc.Server) {
			myservice.RegisterMyServiceServer(s, grpc_handler.NewMyService(container))
			myservice2.RegisterMyServiceServer(s, grpc_handler.NewMyService2(container))
		},
		":50052",
		logger,
	)
	app.RegisterShutdown("gRPCS server", gRPCSShutdown, 1)

	<-father.Done()
}
