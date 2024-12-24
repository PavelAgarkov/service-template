package websocket_handler

import (
	"github.com/gorilla/websocket"
	"service-template/internal"
	"service-template/server"
)

type Handlers struct {
	globalContainer *internal.Container
	hub             *server.Hub
	upgrader        *websocket.Upgrader
}

func NewHandlers(globalContainer *internal.Container, hub *server.Hub, upgrader *websocket.Upgrader) *Handlers {
	return &Handlers{
		globalContainer: globalContainer,
		hub:             hub,
		upgrader:        upgrader,
	}
}

func (h *Handlers) Container() *internal.Container {
	return h.globalContainer
}
