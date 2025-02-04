package pkg

import (
	"context"
	"go.uber.org/zap"
	"log"
	"runtime"
	"runtime/debug"
	"time"
)

type memoryManager struct {
	heapSizePercent uint64
}

func newMemoryManager(heapSizePercent uint64) *memoryManager {
	return &memoryManager{
		heapSizePercent: heapSizePercent,
	}
}

func (m *memoryManager) checkAndRunGC() {
	var stats runtime.MemStats
	runtime.ReadMemStats(&stats)
	sys := stats.Sys

	if stats.HeapAlloc >= sys*m.heapSizePercent/100 {
		log.Printf("[GC] RUN. Virtual size: %dmB, Heap size: %dmB\n", sys/1024/1024, stats.HeapAlloc/1024/1024)
		debug.FreeOSMemory()
	}
}

func MemoryCompactionCycle(ctx context.Context, timeToCompaction time.Duration, heapSizePercent uint64) func() {
	m := newMemoryManager(heapSizePercent)
	closer := make(chan struct{})
	go func() {
		runtime.LockOSThread()
		defer runtime.UnlockOSThread()
		defer func() {
			if r := recover(); r != nil {
				log.Println("Recovered in MemoryCompactionCycle %v", zap.Any("recover", r))
			}
		}()
		ticker := time.NewTicker(timeToCompaction)
		defer ticker.Stop()
		for {
			select {
			case <-closer:
				return
			case <-ctx.Done():
				return
			case <-ticker.C:
				m.checkAndRunGC()
			}
		}
	}()

	return func() {
		close(closer)
	}
}
