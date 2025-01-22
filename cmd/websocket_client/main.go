package main

import (
	"context"
	"fmt"
	"github.com/gorilla/websocket"
	"go.uber.org/dig"
	"net/url"
	"os"
	"os/signal"
	"service-template/application"
	"service-template/internal/websocket_client"
	"service-template/pkg"
	"sync"
	"syscall"
)

func main() {
	logger := pkg.NewLogger(pkg.LoggerConfig{ServiceName: "grpc_server", LogPath: "logs/app.log"})
	father, cancel := context.WithCancel(context.Background())
	father = pkg.LoggerWithCtx(father, logger)
	defer cancel()

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGTERM, syscall.SIGINT, syscall.SIGKILL)
	defer signal.Stop(sig)

	go func() {
		<-sig
		logger.Info("Signal received. Shutting down websocket client...")
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

	container := dig.New()
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
	<-father.Done()
	wg.Wait()
	logger.Info("Родительский контекст завершён")
}
