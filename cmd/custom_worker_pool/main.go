package main

import (
	"context"
	"fmt"
	"go.uber.org/zap"
	"service-template/application"
	"service-template/pkg"
)

func main() {
	logger := pkg.NewLogger(pkg.LoggerConfig{ServiceName: "custom_worker_pool", LogPath: "logs/app.log"})

	father, cancel := context.WithCancel(context.Background())
	father = pkg.LoggerWithCtx(father, logger)
	defer cancel()

	app := application.NewApp(father, nil, logger)
	app.Start(cancel)
	defer app.Stop()
	defer app.RegisterRecovers()()

	pool := pkg.NewPoolStore[context.Context, int, string, string](father, pkg.NewMemStore(), 12)
	pool.Run(father)
	app.RegisterShutdown("pool", func() {
		pool.Shutdown()
	}, 1)

	keys := []string{"a", "b", "c", "d", "e", "f", "g"}

	for i, key := range keys {
		err := pool.Apply(father, i%3, key, fmt.Sprintf("%s-%d", key, i))
		if err != nil {
			logger.Error(err.Error())
			//logger.Error("stop applying" + zap.Error(err).String)
			break
		}
	}
	logger.Info("first pack of messages sent")

	pool.Stop()
	pool.Reconnect()

	for i, key := range keys {
		err := pool.Apply(father, i%3, key, fmt.Sprintf("%s-%d", key, i))
		if err != nil {
			logger.Error("stop applying" + zap.Error(err).String)
			break
		}
	}
	logger.Info("next pack of messages sent")

	pool.Stop()
	pool.Reconnect()

	fmt.Println("end_inWork:", pool.GetInWork())

	logger.Info("last messages sent")
	err := pool.Apply(father, 0, "a", "new-a")
	if err != nil {
		logger.Error("stop applying" + zap.Error(err).String)
	}

	app.Run()
}
