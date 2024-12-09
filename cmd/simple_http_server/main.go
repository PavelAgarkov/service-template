package main

import (
	"context"
	httpSwagger "github.com/swaggo/http-swagger"
	"log"
	"net/http"
	"os"
	"os/signal"
	"service-template/application"
	_ "service-template/cmd/simple_http_server/docs"
	"service-template/internal"
	"service-template/internal/http_handler"
	"service-template/internal/repository"
	"service-template/internal/service"
	"service-template/pkg"
	"service-template/server"
	"syscall"
)

// @title Simple HTTP Server API
// @version 1.0
// @description Это пример HTTP-сервера с документацией Swagger.
// @contact.name Поддержка API
// @contact.url http://example.com/support
// @contact.email support@example.com
// @host localhost:3000
// @BasePath /
func main() {
	father, cancel := context.WithCancel(context.Background())
	defer cancel()

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGTERM, syscall.SIGINT)
	defer signal.Stop(sig)

	go func() {
		<-sig
		log.Println("Signal received. Shutting down server...")
		cancel()
	}()

	app := application.NewApp()
	defer func() {
		app.Stop()
		log.Printf("app is stopped")
	}()

	postgres, postgresShutdown := pkg.NewPostgres("0.0.0.0", "habrpguser", "habrdb", "pgpwd4habr", "disable")
	app.RegisterShutdown("postgres", postgresShutdown, 100)

	pkg.NewMigrations(postgres.GetDB().DB).Migrate("./migrations")

	container := internal.NewContainer(
		&internal.ServiceInit{Name: pkg.SerializerService, Service: pkg.NewSerializer()},
		&internal.ServiceInit{Name: pkg.PostgresService, Service: postgres},
	).
		Set(repository.SrvRepositoryService, repository.NewSrvRepository(), pkg.PostgresService).
		Set(service.ServiceSrv, service.NewSrv(), pkg.SerializerService, repository.SrvRepositoryService)

	handlers := http_handler.NewHandlers(container)

	simpleHttpServerShutdownFunction := server.CreateHttpServer(
		handlerList(handlers),
		":3000",
		server.RecoverMiddleware,
		server.LoggingMiddleware,
	)
	app.RegisterShutdown("simple_http_server", simpleHttpServerShutdownFunction, 1)

	<-father.Done()
}

func handlerList(handlers *http_handler.Handlers) func(simple *server.SimpleHTTPServer) {
	return func(simple *server.SimpleHTTPServer) {
		// http://localhost:3000/swagger/index.html
		simple.Router.PathPrefix("/swagger/").Handler(httpSwagger.WrapHandler)
		simple.Router.Handle("/empty", http.HandlerFunc(handlers.EmptyHandler)).Methods("POST")
		//router.HandleFunc("/user/{id}/posts/{postId}", GetPostHandler).Methods("GET")
		//router.HandleFunc("/user/{id:[0-9]+}/posts/{postId:[0-9]+}", GetPostHandler).Methods("POST")
	}
}
