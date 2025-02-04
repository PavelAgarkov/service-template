package pkg

import (
	"context"
	"go.uber.org/zap"
	"log"
	"runtime"
	"runtime/debug"
	"sync"
	"time"
)

type MemoryManager struct {
	rwMu            sync.RWMutex
	upperLimit      uint64
	memStats        runtime.MemStats
	diff            int64
	heapSizePercent uint64
}

func NewMemoryManager(mid uint64, heapSizePercent uint64) *MemoryManager {
	return &MemoryManager{
		//upperLimit:      mid - uint64(float64(upperLimitPercent/100)*float64(mid)),
		upperLimit:      mid,
		heapSizePercent: heapSizePercent,
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
	// Блокируем мьютекс для защиты доступа к общим полям (m.diff)
	m.rwMu.Lock()
	defer m.rwMu.Unlock()

	// Читаем статистику памяти напрямую (чтобы не было двойного лока, если методы GetCurrentSysAlloc/GetCurrentHeap тоже используют m.rwMu)
	var stats runtime.MemStats
	runtime.ReadMemStats(&stats)
	sys := stats.Sys

	// Вычисляем разницу между заданным лимитом и текущей виртуальной памятью
	currentDiff := int64(m.upperLimit) - int64(sys)

	// Если m.diff ещё не инициализировано (равно 0), задаём его текущим значением
	if m.diff == 0 {
		m.diff = currentDiff
	}

	// Если разница увеличилась, значит, используемая память снижается – GC пропускаем
	if currentDiff > m.diff {
		log.Println("skip gc because memory usage is decreasing")
		m.diff = currentDiff // обновляем для следующего вызова
		return
	}

	// Если текущее использование памяти превышает лимит и тенденция не положительная – запускаем GC
	allowedHeapSize := sys * m.heapSizePercent / 100
	log.Println("now 2/3 sym", (allowedHeapSize)/1024)
	if stats.HeapAlloc >= allowedHeapSize {
		//if sys >= m.upperLimit {
		log.Printf("[GC] RUN. Virtual size: %dmB, Heap size: %dmB\n", sys/1024/1024, stats.HeapAlloc/1024/1024)

		//log.Printf("[GC] RUN. Virtual size: %dKB, Heap size: %dKB\n", sys/1024, stats.HeapAlloc/1024)
		debug.FreeOSMemory()
	}

	// Обновляем значение для отслеживания тренда на следующий вызов
	m.diff = currentDiff
}

func (m *MemoryManager) MemoryCompactionCycle(ctx context.Context, timeToCompaction time.Duration) func() {
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
				m.CheckAndRunGC()
			}
		}
	}()

	return func() {
		close(closer)
	}
}
