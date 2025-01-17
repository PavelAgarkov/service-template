package main

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/signal"
	"service-template/application"
	"service-template/pkg"
	"service-template/server"
	"strings"
	"syscall"
	"time"
)

func main() {
	logger := pkg.NewLogger("websocket-server", "logs/app.log")
	father, cancel := context.WithCancel(context.Background())
	father = pkg.LoggerWithCtx(father, logger)
	defer cancel()

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGTERM, syscall.SIGINT)
	defer signal.Stop(sig)

	go func() {
		<-sig
		logger.Info("Signal received. Shutting down server...")
		cancel()
	}()
	app := application.NewApp(logger)

	httpServerShutdownFunction := server.CreateHttpServer(
		logger,
		nil,
		handlerList(father, "sse_client_http.html"),
		":8081",
		server.LoggerContextMiddleware(logger),
		server.RecoverMiddleware,
		server.LoggingMiddleware,
	)
	app.RegisterShutdown("websocket_http_server", httpServerShutdownFunction, 1)

	//openssl req \
	//-x509 \
	//-nodes \
	//-newkey rsa:4096 \
	//-keyout server.key \
	//-out server.crt \
	//-days 365 \
	//-subj "/CN=localhost" \
	//-addext "subjectAltName=DNS:localhost"
	simpleHttpsServerShutdownFunction := server.CreateHttpsServer(
		logger,
		nil,
		handlerList(father, "sse_client_https.html"),
		":8080",        // Порт сервера
		"./server.crt", // Путь к сертификату
		"./server.key", // Путь к ключу
		server.LoggerContextMiddleware(logger),
		server.RecoverMiddleware,
		server.LoggingMiddleware,
	)

	app.RegisterShutdown("websocket_https_server", simpleHttpsServerShutdownFunction, 1)

	<-father.Done()
	app.Stop()
	logger.Info("app is stopped")
}

func handlerList(father context.Context, clientFrontend string) func(simple *server.HTTPServer) {
	return func(simple *server.HTTPServer) {
		simple.Router.PathPrefix("/events").Handler(http.HandlerFunc(sseHandler))
		simple.Router.PathPrefix("/").Handler(
			http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				http.ServeFile(w, r, clientFrontend)
			},
			))
	}
}

// для реализации broadcast в sse https://github.com/r3labs/sse
// или реализовать самостоятельно записывая все подключенные клиенты в map
// и отправлять сообщения каждому из них по очереди
// Так же можно использовать эту технологию для небольших онлайн игр
// для отправки сообщений о действиях игроков друг другу, исключая отправляющего игрока
func sseHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming unsupported!", http.StatusInternalServerError)
		return
	}

	// Отправим несколько событий с разными параметрами
	sendSSE(w, "alert", "1", "{'message': 'Привет, мир!'}")
	flusher.Flush()
	time.Sleep(1 * time.Second)

	sendSSE(w, "news", "2", `Первая строка Вторая строка Третья строка`)
	flusher.Flush()
	time.Sleep(1 * time.Second)

	sendSSE(w, "", "3", "Событие без имени (onmessage)")
	flusher.Flush()
}

func sendSSE(w io.Writer, event, id, data string) {
	// Если указано имя события
	if event != "" {
		fmt.Fprintf(w, "event: %s\n", event)
	}

	// Если указан идентификатор
	if id != "" {
		fmt.Fprintf(w, "id: %s\n", id)
	}

	// Само содержимое события (data).
	// Если нужно разбить по строкам, используем Split и делаем data: на каждую строку:
	lines := strings.Split(data, "\n")
	for _, line := range lines {
		fmt.Fprintf(w, "data: %s\n", line)
	}

	// Пустая строка отделяет одно событие от другого
	fmt.Fprint(w, "\n")
}
