package main

import (
	"context"
	"fmt"
	"log"
	"service-template/application"
	myservice "service-template/cmd/grps_server/pb/myservice/pb"
	myservice2 "service-template/cmd/grps_server/pb/myservice2/pb"
	"service-template/pkg"
	"service-template/server"
)

func main() {
	logger := pkg.NewLogger(pkg.LoggerConfig{ServiceName: "grpc_server", LogPath: "logs/app.log"})
	father, cancel := context.WithCancel(context.Background())
	father = pkg.LoggerWithCtx(father, logger)
	defer cancel()

	app := application.NewApp(father, nil, logger)
	app.Start(cancel)
	defer app.Stop()
	defer app.RegisterRecovers()()

	app.RegisterShutdown("logger", func() {
		err := logger.Sync()
		if err != nil {
			log.Println(fmt.Sprintf("failed to sync logger: %v", err))
		}
	}, 101)

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

	app.Run()
}

func DoWithTLS(grpcClient *server.GRPCClientConnection) {
	client := myservice.NewMyServiceClient(grpcClient.ClientConnection)
	client2 := myservice2.NewMyServiceClient(grpcClient.ClientConnection)

	resp, err := client.SayHello(context.Background(), &myservice.HelloRequest{Name: "Name1"})
	if err != nil {
		log.Println("Failed to call MyMethod: %v", err)
	}
	log.Printf("Response from MyMethod: %v", resp)

	resp2, err := client2.SayHello(context.Background(), &myservice2.HelloRequest{Name: "Name2"})
	if err != nil {
		log.Println("Failed to call MyMethod: %v", err)
	}
	log.Printf("Response from MyMethod: %v", resp2)
}

func Do(grpcClient *server.GRPCClientConnection) {
	client := myservice.NewMyServiceClient(grpcClient.ClientConnection)
	client2 := myservice2.NewMyServiceClient(grpcClient.ClientConnection)

	resp, err := client.SayHello(context.Background(), &myservice.HelloRequest{Name: "Name1"})
	if err != nil {
		log.Println("Failed to call MyMethod: %v", err)
	}
	log.Printf("Response from MyMethod: %v", resp)

	resp2, err := client2.SayHello(context.Background(), &myservice2.HelloRequest{Name: "Name2"})
	if err != nil {
		log.Println("Failed to call MyMethod: %v", err)
	}
	log.Printf("Response from MyMethod: %v", resp2)
}
