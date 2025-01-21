package main

import (
	"context"
	"fmt"
	httpSwagger "github.com/swaggo/http-swagger"
	"log"
	"net/http"
	"os"
	"os/signal"
	"service-template/application"
	_ "service-template/cmd/http_server/docs"
	"service-template/config"
	"service-template/internal"
	"service-template/internal/http_handler"
	"service-template/internal/repository"
	"service-template/internal/service"
	"service-template/pkg"
	"service-template/server"
	"syscall"
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
			logger.Error(fmt.Sprintf("failed to sync logger: %v", err))
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

	port := ":" + os.Getenv("HTTP_PORT")

	serviceID := pkg.NewServiceId()
	serviceKey := pkg.NewServiceKey(serviceID, "my-service")
	etcdClientService, etcdCloser := pkg.NewEtcdClientService(
		father,
		"http://localhost:2379",
		"admin",
		"adminpassword",
		port,
		"http",
		serviceKey,
		serviceID,
		logger,
	)
	_, err := etcdClientService.CreateSession()
	if err != nil {
		logger.Error(fmt.Sprintf("failed to create session: %v", err))
	}
	app.RegisterShutdown("etcd_session", func() {
		etcdClientService.StopSession()
	}, 98)

	app.RegisterShutdown(pkg.EtcdClient, func() {
		etcdCloser()
		logger.Info("etcd client closed")
	}, 99)

	err = etcdClientService.Register(father, logger)
	if err != nil {
		logger.Error(fmt.Sprintf("failed to register service: %v", err))
	}

	container := internal.NewContainer(
		logger,
		&internal.ServiceInit{Name: pkg.SerializerService, Service: pkg.NewSerializer()},
		&internal.ServiceInit{Name: pkg.PostgresService, Service: postgres},
		&internal.ServiceInit{Name: pkg.EtcdClient, Service: etcdClientService},
	).
		Set(repository.SrvRepositoryService, repository.NewSrvRepository(), pkg.PostgresService).
		Set(service.ServiceSrv, service.NewSrv(), pkg.SerializerService, repository.SrvRepositoryService, pkg.EtcdClient)

	handlers := http_handler.NewHandlers(container)

	simpleHttpServerShutdownFunctionHttp := server.CreateHttpServer(
		logger,
		nil,
		handlerList(handlers),
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
	//simpleHttpServerShutdownFunctionHttps := server.CreateHttpsServer(
	//	logger,
	//	handlerList(handlers),
	//	":8080",        // Порт сервера
	//	"./server.crt", // Путь к сертификату
	//	"./server.key", // Путь к ключу
	//	server.LoggerContextMiddleware(logger),
	//	server.RecoverMiddleware,
	//	server.LoggingMiddleware,
	//)
	//app.RegisterShutdown("simple_https_server", simpleHttpServerShutdownFunctionHttps, 1)

	<-father.Done()
}

func handlerList(handlers *http_handler.Handlers) func(simple *server.HTTPServer) {
	return func(simple *server.HTTPServer) {
		// http://localhost:3000/swagger/index.html
		simple.Router.PathPrefix("/swagger/").Handler(httpSwagger.WrapHandler)

		simple.Router.Handle("/health", http.HandlerFunc(
			func(writer http.ResponseWriter, request *http.Request) {
				writer.WriteHeader(http.StatusOK)
				log.Println("health check")
				return
			})).Methods("GET")

		simple.Router.Handle("/empty", http.HandlerFunc(handlers.EmptyHandler)).Methods("POST")
		//router.HandleFunc("/user/{id}/posts/{postId}", GetPostHandler).Methods("GET")
		//router.HandleFunc("/user/{id:[0-9]+}/posts/{postId:[0-9]+}", GetPostHandler).Methods("POST")
	}
}
