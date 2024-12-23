package server

import (
	"fmt"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"log"
	"net"
)

type MyGRPCServer struct {
	port   string
	server *grpc.Server
	logger *zap.Logger
}

// NewMyGRPCServer создает новый экземпляр gRPC сервера.
func newMyGRPCServer(logger *zap.Logger, port string) *MyGRPCServer {
	return &MyGRPCServer{
		port:   port,
		logger: logger,
	}
}

// Start запускает gRPC сервер.
func (s *MyGRPCServer) Start(registerServices func(*grpc.Server), interceptors ...grpc.ServerOption) func() {
	// Создаём gRPC сервер.
	s.server = grpc.NewServer(interceptors...)

	// Регистрируем службы.
	registerServices(s.server)

	// Включаем reflection (для gRPC клиентов вроде evans).
	//reflection.Register(s.server)

	listener, err := net.Listen("tcp", s.port)
	if err != nil {
		log.Fatalf("Failed to listen on port %s: %v", s.port, err)
	}

	go func() {
		//l := pkg.GetLogger()
		defer func() {
			if r := recover(); r != nil {
				s.logger.Error(fmt.Sprintf("Recovered from gRPC server: %v", r))
			}
		}()
		s.logger.Info(fmt.Sprintf("gRPC server is running on %s", s.port))
		if err = s.server.Serve(listener); err != nil {
			panic(fmt.Sprintf("Server gRPC stopped by error: %v", err))
		}
	}()

	return s.shutdown
}

// Shutdown завершает работу сервера.
func (s *MyGRPCServer) shutdown() {
	s.logger.Info("Shutting down gRPC server...")
	s.server.GracefulStop()
	s.logger.Info("gRPC server has stopped.")
}

// CreateGRPCServer создаёт и запускает gRPC сервер.
func CreateGRPCServer(registerServices func(*grpc.Server), port string, logger *zap.Logger) func() {
	grpcServer := newMyGRPCServer(logger, port)
	shutdownFunc := grpcServer.Start(registerServices)
	return shutdownFunc
}
