package pkg

import (
	"context"
	"fmt"
	"go.uber.org/zap"
	"log"
	"runtime"
	"sync"
	"time"
)

type MemoryManager struct {
	rwMu       sync.RWMutex
	upperLimit uint64 // верхний порог (при превышении вызываем GC)
	lowerLimit uint64 // нижний порог (при падении ниже - сбрасываем "режим паники")
	exceeded   bool   // флаг, говорящий, что мы превысили upperLimit
	memStats   runtime.MemStats
	logger     *zap.Logger
}

func NewMemoryManager(mid uint64, logger *zap.Logger) *MemoryManager {
	return &MemoryManager{
		upperLimit: mid * 11 / 10,
		lowerLimit: mid * 9 / 10,
		logger:     logger,
	}
}

func (m *MemoryManager) GetCurrentAlloc() uint64 {
	m.rwMu.Lock()
	defer m.rwMu.Unlock()
	runtime.ReadMemStats(&m.memStats)
	return m.memStats.HeapAlloc
}

func (m *MemoryManager) CheckAndRunGC() {
	current := m.GetCurrentAlloc()
	//m.logger.Info(fmt.Sprintf("HeapAlloc: %d KB\n", current/1024))
	log.Println(fmt.Sprintf("HeapAlloc: %d KB\n", current/1024))

	switch {
	case !m.exceeded && current > m.upperLimit:
		// Память превысила верхний порог, запускаем GC
		//m.logger.Info(fmt.Sprintf("[GC] Usage exceeded upper limit (%dKB), run GC\n", m.upperLimit/1024))
		log.Println(fmt.Sprintf("[GC] Usage exceeded upper limit (%dKB), run GC\n", m.upperLimit/1024))
		runtime.GC()
		m.exceeded = true

	case m.exceeded && current < m.lowerLimit:
		// Если мы были выше порога, но теперь упали ниже нижнего, сбрасываем флаг
		//m.logger.Info(fmt.Sprintf("[GC] Usage dropped below lower limit (%dKB), reset 'exceeded'\n", m.lowerLimit/1024))
		log.Println(fmt.Sprintf("[GC] Usage dropped below lower limit (%dKB), reset 'exceeded'\n", m.lowerLimit/1024))
		m.exceeded = false

	default:
		//m.logger.Info(fmt.Sprintf("skip GC, current usage: %dKB\n", current/1024))
		log.Println(fmt.Sprintf("skip GC, current usage: %dKB\n", current/1024))
		// Иначе ничего не делаем: либо мы ещё не достигли upperLimit,
		// либо мы его достигли, но не опустились ниже lowerLimit
	}
}

func (m *MemoryManager) MemoryCompactionCycle(ctx context.Context, timeToCompaction time.Duration) func() {
	closer := make(chan struct{})
	go func() {
		defer func() {
			if r := recover(); r != nil {
				m.logger.Error("Recovered in MemoryCompactionCycle %v", zap.Any("recover", r))
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
