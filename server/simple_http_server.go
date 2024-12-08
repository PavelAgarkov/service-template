package server

import (
	"context"
	"errors"
	"fmt"
	"github.com/gorilla/mux"
	"log"
	"net/http"
	"time"
)

func LoggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		log.Printf("Incoming request: %s %s", r.Method, r.URL.Path)
		next.ServeHTTP(w, r)
		log.Printf("Request processed in %s", time.Since(start))
	})
}

func RecoverMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if r := recover(); r != nil {
				log.Printf("Recovered from panic: %v", r)
				w.WriteHeader(http.StatusInternalServerError)
			}
		}()
		next.ServeHTTP(w, r)
	})
}

type SimpleHTTPServer struct {
	port   string
	Router *mux.Router
}

func newSimpleHTTPServer(port string) *SimpleHTTPServer {
	return &SimpleHTTPServer{
		Router: mux.NewRouter(),
		port:   port,
	}
}

// RunSimpleHTTPServer runs a simple HTTP server on the given port.
func (simple *SimpleHTTPServer) RunSimpleHTTPServer(mwf ...mux.MiddlewareFunc) func() {
	simple.Router.Use(mwf...)

	server := &http.Server{
		Addr:    simple.port,
		Handler: simple.Router,
	}

	go func() {
		defer func() {
			if r := recover(); r != nil {
				log.Printf("Recovered from panic: %v on server %v", r, simple.port)
			}
		}()
		log.Printf("Server is running on %s", simple.port)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			panic(fmt.Sprintf("Server stopped by error: %s", err))
		}
		log.Printf("Server has stopped")
	}()

	return simple.Shutdown(server)
}

func (simple *SimpleHTTPServer) ToConfigureHandlers(configure func(simple *SimpleHTTPServer)) {
	configure(simple)
}

// Shutdown gracefully shuts down the server without interrupting any active connections.
func (simple *SimpleHTTPServer) Shutdown(server *http.Server) func() {
	return func() {
		log.Println("Shutting down the server...")
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		// использовать server.Shutdown(ctx) вместо server.Close() для корректного завершения запросов
		// и предотвращения утечек ресурсов
		// последние запросы за 5 секунд будут обработаны
		// после этого сервер будет остановлен
		// если необходимо остановить сервер сразу, то использовать server.Close()
		if err := server.Shutdown(ctx); err != nil {
			log.Printf("Server shutdown failed: %s", err)
		}
		log.Printf("Server has done: %s", simple.port)
	}
}

func CreateHttpServer(fn func(simple *SimpleHTTPServer), port string, mwf ...mux.MiddlewareFunc) func() {
	serverHttp := newSimpleHTTPServer(port)
	serverHttp.ToConfigureHandlers(fn)
	simpleHttpServerShutdownFunction := serverHttp.RunSimpleHTTPServer(mwf...)
	return simpleHttpServerShutdownFunction
}
