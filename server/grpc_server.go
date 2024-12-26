package server

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"log"
	"net"
	"os"
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

func loadTLSCredentials(serverCert, serverKey string) (credentials.TransportCredentials, error) {
	creds, err := credentials.NewServerTLSFromFile(serverCert, serverKey)
	if err != nil {
		return nil, fmt.Errorf("failed to load TLS credentials: %v", err)
	}

	return creds, nil
}

func (s *MyGRPCServer) StartTLS(
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

func CreateGRPCServerTLS(serverCert, serverKey string,
	registerServices func(*grpc.Server), port string,
	logger *zap.Logger,
) func() {
	grpcServer := newMyGRPCServer(logger, port)
	shutdownFunc := grpcServer.StartTLS(serverCert, serverKey, registerServices)
	return shutdownFunc
}

type GRPCClientConnection struct {
	ClientConnection *grpc.ClientConn
}

func NewGRPCClientConnection(target string) (*GRPCClientConnection, func() error, error) {
	conn, err := grpc.NewClient(
		target,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		log.Fatalf("Failed to connect to gRPC server: %v", err)
		return nil, nil, err
	}
	return &GRPCClientConnection{
		ClientConnection: conn,
	}, conn.Close, nil
}

func NewGRPCSClientConnection(target string, crt string) (*GRPCClientConnection, func() error, error) {
	certData, err := os.ReadFile(crt)
	if err != nil {
		log.Fatalf("Could not read certificate file: %v", err)
		return nil, nil, err
	}

	certPool := x509.NewCertPool()
	if ok := certPool.AppendCertsFromPEM(certData); !ok {
		log.Fatalf("Failed to append cert from PEM")
		return nil, nil, err
	}

	tlsConfig := &tls.Config{
		RootCAs: certPool,
		// При необходимости можно указать имя хоста (CN/SAN), который проверяется в сертификате:
		// ServerName: "example.com",
		// Если сертификат выписан на "localhost", оставляем пустым или проставляем "localhost".
	}
	creds := credentials.NewTLS(tlsConfig)

	//conn, err := grpc.Dial("localhost:50051", grpc.WithInsecure())
	clientConn, err := grpc.NewClient(
		target,
		grpc.WithTransportCredentials(creds),
	)
	if err != nil {
		log.Fatalf("Failed to connect to gRPC server: %v", err)
		return nil, nil, err
	}
	return &GRPCClientConnection{ClientConnection: clientConn}, clientConn.Close, nil
}
