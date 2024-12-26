package server

import (
	"crypto/tls"
	"crypto/x509"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"os"
)

type GRPCClientConnection struct {
	ClientConnection *grpc.ClientConn
}

func NewGRPCClientConnection(target string, logger *zap.Logger) (*GRPCClientConnection, func() error, error) {
	conn, err := grpc.NewClient(
		target,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		logger.Fatal("Failed to connect to gRPC server: %v", zap.Error(err))
		return nil, nil, err
	}
	return &GRPCClientConnection{
		ClientConnection: conn,
	}, conn.Close, nil
}

func NewGRPCSClientConnection(target string, crt string, logger *zap.Logger) (*GRPCClientConnection, func() error, error) {
	certData, err := os.ReadFile(crt)
	if err != nil {
		logger.Fatal("Could not read certificate file: %v", zap.Error(err))
		return nil, nil, err
	}

	certPool := x509.NewCertPool()
	if ok := certPool.AppendCertsFromPEM(certData); !ok {
		logger.Fatal("Failed to append cert from PEM", zap.Error(err))
		return nil, nil, err
	}

	tlsConfig := &tls.Config{
		RootCAs: certPool,
		// При необходимости можно указать имя хоста (CN/SAN), который проверяется в сертификате:
		// ServerName: "example.com",
		// Если сертификат выписан на "localhost", оставляем пустым или проставляем "localhost".
	}
	creds := credentials.NewTLS(tlsConfig)

	clientConn, err := grpc.NewClient(
		target,
		grpc.WithTransportCredentials(creds),
	)
	if err != nil {
		logger.Fatal("Failed to connect to gRPC server: %v", zap.Error(err))
		return nil, nil, err
	}
	return &GRPCClientConnection{ClientConnection: clientConn}, clientConn.Close, nil
}
