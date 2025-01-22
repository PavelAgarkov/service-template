package main

import (
	"context"
	"fmt"
	"github.com/oklog/run"
	"go.etcd.io/etcd/client/v3/concurrency"
	"go.uber.org/zap"
	"log"
	"os"
	"os/signal"
	"service-template/application"
	"service-template/pkg"
	"service-template/server"
	"syscall"
)

func main() {
	logger := pkg.NewLogger(pkg.LoggerConfig{ServiceName: "simple_http_server", LogPath: "logs/app.log"})
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
	defer app.RegisterRecovers(logger, sig)()

	port := ":" + os.Getenv("HTTP_PORT")

	serviceID := pkg.NewServiceId()
	serviceKey := pkg.NewServiceKey(serviceID, "my-service")
	etcdClientService := pkg.NewEtcdClientService(
		father,
		"http://localhost:2379",
		"admin",
		"adminpassword",
		port,
		"http",
		serviceKey,
		serviceID,
		logger,
	)

	app.RegisterShutdown(pkg.EtcdClient, func() {
		etcdClientService.ShutdownFunc()
		logger.Info("etcd client closed")
	}, 99)

	err := etcdClientService.Register(father, logger)
	if err != nil {
		logger.Error(fmt.Sprintf("failed to register service: %v", err))
	}

	_, err = etcdClientService.CreateSession(concurrency.WithTTL(5))
	if err != nil {
		logger.Error(fmt.Sprintf("failed to create session: %v", err))
	}
	app.RegisterShutdown("etcd_session", func() {
		etcdClientService.StopSession()
	}, 98)

	election := concurrency.NewElection(etcdClientService.GetSession(), "/my-service/"+"leader-election")
	stopElection := pkg.DoElection(logger, father, etcdClientService.GetSession(), election)
	app.RegisterShutdown("election", stopElection, 97)

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
