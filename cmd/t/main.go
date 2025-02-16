package main

import (
	"fmt"
	"runtime"
	"weak"
)

type Data struct {
	Value int
	Blob  *Blob
}

// createWeak возвращает слабую ссылку на объект Data,
// который создается внутри функции и после её завершения не имеет сильных ссылок.
func createWeak(d *Data) weak.Pointer[Data] {
	w := weak.Make(d)
	d = nil
	return w
}

type Blob []byte

func (b Blob) String() string {
	return fmt.Sprintf("Blob(%d KB)", len(b)/1024)
}

// newBlob возвращает новый Blob указанного размера в КБ.
func newBlob(size int) *Blob {
	b := make([]byte, size*1024)
	for i := range size {
		b[i] = byte(i) % 255
	}
	return (*Blob)(&b)
}

func main() {
	heapSize := getAlloc()
	//wb := weak.Make(newBlob(1000))
	wb := createWeak(&Data{Value: 42})
	//wb := createWeak(&Data{Value: 42, Blob: newBlob(1000)})

	fmt.Println("value before GC =", wb.Value())
	runtime.GC()
	fmt.Println("value after GC =", wb.Value())
	fmt.Printf("heap size delta = %d KB\n", heapDelta(heapSize))

	fmt.Println(wb)
	fmt.Println(wb)
}

func heapDelta(prev uint64) uint64 {
	cur := getAlloc()
	if cur < prev {
		return 0
	}
	return cur - prev
}

// getAlloc возвращает текущий размер кучи в КБ.
func getAlloc() uint64 {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	return m.Alloc / 1024
}
