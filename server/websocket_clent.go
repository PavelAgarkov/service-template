package server

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"github.com/gorilla/websocket"
	"go.uber.org/zap"
	"log"
	"net/url"
	"os"
)

type WebSocketClientConnection struct {
	Connection *websocket.Conn
}

func NewWebSocketHttpClientConnection(dialer *websocket.Dialer, u *url.URL) (*WebSocketClientConnection, func() error, error) {
	conn, resp, err := dialer.Dial(u.String(), nil)
	if err != nil {
		if resp != nil {
			log.Printf("Код статуса HTTP при попытке подключения: %d", resp.StatusCode)
			return nil, nil, err
		}
		log.Fatal("Ошибка Dial:", err)
		return nil, nil, err
	}
	return &WebSocketClientConnection{
		Connection: conn,
	}, conn.Close, nil
}

func NewWebSocketHttpsClientConnection(dialer *websocket.Dialer, u url.URL, crt string) (*WebSocketClientConnection, func() error, error) {
	rootCAs, err := x509.SystemCertPool()
	if err != nil {
		// Если не удалось загрузить, создаём новый
		rootCAs = x509.NewCertPool()
	}

	// Читаем самоподписанный сертификат сервера (public)
	certBytes, err := os.ReadFile(crt)
	if err != nil {
		log.Fatalf("Не удалось прочитать файл сертификата server.crt: %v", err)
	}

	// Добавляем считанный сертификат в пул rootCAs
	if ok := rootCAs.AppendCertsFromPEM(certBytes); !ok {
		log.Fatal("Не удалось добавить сертификат в пул корневых сертификатов")
	}

	// Настраиваем конфиг с проверкой сертификата
	dialer.TLSClientConfig = &tls.Config{
		RootCAs: rootCAs,
	}
	conn, resp, err := dialer.Dial(u.String(), nil)
	if err != nil {
		if resp != nil {
			log.Printf("Код статуса HTTPS при попытке подключения: %d", resp.StatusCode)
			return nil, nil, err
		}
		log.Fatal("Ошибка Dial:", err)
		return nil, nil, err
	}
	return &WebSocketClientConnection{
		Connection: conn,
	}, conn.Close, nil
}

func (c *WebSocketClientConnection) WritePump(father context.Context, responses chan Message, logger *zap.Logger) {
WriteLoop:
	for {
		select {
		case <-father.Done():
			logger.Info("Закрытие соединения")
			return
		case msg, ok := <-responses:
			if !ok {
				err := c.Connection.WriteMessage(websocket.CloseMessage, []byte{})
				if err != nil {
					logger.Info("Сервер уже закрыл соединение, поэтому не доставляется сообщение о закрытии")
					return
				} else {
					logger.Info("Сообщение закрытия отправлено")
				}
				return
			}
			err := c.Connection.WriteMessage(msg.Type, msg.Data)
			if err != nil {
				logger.Error("Ошибка при отправке сообщения", zap.Error(err))
				break WriteLoop
			}
		}
	}
}
