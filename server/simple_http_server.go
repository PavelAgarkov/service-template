package server

import (
	"context"
	"errors"
	"fmt"
	"github.com/gorilla/mux"
	"github.com/rs/xid"
	"go.uber.org/zap"
	"log"
	"net/http"
	"service-template/internal/logger"
	"time"
)

type contextKey string

const (
	correlationIDCtxKey contextKey = "correlation_id"
)

type loggingResponseWriter struct {
	http.ResponseWriter
	statusCode int
}

func newLoggingResponseWriter(w http.ResponseWriter) *loggingResponseWriter {
	return &loggingResponseWriter{w, http.StatusOK}
}

func LoggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		l := logger.Get()

		correlationID := xid.New().String()

		ctx := context.WithValue(
			r.Context(),
			correlationIDCtxKey,
			correlationID,
		)

		r = r.WithContext(ctx)

		l = l.With(zap.String(string(correlationIDCtxKey), correlationID))
		w.Header().Add("X-Correlation-ID", correlationID)

		lrw := newLoggingResponseWriter(w)
		r = r.WithContext(logger.WithCtx(ctx, l))

		defer func(start time.Time) {
			l.Info(
				fmt.Sprintf(
					"%s request to %s completed",
					r.Method,
					r.RequestURI,
				),
				zap.String("method", r.Method),
				zap.String("url", r.RequestURI),
				zap.String("user_agent", r.UserAgent()),
				zap.Int("status_code", lrw.statusCode),
				zap.Duration("elapsed_ms", time.Since(start)),
			)
		}(time.Now())
		next.ServeHTTP(lrw, r)
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
		l := logger.Get()
		defer func() {
			if r := recover(); r != nil {
				l.Error(fmt.Sprintf("Recovered from panic: %v on server %v", r, simple.port))
			}
		}()
		l.Info(fmt.Sprintf("Server is running on %s", simple.port))
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			panic(fmt.Sprintf("Server stopped by error: %s", err))
		}
		l.Info("Server has stopped")
	}()

	return simple.Shutdown(server)
}

func (simple *SimpleHTTPServer) ToConfigureHandlers(configure func(simple *SimpleHTTPServer)) {
	configure(simple)
}

// Shutdown gracefully shuts down the server without interrupting any active connections.
func (simple *SimpleHTTPServer) Shutdown(server *http.Server) func() {
	return func() {
		l := logger.Get()
		l.Info("Shutting down the server...")
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		// использовать server.Shutdown(ctx) вместо server.Close() для корректного завершения запросов
		// и предотвращения утечек ресурсов
		// последние запросы за 5 секунд будут обработаны
		// после этого сервер будет остановлен
		// если необходимо остановить сервер сразу, то использовать server.Close()
		if err := server.Shutdown(ctx); err != nil {
			l.Error(fmt.Sprintf("Server shutdown failed: %s", err))
		}
		l.Info(fmt.Sprintf("Server has done: %s", simple.port))
	}
}

func CreateHttpServer(fn func(simple *SimpleHTTPServer), port string, mwf ...mux.MiddlewareFunc) func() {
	serverHttp := newSimpleHTTPServer(port)
	serverHttp.ToConfigureHandlers(fn)
	simpleHttpServerShutdownFunction := serverHttp.RunSimpleHTTPServer(mwf...)
	return simpleHttpServerShutdownFunction
}
