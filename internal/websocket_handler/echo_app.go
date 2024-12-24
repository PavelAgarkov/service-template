package websocket_handler

import (
	"context"
	"github.com/gorilla/websocket"
	"go.uber.org/zap"
	"net/http"
	"service-template/pkg"
	"service-template/server"
	"time"
)

func (h *Handlers) Echo(father context.Context) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		logger := pkg.LoggerFromCtx(r.Context())
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
		send := make(chan server.Message, 256)
		defer close(send)

		// Запускаем writePump
		go server.WritePump(father, conn, send, logger)

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

		tickerMain := time.NewTicker(500 * time.Nanosecond)
		defer tickerMain.Stop()
		// Основной цикл чтения сообщений
	BREAK:
		for {
			select {
			case <-father.Done():
				logger.Info("Закрытие соединения")
				return
			case <-tickerMain.C:
				messageType, message, err := conn.ReadMessage()
				if err != nil {
					if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
						logger.Error("Неожиданная ошибка при чтении сообщения", zap.Error(err))
					} else {
						break BREAK
					}
				}

				logger.Info("Получено сообщение", zap.String("message", string(message)))

				// Отправляем эхо-сообщение через канал send
				send <- server.Message{Type: messageType, Data: message}
			}
		}
		<-time.NewTimer(5 * time.Second).C
		logger.Info("Соединение закрыто клиентом")
	}
}
