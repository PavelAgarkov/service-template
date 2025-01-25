package container

import (
	"context"
	"github.com/elastic/go-elasticsearch/v8"
	"github.com/redis/go-redis/v9"
	gorabbitmq "github.com/wagslane/go-rabbitmq"
	"go.uber.org/dig"
	"go.uber.org/zap"
	"log"
	"service-template/config"
	"service-template/internal/grpc_handler"
	"service-template/internal/http_handler"
	"service-template/internal/repository"
	"service-template/internal/service"
	"service-template/pkg"
	"time"
)

func BuildContainerForHttpServer(father context.Context, logger *zap.Logger, cfg *config.Config, port string) *dig.Container {
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

	err = container.Provide(func(prom *pkg.Prometheus, postgres *pkg.PostgresRepository, etcd *pkg.EtcdClientService, serializer *pkg.Serializer) *http_handler.Handlers {
		return http_handler.NewHandlers(postgres, etcd, serializer, prom)
	})
	if err != nil {
		log.Fatalf("failed to provide prometheus %v", err)
	}

	return container
}

func BuildContainerForGrpcServer(logger *zap.Logger, cfg *config.Config, connectionRabbitString string) *dig.Container {
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

	err = container.Provide(func() *gorabbitmq.Conn {
		r, _ := gorabbitmq.NewClusterConn(
			gorabbitmq.NewStaticResolver(
				[]string{
					connectionRabbitString,
				},
				false,
			),
			gorabbitmq.WithConnectionOptionsLogging,
		)
		return r
	})
	if err != nil {
		log.Fatalf("failed to provide rabbitmq %v", err)
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

	err = container.Provide(func(postgres *pkg.PostgresRepository, logger *zap.Logger, redisClient *pkg.RedisClient) *pkg.Migrations {
		return pkg.NewMigrations(postgres.GetDB().DB, logger, redisClient.Client)
	})
	if err != nil {
		log.Fatalf("failed to provide migrations %v", err)
	}

	err = container.Provide(func(postgres *pkg.PostgresRepository, srv *service.Srv) *grpc_handler.MyService {
		return grpc_handler.NewMyService(postgres, srv)
	})
	if err != nil {
		log.Fatalf("failed to provide grpc_handler.MyService %v", err)
	}

	err = container.Provide(func(postgres *pkg.PostgresRepository, srv *service.Srv) *grpc_handler.MyService2 {
		return grpc_handler.NewMyService2(postgres, srv)
	})
	if err != nil {
		log.Fatalf("failed to provide grpc_handler.MyService2 %v", err)
	}

	return container
}
