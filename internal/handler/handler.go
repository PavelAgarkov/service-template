package handler

import (
	"service-template/internal"
)

type Handlers struct {
	globalContainer *internal.Container
}

func NewHandlers(globalContainer *internal.Container) *Handlers {
	return &Handlers{
		globalContainer: globalContainer,
	}
}

func (h *Handlers) Container() *internal.Container {
	return h.globalContainer
}
