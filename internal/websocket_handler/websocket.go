package websocket_handler

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/gorilla/websocket"
	"service-template/internal"
	"service-template/server"
)

type Handlers struct {
	globalContainer *internal.Container
	hub             *server.Hub
	upgrader        *websocket.Upgrader
	wsrouter        *WsRouter
}

func NewHandlers(
	globalContainer *internal.Container,
	hub *server.Hub,
	upgrader *websocket.Upgrader,
) *Handlers {
	return &Handlers{
		globalContainer: globalContainer,
		hub:             hub,
		upgrader:        upgrader,
	}
}

func (h *Handlers) Container() *internal.Container {
	return h.globalContainer
}

type WsRouter struct {
	routes map[string]func(ctx context.Context, message server.Routable) error
}

func (h *Handlers) RegisterWsRoutes(routes map[string]func(ctx context.Context, message server.Routable) error) {
	h.wsrouter = newWsRouter(routes)
}

func newWsRouter(routes map[string]func(ctx context.Context, message server.Routable) error) *WsRouter {
	return &WsRouter{
		routes: routes,
	}
}

func (r *WsRouter) AssertMessageType(message []byte) (server.Routable, error) {
	var msg server.Routable
	first := &server.FirstRequestWsMessage{}
	if err := json.Unmarshal(message, first); err != nil {
		return nil, err
	}

	switch {
	case first.Routing == "second":
		msg = &server.SecondRequestWsMessage{}
		if err := json.Unmarshal(message, msg); err != nil {
			return nil, err
		}
	default:
		return nil, errors.New("unknown message type")
	}

	return msg, nil
}

func (r *WsRouter) HandleMessage(ctx context.Context, message server.Routable) error {
	return r.routes[message.Route()](ctx, message)
}