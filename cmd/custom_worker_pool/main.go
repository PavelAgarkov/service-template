package main

import (
	"context"
	"fmt"
	"service-template/application"
	"service-template/pkg"
	"time"
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

	pool := pkg.NewPoolStore[context.Context, int, string, string](pkg.NewMemStore(), 4)
	pool.Start(father)
	app.RegisterShutdown("pool", func() {
		pool.Shutdown()
	}, 1)

	keys := []string{"a", "b", "c", "d", "e", "f", "g"}

	time.Sleep(100 * time.Millisecond)
	for i, key := range keys {
		pool.Apply(father, i%3, key, fmt.Sprintf("%s-%d", key, i))
	}
	logger.Info("first pack of messages sent")

	for {
		active := pool.Stop()
		if active == 0 {
			fmt.Println("pool is empty")
			break
		}
		time.Sleep(100 * time.Millisecond)
		fmt.Println("waiting for pool to be empty")
	}
	pool.Reconnect()

	time.Sleep(100 * time.Millisecond)
	for i, key := range keys {
		pool.Apply(father, i%3, key, fmt.Sprintf("%s-%d", key, i))
	}
	logger.Info("next pack of messages sent")

	for {
		active := pool.Stop()
		if active == 0 {
			fmt.Println("pool is empty")
			break
		}
		time.Sleep(100 * time.Millisecond)
		fmt.Println("waiting for pool to be empty")
	}

	pool.Reconnect()
	fmt.Println("end_inWork:", pool.GetInWork())
	pool.Apply(father, 0, "a", "new-a")

	app.Run()
}
