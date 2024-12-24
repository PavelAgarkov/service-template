package server

import (
	"context"
	"github.com/gorilla/websocket"
	"go.uber.org/zap"
	"net/http"
	"sync"
	"time"
)

type Client struct {
	hub *Hub

	conn *websocket.Conn
}

func NewClient(hub *Hub, conn *websocket.Conn) *Client {
	return &Client{
		hub:  hub,
		conn: conn,
	}
}

func (h *Hub) CollectGarbageConnections(father context.Context, logger *zap.Logger) func() {
	return func() {
		h.mu.Lock()
		for client := range h.clients {
			err := client.conn.Close()
			if err != nil {
				logger.Error("Ошибка при закрытии соединения", zap.Error(err))
				continue
			}
			delete(h.clients, client)
			logger.Info("Соединение закрыто и удалено из списка")
		}
		h.mu.Unlock()
	}

}

type Hub struct {
	clients map[*Client]bool
	mu      sync.Mutex
}

func NewHub() *Hub {
	return &Hub{
		clients: make(map[*Client]bool),
	}
}

func (h *Hub) Register(client *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.clients[client] = true
}

func (h *Hub) Unregister(client *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if _, ok := h.clients[client]; ok {
		delete(h.clients, client)
	}
}

func NewUpgrader() *websocket.Upgrader {
	return &websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
		CheckOrigin: func(r *http.Request) bool {
			return true
		},
	}
}

type Message struct {
	Type int
	Data []byte
}

func WritePump(father context.Context, conn *websocket.Conn, send chan Message, logger *zap.Logger) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-father.Done():
			logger.Info("Закрытие соединения")
			return
		case msg, ok := <-send:
			conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if !ok {
				err := conn.WriteMessage(websocket.CloseMessage, []byte{})
				if err != nil {
					logger.Error("Ошибка при отправке сообщения закрытия", zap.Error(err))
					return
				}
				return
			}
			err := conn.WriteMessage(msg.Type, msg.Data)
			if err != nil {
				logger.Error("Ошибка при отправке сообщения", zap.Error(err))
				break
			}
		case <-ticker.C:
			// Дополнительная отправка пингов, если необходимо
			conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				logger.Error("Ошибка при отправке пинга", zap.Error(err))
				return
			}
		}
	}
}
