package server

import (
	"fmt"
	"log"
	"net"

	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

type MyGRPCServer struct {
	port   string
	server *grpc.Server
}

// NewMyGRPCServer создает новый экземпляр gRPC сервера.
func newMyGRPCServer(port string) *MyGRPCServer {
	return &MyGRPCServer{
		port: port,
	}
}

// Start запускает gRPC сервер.
func (s *MyGRPCServer) Start(registerServices func(*grpc.Server)) func() {
	// Создаём gRPC сервер.
	s.server = grpc.NewServer()

	// Регистрируем службы.
	registerServices(s.server)

	// Включаем reflection (для gRPC клиентов вроде evans).
	reflection.Register(s.server)

	listener, err := net.Listen("tcp", s.port)
	if err != nil {
		log.Fatalf("Failed to listen on port %s: %v", s.port, err)
	}

	go func() {
		log.Printf("gRPC server is running on %s", s.port)
		if err = s.server.Serve(listener); err != nil {
			panic(fmt.Sprintf("Server gRPC stopped by error: %v", err))
		}
	}()

	return s.Shutdown
}

// Shutdown завершает работу сервера.
func (s *MyGRPCServer) Shutdown() {
	log.Println("Shutting down gRPC server...")
	s.server.GracefulStop()
	log.Println("gRPC server has stopped.")
}

// CreateGRPCServer создаёт и запускает gRPC сервер.
func CreateGRPCServer(registerServices func(*grpc.Server), port string) func() {
	grpcServer := newMyGRPCServer(port)
	shutdownFunc := grpcServer.Start(registerServices)
	return shutdownFunc
}
