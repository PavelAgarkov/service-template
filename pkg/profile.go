package pkg

import (
	"log"
	"os"
	"runtime"
	"runtime/pprof"
)

func SaveProfiles() {
	runtime.SetBlockProfileRate(1) // Записывать каждую блокировку

	// Активируем аллокационное профилирование
	runtime.MemProfileRate = 1 // Записывать каждую аллокацию

	// Сохраняем CPU профиль
	f, err := os.Create("cpu.prof")
	if err != nil {
		log.Fatalf("Could not create CPU profile: %v", err)
	}
	defer f.Close()
	if err := pprof.StartCPUProfile(f); err != nil {
		log.Fatalf("Could not start CPU profile: %v", err)
	}
	log.Println("Running CPU profiling...")
	pprof.StopCPUProfile()
	log.Println("CPU profile saved")

	// Сохраняем профиль памяти (heap)
	heapFile, err := os.Create("heap.prof")
	if err != nil {
		log.Fatalf("Could not create heap profile: %v", err)
	}
	defer heapFile.Close()
	if err := pprof.WriteHeapProfile(heapFile); err != nil {
		log.Fatalf("Could not write heap profile: %v", err)
	}
	log.Println("Heap profile saved")

	// Сохраняем профиль блокировок
	blockFile, err := os.Create("block.prof")
	if err != nil {
		log.Fatalf("Could not create block profile: %v", err)
	}
	defer blockFile.Close()
	if err := pprof.Lookup("block").WriteTo(blockFile, 0); err != nil {
		log.Fatalf("Could not write block profile: %v", err)
	}
	log.Println("Block profile saved")

	// Сохраняем профиль горутин
	goroutineFile, err := os.Create("goroutines.prof")
	if err != nil {
		log.Fatalf("Could not create goroutines profile: %v", err)
	}
	defer goroutineFile.Close()
	if err := pprof.Lookup("goroutine").WriteTo(goroutineFile, 0); err != nil {
		log.Fatalf("Could not write goroutines profile: %v", err)
	}
	log.Println("Goroutines profile saved")
}

func GetGoroutines() {
	p := pprof.Lookup("goroutine")
	p.WriteTo(os.Stdout, 1)
}
