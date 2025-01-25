package http_handler

import (
	"github.com/prometheus/client_golang/prometheus/promhttp"
	httpSwagger "github.com/swaggo/http-swagger"
	"log"
	"net/http"
	"service-template/pkg"
	"service-template/server"
)

type Handlers struct {
	postgres   *pkg.PostgresRepository
	etcd       *pkg.EtcdClientService
	serializer *pkg.Serializer
	prometheus *pkg.Prometheus
}

func NewHandlers(p *pkg.PostgresRepository, e *pkg.EtcdClientService, s *pkg.Serializer, prometheus *pkg.Prometheus) *Handlers {
	return &Handlers{
		postgres:   p,
		etcd:       e,
		serializer: s,
		prometheus: prometheus,
	}
}

func (h *Handlers) HandlerList() func(simple *server.HTTPServer) {
	return func(simple *server.HTTPServer) {
		// http://localhost:3000/swagger/index.html
		simple.Router.PathPrefix("/swagger/").Handler(httpSwagger.WrapHandler)

		simple.Router.Handle("/health", http.HandlerFunc(
			func(writer http.ResponseWriter, request *http.Request) {
				writer.WriteHeader(http.StatusOK)
				log.Println("health check")
				return
			})).Methods("GET")

		simple.Router.Handle("/empty", http.HandlerFunc(h.EmptyHandler)).Methods("POST")
		simple.Router.Handle("/metrics", promhttp.Handler())
		//router.HandleFunc("/user/{id}/posts/{postId}", GetPostHandler).Methods("GET")
		//router.HandleFunc("/user/{id:[0-9]+}/posts/{postId:[0-9]+}", GetPostHandler).Methods("POST")
	}
}
