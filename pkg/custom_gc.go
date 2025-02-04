package pkg

import (
	"context"
	"fmt"
	"go.uber.org/zap"
	"log"
	"runtime"
	"runtime/debug"
	"sync"
	"time"
)

type MemoryManager struct {
	rwMu       sync.RWMutex
	upperLimit uint64
	memStats   runtime.MemStats
}

func NewMemoryManager(mid uint64, percent int) *MemoryManager {
	return &MemoryManager{
		upperLimit: mid - uint64(float64(percent/100)*float64(mid)),
	}
}

func (m *MemoryManager) GetCurrentSysAlloc() uint64 {
	m.rwMu.Lock()
	defer m.rwMu.Unlock()
	runtime.ReadMemStats(&m.memStats)
	return m.memStats.Sys
}

func (m *MemoryManager) GetCurrentHeap() uint64 {
	m.rwMu.Lock()
	defer m.rwMu.Unlock()
	runtime.ReadMemStats(&m.memStats)
	return m.memStats.HeapAlloc
}

func (m *MemoryManager) CheckAndRunGC() {
	sys := m.GetCurrentSysAlloc()
	if sys >= m.upperLimit {
		heap := m.GetCurrentHeap()
		log.Println(fmt.Sprintf("[GC]RUN Usage managed a heap size"))
		log.Println(fmt.Sprintf("virtual size: %dKB\n", sys/1024))
		log.Println(fmt.Sprintf("heap size: %dKB\n", heap/1024))
		debug.FreeOSMemory()
	}
}

func (m *MemoryManager) MemoryCompactionCycle(ctx context.Context, timeToCompaction time.Duration) func() {
	closer := make(chan struct{})
	go func() {
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
				m.CheckAndRunGC()
			}
		}
	}()

	return func() {
		close(closer)
	}
}
