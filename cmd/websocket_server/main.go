package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"service-template/application"
	"service-template/internal"
	"service-template/internal/websocket_handler"
	"service-template/pkg"
	"service-template/server"
	"syscall"
)

func main() {
	logger := pkg.NewLogger("websocket-server", "logs/app.log")
	father, cancel := context.WithCancel(context.Background())
	father = pkg.LoggerWithCtx(father, logger)
	defer cancel()

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGTERM, syscall.SIGINT)
	defer signal.Stop(sig)

	go func() {
		<-sig
		logger.Info("Signal received. Shutting down server...")
		cancel()
	}()
	app := application.NewApp(logger)
	defer func() {
		app.Stop()
		logger.Info("app is stopped")
	}()

	container := internal.NewContainer(logger)

	hub := server.NewHub()
	app.RegisterShutdown("garbage_collector", hub.CollectGarbageConnections(father, logger), 1)
	upgrader := server.NewUpgrader()
	handlers := websocket_handler.NewHandlers(container, hub, upgrader)

	simpleHttpServerShutdownFunction := server.CreateHttpServer(
		logger,
		handlerList(father, handlers),
		":8080",
		server.LoggerContextMiddleware(logger),
		server.RecoverMiddleware,
		server.LoggingMiddleware,
	)
	app.RegisterShutdown("simple_http_server", simpleHttpServerShutdownFunction, 1)
	<-father.Done()
}

func handlerList(father context.Context, handlers *websocket_handler.Handlers) func(simple *server.SimpleHTTPServer) {
	return func(simple *server.SimpleHTTPServer) {
		simple.Router.PathPrefix("/ws").Handler(http.HandlerFunc(handlers.Ws(father)))

		simple.Router.PathPrefix("/").Handler(
			http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				http.ServeFile(w, r, "index.html")
			},
			))
	}
}
