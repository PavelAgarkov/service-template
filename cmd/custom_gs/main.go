package main

import (
	"context"
	"fmt"
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

	go func() {
		memoryManager := pkg.NewMemoryManager(3*1024*1024, logger)
		gcStoper := memoryManager.MemoryCompactionCycle(father, 5000*time.Millisecond)
		app.RegisterShutdown("memory-compaction", gcStoper, 100)
		logger.Info("Memory Compaction Cycle started")
	}()

	_ = make([]byte, 4*1024*1024)

	app.Run()
}
