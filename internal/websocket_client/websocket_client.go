package websocket_client

import (
	"go.uber.org/dig"
)

type Handlers struct {
	dig *dig.Container
}

func NewHandlers(
	dig *dig.Container,
) *Handlers {
	return &Handlers{
		dig: dig,
	}
}
