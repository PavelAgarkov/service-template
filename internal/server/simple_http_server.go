package server

import (
	"context"
	"errors"
	"flick/internal/handler"
	"github.com/gorilla/mux"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
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

type SimpleHTTPServer struct {
	port   string
	router *mux.Router
}

func (simple *SimpleHTTPServer) RunSimpleHTTPServer(father context.Context, cancel context.CancelFunc, port string, mwf ...mux.MiddlewareFunc) error {
	simple.router = mux.NewRouter()
	simple.router.Use(mwf...)
	simple.ToConfigureHandlers()
	simple.port = port

	server := &http.Server{
		Addr:    simple.port,
		Handler: simple.router,
	}

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGTERM, syscall.SIGINT)
	defer signal.Stop(sig)

	go func() {
		<-sig
		log.Println("Signal received. Shutting down server...")
		cancel()
	}()

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		<-father.Done()

		log.Println("Shutting down the server...")
		if err := server.Shutdown(context.Background()); err != nil {
			log.Printf("Server shutdown failed: %s", err)
		}
	}()

	log.Printf("Server is running on %s", simple.port)
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatalf("Server stopped by error: %s", err)
		return err
	}

	wg.Wait()
	log.Println("Server exited gracefully")

	return nil
}

func (simple *SimpleHTTPServer) ToConfigureHandlers() {
	simple.router.Handle("/", http.HandlerFunc(handler.EmptyHandler)).Methods("POST")
}
