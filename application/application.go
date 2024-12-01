package application

import (
	"context"
	"log"
	"sync"
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
	shutdownRWM sync.RWMutex
	shutdown    *LinkedList
	//ctx         context.Context
}

func NewApp(ctx context.Context) *App {
	return &App{
		//ctx:      ctx,
		shutdown: &LinkedList{},
	}
}

// RegisterShutdown registers a shutdown function with a priority.
// priority 0 is the highest priority.
func (a *App) RegisterShutdown(name string, fn func(), priority int) {
	a.shutdownRWM.Lock()
	defer a.shutdownRWM.Unlock()
	newShutdown := &Shutdown{
		Name:         name,
		Priority:     priority,
		shutdownFunc: fn,
	}
	if a.shutdown.Node == nil || a.shutdown.Node.Priority > priority {
		newShutdown.Next = a.shutdown.Node
		a.shutdown.Node = newShutdown
		return
	}
	current := a.shutdown.Node
	for current.Next != nil && current.Next.Priority <= priority {
		current = current.Next
	}
	newShutdown.Next = current.Next
	current.Next = newShutdown
}

// ShutdownByName shuts down a registered shutdown function by name.
func (a *App) ShutdownByName(name string) {
	a.shutdownRWM.Lock()
	defer a.shutdownRWM.Unlock()
	if a.shutdown.Node == nil {
		return
	}
	if a.shutdown.Node.Name == name {
		a.shutdown.Node.shutdownFunc()
		a.shutdown.Node = a.shutdown.Node.Next
		return
	}
	current := a.shutdown.Node
	for current.Next != nil {
		if current.Next.Name == name {
			current.Next.shutdownFunc()
			current.Next = current.Next.Next
			return
		}
		current = current.Next
	}
}

func (a *App) GetAllRegisteredShutdown() *LinkedList {
	return a.shutdown
}

// shutdownAll shuts down all registered shutdown functions.
func (a *App) shutdownAll() {
	a.shutdownRWM.Lock()
	defer a.shutdownRWM.Unlock()
	current := a.shutdown.Node
	for current != nil {
		current.shutdownFunc()
		log.Printf("shutdown func %s with priority %v", current.Name, current.Priority)
		current = current.Next
	}
}

// Stop stops the application.
func (a *App) Stop() {
	//var wg sync.WaitGroup
	//wg.Add(1)
	//go func() {
	//	defer wg.Done()
	log.Printf("Stop()")
	//<-a.ctx.Done()
	log.Printf("father.Done()")
	a.shutdownAll()
	// добавить проверку всех завершенных нод. Удалять успешно завершенные из списка
	//wg.Done()
	//}()
	//wg.Wait()
}
