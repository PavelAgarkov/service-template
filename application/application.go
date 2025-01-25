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

type LinkedList struct {
	Node *Shutdown
}

type Shutdown struct {
	Priority     int
	Name         string
	Next         *Shutdown
	shutdownFunc func()
}

type App struct {
	ctx         context.Context
	shutdownRWM sync.RWMutex
	shutdown    *LinkedList
	logger      *zap.Logger
	container   *dig.Container
	sig         chan os.Signal
}

func NewApp(ctx context.Context, container *dig.Container, logger *zap.Logger) *App {
	return &App{
		shutdown:  &LinkedList{},
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
	newShutdown := &Shutdown{
		Name:         name,
		Priority:     priority,
		shutdownFunc: fn,
	}
	if app.shutdown.Node == nil || app.shutdown.Node.Priority > priority {
		newShutdown.Next = app.shutdown.Node
		app.shutdown.Node = newShutdown
		return
	}
	current := app.shutdown.Node
	for current.Next != nil && current.Next.Priority <= priority {
		current = current.Next
	}
	newShutdown.Next = current.Next
	current.Next = newShutdown
}

// ShutdownByName shuts down a registered shutdown function by name.
func (app *App) ShutdownByName(name string) {
	app.shutdownRWM.Lock()
	defer app.shutdownRWM.Unlock()
	if app.shutdown.Node == nil {
		return
	}
	if app.shutdown.Node.Name == name {
		app.shutdown.Node.shutdownFunc()
		app.shutdown.Node = app.shutdown.Node.Next
		return
	}
	current := app.shutdown.Node
	for current.Next != nil {
		if current.Next.Name == name {
			current.Next.shutdownFunc()
			current.Next = current.Next.Next
			return
		}
		current = current.Next
	}
}

func (app *App) GetAllRegisteredShutdown() *LinkedList {
	return app.shutdown
}

func (app *App) shutdownAllAndDeleteAllCanceled() {
	app.shutdownRWM.Lock()
	defer app.shutdownRWM.Unlock()
	for app.shutdown.Node != nil {
		app.shutdown.Node.shutdownFunc()
		app.logger.Info(fmt.Sprintf("shutdown func %s with priority %v", app.shutdown.Node.Name, app.shutdown.Node.Priority))
		app.shutdown.Node = app.shutdown.Node.Next
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
