package server

import (
	"context"
	"errors"
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

func NewSimpleHTTPServer(port string) *SimpleHTTPServer {
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
		log.Printf("Server is running on %s", simple.port)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("Server stopped by error: %s", err)
		}
		log.Printf("Server has stopped")
	}()

	return simple.Shutdown(server)
}

func (simple *SimpleHTTPServer) ToConfigureHandlers(configure func(simple *SimpleHTTPServer)) {
	configure(simple)
}

func (simple *SimpleHTTPServer) Shutdown(server *http.Server) func() {
	return func() {
		log.Println("Shutting down the server...")
		if err := server.Shutdown(context.Background()); err != nil {
			log.Printf("Server shutdown failed: %s", err)
		}
		log.Printf("Server has done: %s", simple.port)
	}
}
