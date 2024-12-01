package handler

import (
	"service-template/internal"
	"service-template/internal/service"
)

type Handlers struct {
	globalContainer *internal.Container
	simpleService   service.Simple
}

func NewHandlers(globalContainer *internal.Container, simpleService *service.Simple) *Handlers {
	return &Handlers{
		globalContainer: globalContainer,
		simpleService:   *simpleService,
	}
}

func (h *Handlers) Container() *internal.Container {
	return h.globalContainer
}

//func (h *Handlers) GetHandlersList() string {
//	return "HandlersList"
//}

//type HandlersList interface {
//	EmptyHandler(w http.ResponseWriter, r *http.Request)
//}
