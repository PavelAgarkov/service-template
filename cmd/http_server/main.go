package main

import (
	"context"
	"fmt"
	"github.com/elastic/go-elasticsearch/v8"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/redis/go-redis/v9"
	httpSwagger "github.com/swaggo/http-swagger"
	"go.uber.org/zap"
	"log"
	"net/http"
	"os"
	"os/signal"
	"service-template/application"
	_ "service-template/cmd/http_server/docs"
	"service-template/config"
	"service-template/internal/http_handler"
	"service-template/internal/repository"
	"service-template/internal/service"
	"service-template/pkg"
	"service-template/server"
	"syscall"
	"time"

	"go.uber.org/dig"
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
	logger := pkg.NewLogger(pkg.LoggerConfig{ServiceName: "simple_http_server", LogPath: "logs/app.log"})

	father, cancel := context.WithCancel(context.Background())
	father = pkg.LoggerWithCtx(father, logger)
	defer cancel()

	logger.Info("config initializing")
	cfg := config.GetConfig()
	port := ":" + os.Getenv("HTTP_PORT")

	connectionRabbitString := "amqp://user:password@localhost:5672/"
	container := BuildContainer(father, logger, cfg, port)

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGTERM, syscall.SIGINT)
	defer signal.Stop(sig)

	go func() {
		<-sig
		logger.Info("Signal received. Shutting down server...")
		cancel()
	}()
	app := application.NewApp(logger)
	defer app.Stop()
	defer app.RegisterRecovers(logger, sig)()

	err := container.Invoke(func(
		postgres *pkg.PostgresRepository,
		etcdClientService *pkg.EtcdClientService,
		srvRepository *repository.SrvRepository,
		srvService *service.Srv,
		elk *pkg.ElasticFacade,
		redisClient *pkg.RedisClient,
		migrations *pkg.Migrations,
		prom *pkg.Prometheus,
	) {
		prometheus.MustRegister(prom.Http)

		app.RegisterShutdown("logger", func() {
			err := logger.Sync()
			if err != nil {
				logger.Error(fmt.Sprintf("failed to sync logger: %v", err))
			}
		}, 101)

		app.RegisterShutdown("postgres", postgres.ShutdownFunc, 100)
		migrations.Migrate(father, "./migrations", "goose_db_version")
		migrations.MigrateRabbitMq(father, "rabbit_migrations", []string{connectionRabbitString})

		_, err := etcdClientService.CreateSession()
		if err != nil {
			logger.Error(fmt.Sprintf("failed to create session: %v", err))
		}
		app.RegisterShutdown("etcd_session", func() {
			etcdClientService.StopSession()
		}, 98)
		app.RegisterShutdown(pkg.EtcdClient, func() {
			etcdClientService.ShutdownFunc()
			logger.Info("etcd client closed")
		}, 99)
		err = etcdClientService.Register(father, logger)
		if err != nil {
			logger.Error(fmt.Sprintf("failed to register service: %v", err))
		}

		handlers := http_handler.NewHandlers(container, prom)
		simpleHttpServerShutdownFunctionHttp := server.CreateHttpServer(
			logger,
			nil,
			handlerList(handlers),
			port,
			server.LoggerContextMiddleware(logger),
			server.RecoverMiddleware,
			server.LoggingMiddleware,
		)
		app.RegisterShutdown("simple_https_server", simpleHttpServerShutdownFunctionHttp, 1)

		//openssl req \
		//-x509 \
		//-nodes \
		//-newkey rsa:4096 \
		//-keyout server.key \
		//-out server.crt \
		//-days 365 \
		//-subj "/CN=localhost" \
		//-addext "subjectAltName=DNS:localhost"
		simpleHttpServerShutdownFunctionHttps := server.CreateHttpsServer(
			logger,
			nil,
			handlerList(handlers),
			":8433",        // Порт сервера
			"./server.crt", // Путь к сертификату
			"./server.key", // Путь к ключу
			server.LoggerContextMiddleware(logger),
			server.RecoverMiddleware,
			server.LoggingMiddleware,
		)
		app.RegisterShutdown("simple_https_server", simpleHttpServerShutdownFunctionHttps, 1)
	})

	if err != nil {
		logger.Error(fmt.Sprintf("failed to start DI: %v", err))
		return
	}

	//handlers := http_handler.NewHandlers(container)
	//simpleHttpServerShutdownFunctionHttp := server.CreateHttpServer(
	//	logger,
	//	nil,
	//	handlerList(handlers),
	//	port,
	//	server.LoggerContextMiddleware(logger),
	//	server.RecoverMiddleware,
	//	server.LoggingMiddleware,
	//)
	//app.RegisterShutdown("simple_https_server", simpleHttpServerShutdownFunctionHttp, 1)
	//
	////openssl req \
	////-x509 \
	////-nodes \
	////-newkey rsa:4096 \
	////-keyout server.key \
	////-out server.crt \
	////-days 365 \
	////-subj "/CN=localhost" \
	////-addext "subjectAltName=DNS:localhost"
	//simpleHttpServerShutdownFunctionHttps := server.CreateHttpsServer(
	//	logger,
	//	nil,
	//	handlerList(handlers),
	//	":8433",        // Порт сервера
	//	"./server.crt", // Путь к сертификату
	//	"./server.key", // Путь к ключу
	//	server.LoggerContextMiddleware(logger),
	//	server.RecoverMiddleware,
	//	server.LoggingMiddleware,
	//)
	//app.RegisterShutdown("simple_https_server", simpleHttpServerShutdownFunctionHttps, 1)
	<-father.Done()
}

func BuildContainer(father context.Context, logger *zap.Logger, cfg *config.Config, port string) *dig.Container {
	container := dig.New()
	err := container.Provide(func() *zap.Logger { return logger })
	if err != nil {
		log.Fatalf("failed to provide logger %v", err)
	}
	err = container.Provide(func() *config.Config { return cfg })
	if err != nil {
		log.Fatalf("failed to provide config %v", err)
	}
	err = container.Provide(func() *pkg.PostgresRepository {
		return pkg.NewPostgres(
			logger,
			cfg.DB.Host,
			cfg.DB.Port,
			cfg.DB.Username,
			cfg.DB.Password,
			cfg.DB.Database,
			"disable",
		)
	})
	if err != nil {
		log.Fatalf("failed to provide postgres %v", err)
	}

	serviceID := pkg.NewServiceId()
	//serviceKey := pkg.NewServiceKey(serviceID, "my-service")
	err = container.Provide(func() *pkg.EtcdClientService {
		return pkg.NewEtcdClientService(
			father,
			"http://localhost:2379",
			"admin",
			"adminpassword",
			port,
			"http",
			pkg.NewServiceKey(serviceID, "my-service"),
			serviceID,
			logger,
		)
	})
	if err != nil {
		log.Fatalf("failed to provide etcd %v", err)
	}

	err = container.Provide(func() *repository.SrvRepository {
		return repository.NewSrvRepository()
	})
	if err != nil {
		log.Fatalf("failed to provide repository %v", err)
	}

	err = container.Provide(func() *service.Srv {
		return service.NewSrv()
	})
	if err != nil {
		log.Fatalf("failed to provide service %v", err)
	}

	err = container.Provide(func() *pkg.Serializer {
		return pkg.NewSerializer()
	})
	if err != nil {
		log.Fatalf("failed to provide serializer %v", err)
	}

	err = container.Provide(func() *pkg.RedisClient {
		return pkg.NewRedisClient(&redis.Options{
			Addr:     "127.0.0.1:6379",
			Username: "myuser",
			Password: "mypassword",
			//такое использование баз данных возможно только без кластера
			// каждый сервис должен использовать свою базу данных DB
			// всего баз в сервере 16 DB
			// каждое подключение может использовать только одну базу данных DB
			DB:           1,
			DialTimeout:  5 * time.Second,
			ReadTimeout:  3 * time.Second,
			WriteTimeout: 3 * time.Second,

			// PoolSize, MinIdleConns можно настраивать при высоконагруженных сценариях.
			PoolSize:     10,
			MinIdleConns: 2,
		},
			logger,
		)
	})
	if err != nil {
		log.Fatalf("failed to provide redis %v", err)
	}

	err = container.Provide(func() *pkg.ElasticFacade {
		es, err := pkg.NewElasticFacade(
			elasticsearch.Config{
				Addresses: []string{"http://localhost:9200"},
				Username:  "elastic",
				Password:  "elasticpassword",
				// Если работаете по HTTPS/TLS — нужно добавить настройки для сертификатов (TLS), см. документацию.
			}, logger)
		if err != nil {
			logger.Fatal("Ошибка создания фасада Elasticsearch: %s", zap.Error(err))
		}
		return es
	})
	if err != nil {
		log.Fatalf("failed to provide elastic %v", err)
	}

	err = container.Provide(func(postgres *pkg.PostgresRepository, logger *zap.Logger, redisClient *pkg.RedisClient) *pkg.Migrations {
		return pkg.NewMigrations(postgres.GetDB().DB, logger, redisClient.Client)
	})
	if err != nil {
		log.Fatalf("failed to provide migrations %v", err)
	}

	err = container.Provide(func() *pkg.Prometheus {
		return pkg.NewPrometheus()
	})
	if err != nil {
		log.Fatalf("failed to provide prometheus %v", err)
	}

	return container
}

func handlerList(handlers *http_handler.Handlers) func(simple *server.HTTPServer) {
	return func(simple *server.HTTPServer) {
		// http://localhost:3000/swagger/index.html
		simple.Router.PathPrefix("/swagger/").Handler(httpSwagger.WrapHandler)

		simple.Router.Handle("/health", http.HandlerFunc(
			func(writer http.ResponseWriter, request *http.Request) {
				writer.WriteHeader(http.StatusOK)
				log.Println("health check")
				return
			})).Methods("GET")

		simple.Router.Handle("/empty", http.HandlerFunc(handlers.EmptyHandler)).Methods("POST")
		simple.Router.Handle("/metrics", promhttp.Handler())
		//router.HandleFunc("/user/{id}/posts/{postId}", GetPostHandler).Methods("GET")
		//router.HandleFunc("/user/{id:[0-9]+}/posts/{postId:[0-9]+}", GetPostHandler).Methods("POST")
	}
}
