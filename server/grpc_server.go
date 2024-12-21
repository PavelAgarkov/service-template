package server

import (
	"fmt"
	"log"
	"net"
	"service-template/pkg"

	"google.golang.org/grpc"
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
	//reflection.Register(s.server)

	listener, err := net.Listen("tcp", s.port)
	if err != nil {
		log.Fatalf("Failed to listen on port %s: %v", s.port, err)
	}

	go func() {
		l := pkg.GetLogger()
		defer func() {
			if r := recover(); r != nil {
				l.Error(fmt.Sprintf("Recovered from gRPC server: %v", r))
			}
		}()
		l.Info(fmt.Sprintf("gRPC server is running on %s", s.port))
		if err = s.server.Serve(listener); err != nil {
			panic(fmt.Sprintf("Server gRPC stopped by error: %v", err))
		}
	}()

	return s.shutdown
}

// Shutdown завершает работу сервера.
func (s *MyGRPCServer) shutdown() {
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
