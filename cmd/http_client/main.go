package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
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

	httpClient := server.NewHttpClientConnection("http://localhost:8081")
	httpsClient, _ := server.NewHttpsClientConnection("https://localhost:8080", "./server.crt", logger)

	Do(httpClient)
	Do(httpsClient)

	//openssl req \
	//-x509 \
	//-nodes \
	//-newkey rsa:4096 \
	//-keyout server.key \
	//-out server.crt \
	//-days 365 \
	//-subj "/CN=localhost" \
	//-addext "subjectAltName=DNS:localhost"

	<-father.Done()
}

func Do(httpsClient *server.HttpClientConnection) {
	data := []byte(`{
  "name": "John1",
  "age": 30,
  "email": "a@mail.ru"
}`)

	requestBody := bytes.NewBuffer(data)

	resp, err := httpsClient.ClientConnection.Post(httpsClient.GetBaseURL()+"/empty", "application/json", requestBody)
	if err != nil {
		log.Fatalf("Failed to send POST request: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Fatalf("Failed to read response body: %v", err)
	}

	fmt.Println("Status:", resp.Status)
	fmt.Println("Body:", string(body))
}
