package websocket_client

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/chzyer/readline"
	"github.com/gorilla/websocket"
	"go.uber.org/zap"
	"net/url"
	"service-template/server"
	"sync"
	"time"
)

func (h *Handlers) DoClientApp(
	father context.Context,
	dialer *websocket.Dialer,
	u url.URL,
	crt string,
	logger *zap.Logger,
) func() error {
	webSocketClientConnection, shutdown, _ := server.NewWebSocketHttpsClientConnection(dialer, u, crt)

	webSocketClientConnection.Connection.SetPingHandler(func(appData string) error {
		fmt.Println("Получен ping:", appData)
		deadline := time.Now().Add(time.Second * 5) // на случай, если сервер ожидает быстрый ответ
		return webSocketClientConnection.Connection.WriteControl(websocket.PongMessage, []byte(appData), deadline)
	})

	message := &server.SecondRequestWsMessage{
		Message: "Hello from client!",
		Routing: "second",
	}
	responses := make(chan server.Message, 256)

	marshals, _ := json.Marshal(message)
	responses <- server.Message{Type: websocket.TextMessage, Data: marshals}

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		webSocketClientConnection.WritePump(father, responses, logger)
		logger.Info("WritePump завершен")
	}()

	go func() {
		defer func() {
			if r := recover(); r != nil {
				logger.Error("Паника в handleUserInput", zap.Any("recover", r))
			}
		}()
		defer func() {
			logger.Info("Завершение цикла чтения сообщений")
			close(responses)
			logger.Info("Закрытие канала responses")
			wg.Wait()
			logger.Info("Ожидание завершения WritePump")
		}()
	ReadLoop:
		for {
			messageType, message, err := webSocketClientConnection.Connection.ReadMessage()
			if err != nil {
				// Проверяем, если ошибка закрытия соединения ожидаемая
				if websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) {
					logger.Info("Соединение WebSocket закрыто ожидаемо")
				} else {
					logger.Error("Ошибка при чтении сообщения:", zap.Error(err))
				}
				break ReadLoop
			}

			if father.Err() != nil {
				logger.Info("Закрытие соединения, завершение работы")
				break ReadLoop
			}

			responses <- server.Message{Type: websocket.TextMessage, Data: message}

			logger.Info(
				"Получено сообщение (тип=%d): %s\n",
				zap.Int("type", messageType),
				zap.String("message", string(message)),
			)
		}
		logger.Info("Цикл чтения сообщений завершён")
	}()

	logger.Info("Обработка соединения завершена")
	return shutdown
}

// это какой -то бред, такое ощущение что го не умеет нормально работать с stdin
func handleUserInput(ctx context.Context, responses chan<- server.Message, logger *zap.Logger) {
	rl, err := readline.New("Введите сообщение (Ctrl+C для выхода): ")
	if err != nil {
		logger.Fatal("Ошибка создания readline:", zap.Error(err))
	}
	defer rl.Close()

	// Канал для сигнализации завершения работы
	quitChan := make(chan struct{})

	// Горутина для завершения readline при отмене контекста
	go func() {
		<-ctx.Done() // Ожидаем завершения контекста
		close(quitChan)
		rl.Close() // Прерываем блокирующий rl.Readline()
		logger.Info("Контекст завершён, readline закрыт")
	}()

	for {
		select {
		case <-quitChan:
			// Завершаем, если контекст завершён
			logger.Info("Сигнал завершения, выход из handleUserInput")
			return
		default:
			line, err := rl.Readline()
			if err != nil { // EOF, Ctrl+D или rl.Close()
				logger.Info("EOF или завершение ввода")
				rl.Close()
				logger.Info("EOF или завершение ввода")
				return
			}
			if line == "" {
				continue
			}

			// Формируем сообщение и отправляем его в канал
			marshals, err := json.Marshal(&server.SecondRequestWsMessage{
				Message: line,
				Routing: "second",
			})
			if err != nil {
				logger.Error("Ошибка при сериализации:", zap.Error(err))
				continue
			}
			responses <- server.Message{Type: websocket.TextMessage, Data: marshals}
		}
	}
}
