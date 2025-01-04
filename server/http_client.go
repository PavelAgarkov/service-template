package server

import (
	"bufio"
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"go.uber.org/zap"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
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

func NewHttpClientConnection(target url.URL, timeout time.Duration) *HttpClientConnection {
	singleHostTransport := &singleHostRoundTripper{
		host: target,
		rt:   http.DefaultTransport,
	}

	client := &http.Client{
		Transport: singleHostTransport,
		Timeout:   timeout,
	}

	return &HttpClientConnection{
		ClientConnection: client,
		baseURL:          target,
	}
}

func NewHttpsClientConnection(target url.URL, crt string, logger *zap.Logger, timeout time.Duration) (*HttpClientConnection, error) {
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
		Timeout:   timeout,
	}

	return &HttpClientConnection{
		ClientConnection: client,
		baseURL:          target,
	}, nil
}

// StartListen открывает SSE-соединение и непрерывно читает события
// пока контекст не будет отменён (или соединение не разорвётся).
func (c *HttpClientConnection) StartListen(
	ctx context.Context,
	onData func(message string),
	logger *zap.Logger,
) error {

	req, err := http.NewRequestWithContext(ctx, "GET", c.baseURL.String()+"/events", nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	// Возможно, вам нужно передать заголовки:
	req.Header.Set("Accept", "text/event-stream")

	resp, err := c.ClientConnection.Do(req)
	if err != nil {
		return fmt.Errorf("failed to do request: %w", err)
	}

	// В идеале проверяем Content-Type
	contentType := resp.Header.Get("Content-Type")
	if !strings.HasPrefix(contentType, "text/event-stream") {
		_ = resp.Body.Close()
		return fmt.Errorf("unexpected Content-Type: %s", contentType)
	}

	// Создаём читатель по строчкам
	reader := bufio.NewReader(resp.Body)

	// Читаем до тех пор, пока не выйдет из строя соединение или контекст
	for {
		select {
		case <-ctx.Done():
			// При отмене контекста аккуратно закрываем resp.Body
			_ = resp.Body.Close()
			return ctx.Err()
		default:
		}

		// Считываем строку
		line, err := reader.ReadString('\n')
		if err != nil {
			if err == io.EOF {
				// Сервер закрыл соединение
				logger.Info("SSE server closed the connection")
				_ = resp.Body.Close()
				return nil
			}
			// Любая другая ошибка
			_ = resp.Body.Close()
			return fmt.Errorf("read error: %w", err)
		}

		line = strings.TrimSpace(line)

		// Формат SSE:
		// event: someEvent
		// data: someData
		// [blank line - конец события]
		// Для упрощённого примера будем реагировать только на "data:"
		if strings.HasPrefix(line, "data:") {
			// Отрезаем "data: " (можно аккуратно парсить)
			msg := strings.TrimSpace(line[len("data:"):])
			// Вызываем колбэк
			onData(msg)
		}
		// Если нужно обрабатывать event: или id:, можно дописать логику.
	}
}
