package main

import (
	"context"
	"github.com/gofiber/fiber/v3"
	"log"
	"os"
	"os/signal"
	"sync"
	"syscall"
)

const (
	address = ":3000"
)

func main() {
	runHTTPServer()
}

func runHTTPServer() {
	ctx, cancel := context.WithCancel(context.Background())

	app := fiber.New()
	sig := make(chan os.Signal)
	signal.Notify(sig, syscall.SIGTERM, syscall.SIGINT, syscall.SIGKILL)

	go func() {
		<-sig
		cancel()
	}()
	var serverShutdown sync.WaitGroup
	go func() {
		<-ctx.Done()
		_ = app.ShutdownWithContext(ctx)
		serverShutdown.Done()
	}()

	if err := app.Listen(address); err != nil {
		log.Fatalf("server is stopped by error %s", err.Error())
	}

	serverShutdown.Wait()
}
