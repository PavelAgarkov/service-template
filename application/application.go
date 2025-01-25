package application

import (
	"context"
	"fmt"
	"go.uber.org/dig"
	"go.uber.org/zap"
	"os"
	"os/signal"
	"runtime/debug"
	"sync"
	"syscall"
)

type linkedList struct {
	node *shutdown
}

type shutdown struct {
	priority     int
	name         string
	next         *shutdown
	shutdownFunc func()
}

type App struct {
	ctx         context.Context
	shutdownRWM sync.RWMutex
	shutdown    *linkedList
	logger      *zap.Logger
	container   *dig.Container
	sig         chan os.Signal
}

func NewApp(ctx context.Context, container *dig.Container, logger *zap.Logger) *App {
	return &App{
		shutdown:  &linkedList{},
		logger:    logger,
		ctx:       ctx,
		container: container,
		sig:       make(chan os.Signal, 1),
	}
}

// RegisterShutdown registers a shutdown function with a priority.
// priority 0 is the highest priority.
func (app *App) RegisterShutdown(name string, fn func(), priority int) {
	app.shutdownRWM.Lock()
	defer app.shutdownRWM.Unlock()
	newShutdown := &shutdown{
		name:         name,
		priority:     priority,
		shutdownFunc: fn,
	}
	if app.shutdown.node == nil || app.shutdown.node.priority > priority {
		newShutdown.next = app.shutdown.node
		app.shutdown.node = newShutdown
		return
	}
	current := app.shutdown.node
	for current.next != nil && current.next.priority <= priority {
		current = current.next
	}
	newShutdown.next = current.next
	current.next = newShutdown
}

// ShutdownByName shuts down a registered shutdown function by name.
func (app *App) ShutdownByName(name string) {
	app.shutdownRWM.Lock()
	defer app.shutdownRWM.Unlock()
	if app.shutdown.node == nil {
		return
	}
	if app.shutdown.node.name == name {
		app.shutdown.node.shutdownFunc()
		app.shutdown.node = app.shutdown.node.next
		return
	}
	current := app.shutdown.node
	for current.next != nil {
		if current.next.name == name {
			current.next.shutdownFunc()
			current.next = current.next.next
			return
		}
		current = current.next
	}
}

func (app *App) shutdownAllAndDeleteAllCanceled() {
	app.shutdownRWM.Lock()
	defer app.shutdownRWM.Unlock()
	for app.shutdown.node != nil {
		app.shutdown.node.shutdownFunc()
		app.logger.Info(fmt.Sprintf("shutdown func %s with priority %v", app.shutdown.node.name, app.shutdown.node.priority))
		app.shutdown.node = app.shutdown.node.next
	}
}

func (app *App) Stop() {
	app.logger.Info("Stop()")
	app.shutdownAllAndDeleteAllCanceled()
}

func (app *App) Start(cancel context.CancelFunc) {
	signal.Notify(app.sig, syscall.SIGTERM, syscall.SIGINT)

	go func() {
		defer signal.Stop(app.sig)
		<-app.sig
		app.logger.Info("Signal received. Shutting down server...")
		cancel()
	}()
}

func (app *App) RegisterRecovers() func() {
	return func() {
		if r := recover(); r != nil {
			app.logger.Error("Паника произошла в основном приложении",
				zap.Any("panic", r),
				zap.String("stack", string(debug.Stack())),
			)
			app.sig <- syscall.SIGTERM
		}
	}
}

func (app *App) Container() *dig.Container {
	return app.container
}

func (app *App) Run() {
	<-app.ctx.Done()
}
