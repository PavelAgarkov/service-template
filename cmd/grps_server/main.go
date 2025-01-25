package main

import (
	"context"
	"fmt"
	gorabbitmq "github.com/wagslane/go-rabbitmq"
	"google.golang.org/grpc"
	"log"
	"service-template/application"
	myservice "service-template/cmd/grps_server/pb/myservice/pb"
	myservice2 "service-template/cmd/grps_server/pb/myservice2/pb"
	"service-template/config"
	"service-template/container"
	"service-template/internal/grpc_handler"
	"service-template/pkg"
	"service-template/server"
)

func main() {
	logger := pkg.NewLogger(pkg.LoggerConfig{ServiceName: "grpc_server", LogPath: "logs/app.log"})
	father, cancel := context.WithCancel(context.Background())
	father = pkg.LoggerWithCtx(father, logger)
	defer cancel()

	cfg := config.GetConfig()

	connectionRabbitString := "amqp://user:password@localhost:5672/"
	app := application.NewApp(father, container.BuildContainerForGrpcServer(logger, cfg, connectionRabbitString), logger)
	app.Start(cancel)
	defer app.Stop()
	defer app.RegisterRecovers()()

	err := app.Container().Invoke(func(
		postgres *pkg.PostgresRepository,
		connrmq *gorabbitmq.Conn,
		redisClient *pkg.RedisClient,
		migrations *pkg.Migrations,
		myService *grpc_handler.MyService,
		myService2 *grpc_handler.MyService2,
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

		gRPCShutdown := server.CreateGRPCServer(
			func(s *grpc.Server) {
				myservice.RegisterMyServiceServer(s, myService)
				myservice2.RegisterMyServiceServer(s, myService2)
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
				myservice.RegisterMyServiceServer(s, myService)
				myservice2.RegisterMyServiceServer(s, myService2)
			},
			":50052",
			logger,
		)
		app.RegisterShutdown("gRPCS server", gRPCSShutdown, 1)
	})

	if err != nil {
		log.Fatalf("failed to invoke %v", err)
	}

	app.Run()
}
