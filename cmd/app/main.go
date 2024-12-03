package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"service-template/application"
	"service-template/internal"
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

	container := internal.NewContainer().
		Set("serializer", serializer)

	simpleService := service.NewSimple()
	handlers := handler.NewHandlers(container, simpleService)

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
