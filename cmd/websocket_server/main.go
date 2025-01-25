package main

import (
	"context"
	"fmt"
	"go.uber.org/dig"
	"net/http"
	"service-template/application"
	"service-template/internal/websocket_handler"
	"service-template/pkg"
	"service-template/server"
)

func main() {
	logger := pkg.NewLogger(pkg.LoggerConfig{ServiceName: "websocket-server", LogPath: "logs/app.log"})
	father, cancel := context.WithCancel(context.Background())
	father = pkg.LoggerWithCtx(father, logger)
	defer cancel()

	container := dig.New()
	app := application.NewApp(father, container, logger)
	app.Start(cancel)
	defer app.Stop()
	defer app.RegisterRecovers()()

	app.RegisterShutdown("logger", func() {
		err := logger.Sync()
		if err != nil {
			logger.Error(fmt.Sprintf("failed to sync logger: %v", err))
		}
	}, 101)

	hub := server.NewHub()
	app.RegisterShutdown("garbage_collector", hub.CollectGarbageConnections(logger), 1)
	upgrader := server.NewUpgrader()
	handlers := websocket_handler.NewHandlers(container, hub, upgrader)
	handlers.RegisterWsRoutes(
		map[string]func(ctx context.Context, message server.Routable) error{
			"second": handlers.SecondHandler(),
		})

	httpServerShutdownFunction := server.CreateHttpServer(
		logger,
		nil,
		handlerList(father, handlers),
		":8081",
		server.LoggerContextMiddleware(logger),
		server.RecoverMiddleware,
		server.LoggingMiddleware,
	)
	app.RegisterShutdown("websocket_http_server", httpServerShutdownFunction, 1)

	//https://chromewebstore.google.com/detail/simple-websocket-client/pfdhoblngboilpfeibdedpjgfnlcodoo
	//openssl req \
	//-x509 \
	//-nodes \
	//-newkey rsa:4096 \
	//-keyout server.key \
	//-out server.crt \
	//-days 365 \
	//-subj "/CN=localhost" \
	//-addext "subjectAltName=DNS:localhost"
	simpleHttpsServerShutdownFunction := server.CreateHttpsServer(
		logger,
		nil,
		handlerList(father, handlers),
		":8080",        // Порт сервера
		"./server.crt", // Путь к сертификату
		"./server.key", // Путь к ключу
		server.LoggerContextMiddleware(logger),
		server.RecoverMiddleware,
		server.LoggingMiddleware,
	)

	app.RegisterShutdown("websocket_https_server", simpleHttpsServerShutdownFunction, 1)
	app.Run()
	logger.Info("app is stopped")
}

func handlerList(father context.Context, handlers *websocket_handler.Handlers) func(simple *server.HTTPServer) {
	return func(simple *server.HTTPServer) {
		simple.Router.PathPrefix("/ws").Handler(http.HandlerFunc(handlers.Ws(father)))

		simple.Router.PathPrefix("/").Handler(
			http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				http.ServeFile(w, r, "ws_client.html")
			},
			))
	}
}
