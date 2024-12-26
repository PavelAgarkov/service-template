package websocket_handler

import (
	"context"
	"service-template/pkg"
	"service-template/server"
)

func (h *Handlers) SecondHandler() func(ctx context.Context, message server.Routable) error {
	return func(ctx context.Context, message server.Routable) error {
		logger := pkg.LoggerFromCtx(ctx)
		logger.Info("SecondHandler")
		return nil
	}
}
