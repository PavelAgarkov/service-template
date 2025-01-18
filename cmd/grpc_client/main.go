package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"service-template/application"
	myservice "service-template/cmd/grps_server/pb/myservice/pb"
	myservice2 "service-template/cmd/grps_server/pb/myservice2/pb"
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
	defer app.Stop()

	app.RegisterShutdown("logger", func() {
		err := logger.Sync()
		if err != nil {
			log.Println(fmt.Sprintf("failed to sync logger: %v", err))
		}
	}, 101)
	defer app.RegisterRecovers(logger, sig)()

	grpcsClient, shutdown, _ := server.NewGRPCSClientConnection(
		"localhost:50052",
		"./server.crt",
		logger,
	)
	app.RegisterShutdown("grpcsClient", func() { shutdown() }, 1)

	grpcClient, shutdown, _ := server.NewGRPCClientConnection("localhost:50051", logger)
	app.RegisterShutdown("grpcClient", func() { shutdown() }, 1)

	DoWithTLS(grpcsClient)
	Do(grpcClient)

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

func DoWithTLS(grpcClient *server.GRPCClientConnection) {
	client := myservice.NewMyServiceClient(grpcClient.ClientConnection)
	client2 := myservice2.NewMyServiceClient(grpcClient.ClientConnection)

	resp, err := client.SayHello(context.Background(), &myservice.HelloRequest{Name: "Name1"})
	if err != nil {
		log.Fatalf("Failed to call MyMethod: %v", err)
	}
	log.Printf("Response from MyMethod: %v", resp)

	resp2, err := client2.SayHello(context.Background(), &myservice2.HelloRequest{Name: "Name2"})
	if err != nil {
		log.Fatalf("Failed to call MyMethod: %v", err)
	}
	log.Printf("Response from MyMethod: %v", resp2)
}

func Do(grpcClient *server.GRPCClientConnection) {
	client := myservice.NewMyServiceClient(grpcClient.ClientConnection)
	client2 := myservice2.NewMyServiceClient(grpcClient.ClientConnection)

	resp, err := client.SayHello(context.Background(), &myservice.HelloRequest{Name: "Name1"})
	if err != nil {
		log.Fatalf("Failed to call MyMethod: %v", err)
	}
	log.Printf("Response from MyMethod: %v", resp)

	resp2, err := client2.SayHello(context.Background(), &myservice2.HelloRequest{Name: "Name2"})
	if err != nil {
		log.Fatalf("Failed to call MyMethod: %v", err)
	}
	log.Printf("Response from MyMethod: %v", resp2)
}
