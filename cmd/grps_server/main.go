package main

import (
	"context"
	"fmt"
	"google.golang.org/grpc"
	"log"
	"os"
	"os/signal"
	"service-template/application"
	myservice "service-template/cmd/grps_server/pb/myservice/pb"
	myservice2 "service-template/cmd/grps_server/pb/myservice2/pb"
	"service-template/config"
	"service-template/internal"
	"service-template/internal/grpc_handler"
	"service-template/internal/repository"
	"service-template/internal/service"
	"service-template/pkg"
	"syscall"

	"service-template/server"
)

func main() {
	logger := pkg.NewLogger(pkg.LoggerConfig{ServiceName: "grpc_server", LogPath: "logs/app.log"})
	father, cancel := context.WithCancel(context.Background())
	father = pkg.LoggerWithCtx(father, logger)
	defer cancel()

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGTERM, syscall.SIGINT)
	defer signal.Stop(sig)

	cfg := config.GetConfig()

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

	pkg.NewMigrations(postgres.GetDB().DB, logger).Migrate("./migrations", "goose_db_version")

	container := internal.NewContainer(
		logger,
		&internal.ServiceInit{Name: pkg.PostgresService, Service: postgres},
	).
		Set(repository.SrvRepositoryService, repository.NewSrvRepository(), pkg.PostgresService).
		Set(service.ServiceSrv, service.NewSrv(), repository.SrvRepositoryService)

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
