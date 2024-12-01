package main

import (
	"context"
	"flick/application"
	"flick/internal"
	"flick/internal/handler"
	"flick/internal/service"
	"flick/pkg"
	"flick/server"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	father, cancel := context.WithCancel(context.Background())
	defer cancel()

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGTERM, syscall.SIGINT)
	defer signal.Stop(sig)

	go func() {
		<-sig
		log.Println("Signal received. Shutting down server...")
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
	log.Printf("app is shutting down")
}

func handlerList(handlers *handler.Handlers) func(simple *server.SimpleHTTPServer) {
	return func(simple *server.SimpleHTTPServer) {
		simple.Router.Handle("/", http.HandlerFunc(handlers.EmptyHandler)).Methods("POST")
	}
}
