package main

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"github.com/gorilla/websocket"
	"go.uber.org/zap"
	"log"
	"net/url"
	"os"
	"os/signal"
	"service-template/application"
	"service-template/pkg"
	"service-template/server"
	"syscall"
	"time"
)

func main() {
	logger := pkg.NewLogger("grpc_server", "logs/app.log")
	father, cancel := context.WithCancel(context.Background())
	father = pkg.LoggerWithCtx(father, logger)
	defer cancel()

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGTERM, syscall.SIGINT)
	defer signal.Stop(sig)

	go func() {
		<-sig
		logger.Info("Signal received. Shutting down gRPC client...")
		cancel()
	}()

	app := application.NewApp(logger)
	app.RegisterShutdown("logger", func() {
		err := logger.Sync()
		if err != nil {
			log.Println(fmt.Sprintf("failed to sync logger: %v", err))
		}
	}, 101)

	u := url.URL{Scheme: "wss", Host: "localhost:8080", Path: "/ws"}
	log.Printf("Подключаемся к %s", u.String())

	// === Готовим кастомный dialer с TLS-настройками ===
	dialer := *websocket.DefaultDialer

	// Загружаем системный пул сертификатов
	rootCAs, err := x509.SystemCertPool()
	if err != nil {
		// Если не удалось загрузить, создаём новый
		rootCAs = x509.NewCertPool()
	}

	// Читаем самоподписанный сертификат сервера (public)
	certBytes, err := os.ReadFile("server.crt")
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

	// Выполняем подключение
	c, resp, err := dialer.Dial(u.String(), nil)
	if err != nil {
		if resp != nil {
			log.Printf("Код статуса HTTP при попытке подключения: %d", resp.StatusCode)
		}
		log.Fatal("Ошибка Dial:", err)
	}
	app.RegisterShutdown("websocket_connection", func() { c.Close() }, 1)

	c.SetPingHandler(func(appData string) error {
		fmt.Println("Получен ping:", appData)

		deadline := time.Now().Add(time.Second * 5) // на случай, если сервер ожидает быстрый ответ
		return c.WriteControl(websocket.PongMessage, []byte(appData), deadline)
	})

	message := &server.SecondRequestWsMessage{
		Message: "Hello from client!",
		Routing: "second",
	}
	marshals, _ := json.Marshal(message)
	// Отправляем простое сообщение
	err = c.WriteMessage(websocket.TextMessage, marshals)
	if err != nil {
		log.Println("Ошибка при отправке:", err)
		return
	}

	// Читаем ответы в цикле

	go func() {
	MainLoop:
		for {
			messageType, message, err := c.ReadMessage()
			if err != nil {
				logger.Error("Ошибка при чтении сообщения:", zap.Error(err))
				break MainLoop
			}

			err = c.WriteMessage(websocket.TextMessage, marshals)
			if err != nil {
				logger.Error("Ошибка при отправке:", zap.Error(err))
				continue
			}

			logger.Info(
				"Получено сообщение (тип=%d): %s\n",
				zap.Int("type", messageType),
				zap.String("message", string(message)),
			)
		}
	}()

	//openssl req \
	//-x509 \
	//-nodes \
	//-newkey rsa:4096 \
	//-keyout server.key \
	//-out server.crt \
	//-days 365 \
	//-subj "/CN=localhost" \
	//-addext "subjectAltName=DNS:localhost"

	<-father.Done()
	app.Stop()
	logger.Info("Родительский контекст завершён")
}
