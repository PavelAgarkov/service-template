package main

import (
	"context"
	"fmt"
	"math/rand"
	"runtime/debug"
	"service-template/application"
	"service-template/pkg"
	"time"
)

func main() {
	logger := pkg.NewLogger(pkg.LoggerConfig{ServiceName: "custom_gc", LogPath: "logs/app.log"})

	father, cancel := context.WithCancel(context.Background())
	father = pkg.LoggerWithCtx(father, logger)
	defer cancel()

	app := application.NewApp(father, nil, logger)
	app.Start(cancel)
	defer app.Stop()
	defer app.RegisterRecovers()()
	debug.SetGCPercent(-1)
	app.RegisterShutdown("logger", func() {
		err := logger.Sync()
		if err != nil {
			logger.Error(fmt.Sprintf("failed to sync logger: %v", err))
		}
	}, 101)

	app.RegisterShutdown(
		"memory-compaction",
		pkg.MemoryCompactionCycle(father, 10*time.Millisecond, 10),
		100,
	)

	//debug.SetGCPercent(10)

	for i := 0; i < 50000; i++ {
		go func() {
			for {
				select {
				case <-father.Done():
					return
				default:
					rand.Seed(time.Now().UnixNano())

					min := 10
					max := 50
					randomNumber := rand.Intn(max-min+1) + min
					_ = make([]byte, randomNumber*1024) // kb
					time.Sleep(1000 * time.Millisecond)
				}
			}
		}()
	}

	//go func() {
	//	var stats runtime.MemStats
	//	for {
	//		select {
	//		case <-father.Done():
	//			return
	//		default:
	//			runtime.ReadMemStats(&stats)
	//			sys := stats.Sys
	//			log.Printf("[GC] RUN. Virtual size: %dmB, Heap size: %dmB\n", sys/1024/1024, stats.HeapAlloc/1024/1024)
	//			time.Sleep(10 * time.Millisecond)
	//		}
	//	}
	//}()

	app.Run()
}
