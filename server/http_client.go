package server

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"go.uber.org/zap"
	"net/http"
	"net/url"
	"os"
	"strings"
)

type HttpClientConnection struct {
	ClientConnection *http.Client
	baseURL          url.URL
}

func (c *HttpClientConnection) GetBaseURL() url.URL {
	return c.baseURL
}

type singleHostRoundTripper struct {
	host url.URL
	rt   http.RoundTripper
}

func (s *singleHostRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	domain := url.URL{Scheme: req.URL.Scheme, Host: req.URL.Host}
	if !strings.EqualFold(domain.String(), s.host.String()) {
		return nil, fmt.Errorf("attempt to request disallowed host: %q (allowed only %q)", req.URL.Host, s.host)
	}
	return s.rt.RoundTrip(req)
}

func NewHttpClientConnection(target url.URL) *HttpClientConnection {
	singleHostTransport := &singleHostRoundTripper{
		host: target,
		rt:   http.DefaultTransport,
	}

	client := &http.Client{
		Transport: singleHostTransport,
	}

	return &HttpClientConnection{
		ClientConnection: client,
		baseURL:          target,
	}
}

func NewHttpsClientConnection(target url.URL, crt string, logger *zap.Logger) (*HttpClientConnection, error) {
	certData, err := os.ReadFile(crt)
	if err != nil {
		logger.Fatal("Could not read certificate file: %v", zap.Error(err))
		return nil, err
	}

	certPool := x509.NewCertPool()
	if ok := certPool.AppendCertsFromPEM(certData); !ok {
		logger.Fatal("Failed to append cert from PEM", zap.Error(err))
		return nil, err
	}

	tlsConfig := &tls.Config{
		RootCAs: certPool,
		// При необходимости можно указать имя хоста (CN/SAN), который проверяется в сертификате:
		// ServerName: "example.com",
		// Если сертификат выписан на "localhost", оставляем пустым или проставляем "localhost".
	}

	baseTransport := &http.Transport{
		TLSClientConfig: tlsConfig,
	}

	singleHostTransport := &singleHostRoundTripper{
		host: target,
		rt:   baseTransport,
	}

	client := &http.Client{
		Transport: singleHostTransport,
	}

	return &HttpClientConnection{
		ClientConnection: client,
		baseURL:          target,
	}, nil
}
