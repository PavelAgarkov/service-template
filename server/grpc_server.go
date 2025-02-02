package server

import (
	"fmt"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"log"
	"net"
)

type GRPCServer struct {
	port   string
	server *grpc.Server
	logger *zap.Logger
}

// NewMyGRPCServer создает новый экземпляр gRPC сервера.
func newGRPCServer(logger *zap.Logger, port string) *GRPCServer {
	return &GRPCServer{
		port:   port,
		logger: logger,
	}
}

func loadTLSCredentials(serverCert, serverKey string) (credentials.TransportCredentials, error) {
	creds, err := credentials.NewServerTLSFromFile(serverCert, serverKey)
	if err != nil {
		return nil, fmt.Errorf("failed to load TLS credentials: %v", err)
	}

	return creds, nil
}

func (s *GRPCServer) StartTLS(
	serverCert, serverKey string,
	registerServices func(*grpc.Server), interceptors ...grpc.ServerOption,
) func() {
	creds, err := loadTLSCredentials(serverCert, serverKey)
	if err != nil {
		log.Fatalf("Failed to load TLS credentials: %v", err)
	}

	// Добавляем TLS-credentials как опцию сервера
	grpcOptions := append(interceptors, grpc.Creds(creds))

	// Создаём gRPC сервер.
	s.server = grpc.NewServer(grpcOptions...)

	// Регистрируем службы.
	registerServices(s.server)

	// Включаем reflection (для gRPC клиентов вроде evans).
	//reflection.Register(s.server)

	listener, err := net.Listen("tcp", s.port)
	if err != nil {
		log.Fatalf("Failed to listen on port %s: %v", s.port, err)
	}

	go func() {
		s.logger.Info(fmt.Sprintf("gRPC server is running on %s", s.port))
		if err = s.server.Serve(listener); err != nil {
			panic(fmt.Sprintf("Server gRPC stopped by error: %v", err))
		}
	}()

	return s.shutdown
}

// Start запускает gRPC сервер.
func (s *GRPCServer) Start(registerServices func(*grpc.Server), interceptors ...grpc.ServerOption) func() {
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
		s.logger.Info(fmt.Sprintf("gRPC server is running on %s", s.port))
		if err = s.server.Serve(listener); err != nil {
			panic(fmt.Sprintf("Server gRPC stopped by error: %v", err))
		}
	}()

	return s.shutdown
}

// Shutdown завершает работу сервера.
func (s *GRPCServer) shutdown() {
	s.logger.Info("Shutting down gRPC server...")
	s.server.GracefulStop()
	s.logger.Info("gRPC server has stopped.")
}

// CreateGRPCServer создаёт и запускает gRPC сервер.
func CreateGRPCServer(registerServices func(*grpc.Server), port string, logger *zap.Logger) func() {
	grpcServer := newGRPCServer(logger, port)
	shutdownFunc := grpcServer.Start(registerServices)
	return shutdownFunc
}

func CreateGRPCServerTLS(serverCert, serverKey string,
	registerServices func(*grpc.Server), port string,
	logger *zap.Logger,
) func() {
	grpcServer := newGRPCServer(logger, port)
	shutdownFunc := grpcServer.StartTLS(serverCert, serverKey, registerServices)
	return shutdownFunc
}
