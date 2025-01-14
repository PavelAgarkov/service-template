package main

import (
	"context"
	"fmt"
	"github.com/oklog/run"
	"go.etcd.io/etcd/client/v3/concurrency"
	"go.uber.org/zap"
	"log"
	"math/rand"
	"os"
	"os/signal"
	"service-template/application"
	"service-template/internal"
	"service-template/pkg"
	"service-template/server"
	"strconv"
	"syscall"
)

func main() {
	logger := pkg.NewLogger("simple_http_server", "logs/app.log")
	father, cancel := context.WithCancel(context.Background())
	father = pkg.LoggerWithCtx(father, logger)
	defer cancel()

	logger.Info("config initializing")
	//cfg := config.GetConfig()

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

	app.RegisterShutdown("logger", func() {
		err := logger.Sync()
		if err != nil {
			log.Println(fmt.Sprintf("failed to sync logger: %v", err))
		}
	}, 101)

	port := ":" + os.Getenv("HTTP_PORT")

	serviceID := strconv.Itoa(rand.Intn(1000000))
	key := fmt.Sprintf("/loadbalancer/services/%s/%s", "my-service", serviceID)
	etcdClientService, etcdCloser := pkg.NewEtcdClientService(
		father,
		"http://localhost:2379",
		"admin",
		"adminpassword",
		port,
		"http",
		key,
		serviceID,
		logger,
	)

	app.RegisterShutdown(pkg.EtcdClient, func() {
		etcdCloser()
		logger.Info("etcd client closed")
	}, 99)

	err := etcdClientService.Register(father, logger)
	if err != nil {
		logger.Error(fmt.Sprintf("failed to register service: %v", err))
	}

	session, err := concurrency.NewSession(etcdClientService.Client, concurrency.WithTTL(5))
	if err != nil {
		logger.Fatal("Не удалось создать сессию etcd:", zap.Error(err))
	}
	app.RegisterShutdown("etcd_session", func() {
		session.Close()
		logger.Info("Сессия etcd закрыта")
	}, 98)

	election := concurrency.NewElection(session, "/my-service/"+"leader-election")

	stopElection := pkg.DoElection(logger, father, session, election)
	app.RegisterShutdown("election", stopElection, 97)

	_ = internal.NewContainer(
		logger,
		&internal.ServiceInit{Name: pkg.SerializerService, Service: pkg.NewSerializer()},
		&internal.ServiceInit{Name: pkg.EtcdClient, Service: etcdClientService},
	)

	loadBalancer := pkg.NewLoadBalancer(logger, etcdClientService)

	var g run.Group
	loadBalancer.Run(father, "/services/my-service/", &g)
	go func() {
		// если запустить не в горутине - будет блокировка
		// т.к. run.Group().Run() блокирующий метод
		// поэтому нужно для каждой группы контроля запускать с помощью отдельного инстанса run.Group
		e := g.Run()
		if e != nil {
			logger.Error("Ошибка при запуске controller", zap.Error(e))
		}
	}()

	//app.RegisterShutdown("load_balancer", closeLb, 1)

	simpleHttpServerShutdownFunctionHttp := server.CreateHttpServer(
		logger,
		loadBalancer,
		nil,
		port,
		server.LoggerContextMiddleware(logger),
		server.RecoverMiddleware,
		server.LoggingMiddleware,
	)
	app.RegisterShutdown("simple_https_server", simpleHttpServerShutdownFunctionHttp, 1)

	<-father.Done()
}
