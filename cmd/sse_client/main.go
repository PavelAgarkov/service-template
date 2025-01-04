package main

import (
	"context"
	"fmt"
	"go.uber.org/zap"
	"log"
	"net/url"
	"os"
	"os/signal"
	"service-template/application"
	"service-template/pkg"
	"service-template/server"
	"syscall"
)

func main() {
	logger := pkg.NewLogger("grpc_server", "logs/app.log")
	father, cancel := context.WithCancel(context.Background())
	father = pkg.LoggerWithCtx(father, logger)
	defer cancel()

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGTERM, syscall.SIGINT)
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

	httpClient := server.NewHttpClientConnection(url.URL{Scheme: "http", Host: "localhost:8081"}, 0)
	httpsClient, _ := server.NewHttpsClientConnection(url.URL{Scheme: "https", Host: "localhost:8080"}, "./server.crt", logger, 0)

	//openssl req \
	//-x509 \
	//-nodes \
	//-newkey rsa:4096 \
	//-keyout server.key \
	//-out server.crt \
	//-days 365 \
	//-subj "/CN=localhost" \
	//-addext "subjectAltName=DNS:localhost"
	go listen(father, httpsClient, logger)
	go listen(father, httpClient, logger)

	<-father.Done()
	app.Stop()
}

func listen(father context.Context, httpsClient *server.HttpClientConnection, logger *zap.Logger) {
	err := httpsClient.StartListen(father, func(message string) {
		// Обработка поступившего сообщения
		logger.Info("Got SSE message:", zap.String("message", message))
	}, logger)
	if err != nil {
		logger.Error("SSE listen error", zap.Error(err))
	}
	logger.Info("sse was stopped")
}
