package main

import (
	"context"
	"fmt"
	"github.com/gorilla/websocket"
	"log"
	"net/url"
	"os"
	"os/signal"
	"service-template/application"
	"service-template/internal"
	"service-template/internal/websocket_client"
	"service-template/pkg"
	"syscall"
)

func main() {
	logger := pkg.NewLogger("grpc_server", "logs/app.log")
	father, cancel := context.WithCancel(context.Background())
	father = pkg.LoggerWithCtx(father, logger)
	defer cancel()

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGTERM, syscall.SIGINT, syscall.SIGKILL)
	defer signal.Stop(sig)

	go func() {
		<-sig
		logger.Info("Signal received. Shutting down gRPC client...")
		cancel()
	}()

	app := application.NewApp(logger)
	app.RegisterShutdown("logger", func() {
		err := logger.Sync()
		if err != nil {
			log.Println(fmt.Sprintf("failed to sync logger: %v", err))
		}
	}, 101)

	container := internal.NewContainer(logger)
	webSocketClientHandler := websocket_client.NewHandlers(container)

	s := webSocketClientHandler.DoClientApp(
		father,
		websocket.DefaultDialer,
		url.URL{Scheme: "wss", Host: "localhost:8080", Path: "/ws"},
		"./server.crt",
		logger,
	)
	app.RegisterShutdown("ws_client", func() { s() }, 1)

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
	app.Stop()
	logger.Info("Родительский контекст завершён")
}
