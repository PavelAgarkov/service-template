package server

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"github.com/gorilla/mux"
	"github.com/rs/xid"
	"go.uber.org/zap"
	"net"
	"net/http"
	"runtime/debug"
	"service-template/pkg"
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

// Override метода WriteHeader для логирования статуса
func (lrw *loggingResponseWriter) WriteHeader(code int) {
	lrw.statusCode = code
	lrw.ResponseWriter.WriteHeader(code)
}

// Override метода Write (опционально)
func (lrw *loggingResponseWriter) Write(data []byte) (int, error) {
	return lrw.ResponseWriter.Write(data)
}

// Реализация интерфейса http.Hijacker
func (lrw *loggingResponseWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	hj, ok := lrw.ResponseWriter.(http.Hijacker)
	if !ok {
		return nil, nil, errors.New("the ResponseWriter does not implement http.Hijacker")
	}
	return hj.Hijack()
}

// Реализация интерфейса http.Flusher
func (lrw *loggingResponseWriter) Flush() {
	if flusher, ok := lrw.ResponseWriter.(http.Flusher); ok {
		flusher.Flush()
	}
}

func newLoggingResponseWriter(w http.ResponseWriter) *loggingResponseWriter {
	return &loggingResponseWriter{w, http.StatusOK}
}

func LoggerContextMiddleware(logger *zap.Logger) mux.MiddlewareFunc {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Добавляем логгер в контекст запроса
			ctx := r.Context()
			ctx = pkg.LoggerWithCtx(ctx, logger) // Предполагается, что эта функция добавляет логгер в контекст
			r = r.WithContext(ctx)

			// Создаём новый ResponseWriter для логирования ответа (опционально)
			lrw := newLoggingResponseWriter(w)

			// Передаём обработку следующему обработчику в цепочке
			next.ServeHTTP(lrw, r)
		})
	}
}

func LoggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		correlationID := xid.New().String()

		ctx := context.WithValue(
			r.Context(),
			correlationIDCtxKey,
			correlationID,
		)

		r = r.WithContext(ctx)
		logger := pkg.LoggerFromCtx(r.Context())

		logger = logger.With(zap.String(string(correlationIDCtxKey), correlationID))
		w.Header().Add("X-Correlation-ID", correlationID)

		lrw := newLoggingResponseWriter(w)
		r = r.WithContext(pkg.LoggerWithCtx(ctx, logger))

		defer func(start time.Time) {
			logger.Info(
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
		rr := r.WithContext(r.Context())
		logger := pkg.LoggerFromCtx(rr.Context())
		defer func() {
			if r := recover(); r != nil {
				logger.Error("Паника произошла в http запросе приложении",
					zap.Any("panic", r),
					zap.String("stack", string(debug.Stack())),
				)
				w.WriteHeader(http.StatusInternalServerError)
			}
		}()
		next.ServeHTTP(w, r)
	})
}

type HTTPServer struct {
	port   string
	Router *mux.Router
	logger *zap.Logger
}

func newSimpleHTTPServer(logger *zap.Logger, port string) *HTTPServer {
	return &HTTPServer{
		Router: mux.NewRouter(),
		port:   port,
		logger: logger,
	}
}

// RunHTTPServer runs a simple HTTP server on the given port.
func (simple *HTTPServer) RunHTTPServer(balancer http.Handler, mwf ...mux.MiddlewareFunc) func() {
	simple.Router.Use(mwf...)

	var server *http.Server
	if balancer != nil {
		server = &http.Server{
			Addr:    simple.port,
			Handler: balancer,
		}
	} else {
		server = &http.Server{
			Addr:    simple.port,
			Handler: simple.Router,
		}
	}

	go func() {
		simple.logger.Info(fmt.Sprintf("Server is running on %s", simple.port))
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			panic(fmt.Sprintf("Server stopped by error: %s", err))
		}
		simple.logger.Info("Server has stopped")
	}()

	return simple.shutdown(server)
}

func (simple *HTTPServer) ToConfigureHandlers(configure func(simple *HTTPServer)) {
	configure(simple)
}

// Shutdown gracefully shuts down the server without interrupting any active connections.
func (simple *HTTPServer) shutdown(server *http.Server) func() {
	return func() {
		//l := pkg.GetLogger()
		simple.logger.Info("Shutting down the server...")
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		// использовать server.Shutdown(ctx) вместо server.Close() для корректного завершения запросов
		// и предотвращения утечек ресурсов
		// последние запросы за 5 секунд будут обработаны
		// после этого сервер будет остановлен
		// если необходимо остановить сервер сразу, то использовать server.Close()
		if err := server.Shutdown(ctx); err != nil {
			simple.logger.Error(fmt.Sprintf("Server shutdown failed: %s", err))
		}
		simple.logger.Info(fmt.Sprintf("Server has done: %s", simple.port))
	}
}

func (simple *HTTPServer) RunHTTPSServer(balancer http.Handler, certFile, keyFile string, mwf ...mux.MiddlewareFunc) func() {
	simple.Router.Use(mwf...)

	var server *http.Server
	if balancer != nil {
		server = &http.Server{
			Addr:    simple.port,
			Handler: balancer,
		}
	} else {
		server = &http.Server{
			Addr:    simple.port,
			Handler: simple.Router,
		}
	}

	go func() {
		simple.logger.Info(fmt.Sprintf("Server is running on %s with TLS", simple.port))
		if err := server.ListenAndServeTLS(certFile, keyFile); err != nil && !errors.Is(err, http.ErrServerClosed) {
			panic(fmt.Sprintf("Server stopped by error: %s", err))
		}
		simple.logger.Info("Server has stopped")
	}()

	return simple.shutdown(server)
}

func CreateHttpsServer(logger *zap.Logger, loadBalancer http.Handler, fn func(simple *HTTPServer), port, certFile, keyFile string, mwf ...mux.MiddlewareFunc) func() {
	serverHttp := newSimpleHTTPServer(logger, port)
	var simpleHttpServerShutdownFunction func()
	if loadBalancer != nil {
		simpleHttpServerShutdownFunction = serverHttp.RunHTTPSServer(loadBalancer, certFile, keyFile, mwf...)
	} else {
		serverHttp.ToConfigureHandlers(fn)
		simpleHttpServerShutdownFunction = serverHttp.RunHTTPSServer(nil, certFile, keyFile, mwf...)
	}
	return simpleHttpServerShutdownFunction
}

func CreateHttpServer(logger *zap.Logger, loadBalancer http.Handler, fn func(simple *HTTPServer), port string, mwf ...mux.MiddlewareFunc) func() {
	serverHttp := newSimpleHTTPServer(logger, port)
	var simpleHttpServerShutdownFunction func()
	if loadBalancer != nil {
		simpleHttpServerShutdownFunction = serverHttp.RunHTTPServer(loadBalancer, mwf...)
	} else {
		serverHttp.ToConfigureHandlers(fn)
		simpleHttpServerShutdownFunction = serverHttp.RunHTTPServer(nil, mwf...)
	}
	return simpleHttpServerShutdownFunction
}
