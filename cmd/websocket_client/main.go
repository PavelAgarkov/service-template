package main

import (
	"context"
	"fmt"
	"github.com/gorilla/websocket"
	"go.uber.org/dig"
	"net/url"
	"service-template/application"
	"service-template/internal/websocket_client"
	"service-template/pkg"
	"sync"
)

func main() {
	logger := pkg.NewLogger(pkg.LoggerConfig{ServiceName: "websocket-client", LogPath: "logs/app.log"})
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

	webSocketClientHandler := websocket_client.NewHandlers(container)

	wg := sync.WaitGroup{}
	wg.Add(1)
	go func() {
		defer wg.Done()
		webSocketClientHandler.DoClientApp(
			father,
			websocket.DefaultDialer,
			url.URL{Scheme: "wss", Host: "localhost:8080", Path: "/ws"},
			"./server.crt",
			logger,
		)
	}()

	//openssl req \
	//-x509 \
	//-nodes \
	//-newkey rsa:4096 \
	//-keyout server.key \
	//-out server.crt \
	//-days 365 \
	//-subj "/CN=localhost" \
	//-addext "subjectAltName=DNS:localhost"

	logger.Info("Ожидание завершения работы")
	app.Run()
	wg.Wait()
	logger.Info("Родительский контекст завершён")
}
