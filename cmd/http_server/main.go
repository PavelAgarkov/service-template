package main

import (
	"context"
	"fmt"
	"github.com/prometheus/client_golang/prometheus"
	"os"
	"service-template/application"
	_ "service-template/cmd/http_server/docs"
	"service-template/config"
	"service-template/container"
	"service-template/internal/http_handler"
	"service-template/internal/repository"
	"service-template/internal/service"
	"service-template/pkg"
	"service-template/server"
)

// @title Simple HTTP Server API
// @version 1.0
// @description Это пример HTTP-сервера с документацией Swagger.
// @contact.name Поддержка API
// @contact.url http://example.com/support
// @contact.email support@example.com
// @host localhost:3000
// @BasePath /
func main() {
	logger := pkg.NewLogger(pkg.LoggerConfig{ServiceName: "simple_http_server", LogPath: "logs/app.log"})

	father, cancel := context.WithCancel(context.Background())
	father = pkg.LoggerWithCtx(father, logger)
	defer cancel()

	cfg := config.GetConfig()
	port := ":" + os.Getenv("HTTP_PORT")
	app := application.NewApp(father, container.BuildContainerForHttpServer(father, logger, cfg, port), logger)
	app.Start(cancel)
	defer app.Stop()
	defer app.RegisterRecovers()()

	err := app.Container().Invoke(func(
		postgres *pkg.PostgresRepository,
		etcdClientService *pkg.EtcdClientService,
		srvRepository *repository.SrvRepository,
		srvService *service.Srv,
		elk *pkg.ElasticFacade,
		redisClient *pkg.RedisClient,
		migrations *pkg.Migrations,
		prom *pkg.Prometheus,
		handlers *http_handler.Handlers,
	) {
		prometheus.MustRegister(prom.Http)
		app.RegisterShutdown("prometheus-http", func() { prometheus.Unregister(prom.Http) }, 200)

		app.RegisterShutdown("logger", func() {
			err := logger.Sync()
			if err != nil {
				logger.Error(fmt.Sprintf("failed to sync logger: %v", err))
			}
		}, 101)

		app.RegisterShutdown("postgres", postgres.ShutdownFunc, 100)
		migrations.Migrate(father, "./migrations", "goose_db_version")

		connectionRabbitString := "amqp://user:password@localhost:5672/"
		migrations.MigrateRabbitMq(father, "rabbit_migrations", []string{connectionRabbitString})

		_, err := etcdClientService.CreateSession()
		if err != nil {
			logger.Error(fmt.Sprintf("failed to create session: %v", err))
		}
		app.RegisterShutdown("etcd_session", func() {
			etcdClientService.StopSession()
		}, 98)
		app.RegisterShutdown(pkg.EtcdClient, func() {
			etcdClientService.ShutdownFunc()
			logger.Info("etcd client closed")
		}, 99)
		err = etcdClientService.Register(father, logger)
		if err != nil {
			logger.Error(fmt.Sprintf("failed to register service: %v", err))
		}

		simpleHttpServerShutdownFunctionHttp := server.CreateHttpServer(
			logger,
			nil,
			handlers.HandlerList(),
			port,
			server.LoggerContextMiddleware(logger),
			server.RecoverMiddleware,
			server.LoggingMiddleware,
		)
		app.RegisterShutdown("simple_https_server", simpleHttpServerShutdownFunctionHttp, 1)

		//openssl req \
		//-x509 \
		//-nodes \
		//-newkey rsa:4096 \
		//-keyout server.key \
		//-out server.crt \
		//-days 365 \
		//-subj "/CN=localhost" \
		//-addext "subjectAltName=DNS:localhost"
		simpleHttpServerShutdownFunctionHttps := server.CreateHttpsServer(
			logger,
			nil,
			handlers.HandlerList(),
			":8433",        // Порт сервера
			"./server.crt", // Путь к сертификату
			"./server.key", // Путь к ключу
			server.LoggerContextMiddleware(logger),
			server.RecoverMiddleware,
			server.LoggingMiddleware,
		)
		app.RegisterShutdown("simple_https_server", simpleHttpServerShutdownFunctionHttps, 1)
	})

	if err != nil {
		logger.Error(fmt.Sprintf("failed to start DI: %v", err))
		return
	}

	app.Run()
}
