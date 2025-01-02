package websocket_handler

import (
	"context"
	"github.com/gorilla/websocket"
	"go.uber.org/zap"
	"net/http"
	"service-template/pkg"
	"service-template/server"
	"sync"
	"time"
)

func (h *Handlers) Ws(father context.Context) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rmes := recover(); rmes != nil {
				logger := pkg.LoggerFromCtx(r.Context())
				logger.Error("Паника в обработчике /ws", zap.Any("recovered", rmes))
			}
		}()

		logger := pkg.LoggerFromCtx(r.Context())
		//handlerCtx := pkg.LoggerWithCtx(context.Background(), logger)
		logger.Info("Попытка подключения к /ws")

		conn, err := h.upgrader.Upgrade(w, r, nil)
		if err != nil {
			logger.Error("Ошибка при апгрейде", zap.Error(err))
			return
		}
		defer conn.Close()

		client := server.NewClient(h.hub, conn)
		h.hub.Register(client)
		defer h.hub.Unregister(client)

		// Создаем канал для отправки сообщений
		responses := make(chan server.Message, 256)

		conn.SetReadLimit(512)
		// Настраиваем обработчик pong
		err = conn.SetReadDeadline(time.Now().Add(5 * time.Second))
		if err != nil {
			logger.Error("Ошибка при установке времени ожидания чтения", zap.Error(err))
			return
		}
		conn.SetPongHandler(func(string) error {
			err := conn.SetReadDeadline(time.Now().Add(5 * time.Second))
			if err != nil {
				logger.Error("Ошибка при установке времени ожидания чтения в момент pong", zap.Error(err))
				return err
			}
			logger.Info("Получен Pong от клиента")
			return nil
		})

		var wg sync.WaitGroup
		wg.Add(1)
		go func() {
			defer wg.Done()
			client.WritePump(father, responses, logger, 4*time.Second)
			logger.Info("WritePump завершен")
		}()

		// Основной цикл чтения сообщений
	MainLoop:
		for {
			messageType, message, err := conn.ReadMessage()
			if err != nil {
				if websocket.IsCloseError(err, websocket.CloseNoStatusReceived, websocket.CloseAbnormalClosure) {
					logger.Info("Соединение закрыто клиентом")
					break MainLoop
				} else if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway) {
					logger.Error("Неожиданная ошибка при чтении сообщения", zap.Error(err))
				} else {
					logger.Error("Соединение закрыто", zap.Error(err))
					break MainLoop
				}
			}

			msg, err := h.wsrouter.AssertMessageType(message)
			if err != nil {
				logger.Error("Ошибка при декодировании сообщения", zap.Error(err))
				responses <- server.Message{Type: messageType, Data: []byte("Ошибка при декодировании сообщения")}
				continue
			}

			//err = h.wsrouter.HandleMessage(handlerCtx, msg)
			err = h.wsrouter.HandleMessage(r.Context(), msg)
			if err != nil {
				logger.Error("Ошибка при обработке сообщения", zap.Error(err))
				responses <- server.Message{Type: messageType, Data: []byte("Ошибка при обработке сообщения")}
				continue
			}

			if father.Err() != nil {
				responses <- server.Message{Type: websocket.CloseMessage, Data: []byte{}}
				logger.Info("Закрытие соединения, завершение работы")
				break MainLoop
			}

			logger.Info("Получено сообщение", zap.String("message", msg.Route()))

			// Отправляем эхо-сообщение через канал send
			responses <- server.Message{Type: messageType, Data: message}
		}
		// Закрываем канал send и ждем завершения writePump, чтобы не случилась паника
		close(responses)
		wg.Wait()
		logger.Info("Обработка соединения завершена")
	}
}
