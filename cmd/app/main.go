package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"service-template/application"
	"service-template/internal"
	"service-template/internal/config"
	"service-template/internal/handler"
	"service-template/internal/logger"
	"service-template/internal/service"
	"service-template/pkg"
	"service-template/server"
	"syscall"
)

func main() {
	father, cancel := context.WithCancel(context.Background())
	father = logger.WithCtx(father, logger.Get())
	defer cancel()

	logger.FromCtx(father).Info("config initializing")
	cfg := config.GetConfig()

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGTERM, syscall.SIGINT)
	defer signal.Stop(sig)

	go func() {
		<-sig
		logger.FromCtx(father).Info("Signal received. Shutting down server...")
		cancel()
	}()

	app := application.NewApp()
	serializer := pkg.NewSerializer()

	postgres, postgresShutdown := pkg.NewPostgres(cfg.DB.Host, cfg.DB.Port, cfg.DB.Username, cfg.DB.Password, cfg.DB.Database, "disable")
	app.RegisterShutdown("postgres", postgresShutdown, 100)
	//srv := service.NewSrv()

	container := internal.NewContainer().
		Set("serializer", serializer).
		Set("postgres", postgres).
		Set("service.simple", service.NewSrv(), "serializer", "postgres")

	handlers := handler.NewHandlers(container)

	simpleHttpServerShutdownFunction := server.CreateHttpServer(
		handlerList(handlers),
		":3000",
		server.RecoverMiddleware,
		server.LoggingMiddleware,
	)
	app.RegisterShutdown("simple_http_server", simpleHttpServerShutdownFunction, 1)

	simpleHttpServerShutdownFunction2 := server.CreateHttpServer(
		handlerList(handlers),
		":3001",
		server.RecoverMiddleware,
		server.LoggingMiddleware,
	)
	app.RegisterShutdown("simple_http_server", simpleHttpServerShutdownFunction2, 0)

	<-father.Done()
	app.Stop()
	logger.FromCtx(father).Info("app is shutting down")
}

func handlerList(handlers *handler.Handlers) func(simple *server.SimpleHTTPServer) {
	return func(simple *server.SimpleHTTPServer) {
		simple.Router.Handle("/", http.HandlerFunc(handlers.EmptyHandler)).Methods("POST")
	}
}
