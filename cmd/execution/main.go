package main

import (
	"bytes"
	"fmt"
	"log"
	"time"

	"github.com/vmihailenco/msgpack/v5"
)

type ComplexData struct {
	Matrix    [][]int                   `msgpack:"matrix"`
	Mapping   map[string]map[string]int `msgpack:"mapping"`
	Timestamp time.Time                 `msgpack:"timestamp"`
	Email     string                    `msgpack:"email,omitempty"`
}

func main() {
	data := ComplexData{
		Matrix: [][]int{
			{1, 2, 3},
			{4, 5, 6},
			{7, 8, 9},
		},
		Mapping: map[string]map[string]int{
			"group1": {"a": 10, "b": 20},
			"group2": {"x": 100, "y": 200},
		},
		Timestamp: time.Now(),
	}

	// Создаем bytes.Buffer
	buf := new(bytes.Buffer)

	// Кодируем данные в буфер
	enc := msgpack.NewEncoder(buf)
	if err := enc.Encode(data); err != nil {
		log.Fatal("Ошибка кодирования:", err)
	}

	// Выводим копию бинарных данных (без конфликтов)
	fmt.Println("Бинарные данные:", buf.Bytes())

	// Теперь создаем новый декодер
	dec := msgpack.NewDecoder(buf)

	// Декодируем данные обратно
	var newData ComplexData
	if err := dec.Decode(&newData); err != nil {
		log.Fatal("Ошибка декодирования:", err)
	}

	// Вывод восстановленных данных
	log.Println("Декодированные данные:", newData)

	// Альтернативный способ сериализации/десериализации
	encoded, err := SerializeData(data)
	if err != nil {
		log.Fatalf("Ошибка при кодировании: %v", err)
	}
	fmt.Println("Бинарные данные (SerializeData):", encoded)

	var decoded ComplexData
	err = DeserializeData(encoded, &decoded)
	if err != nil {
		log.Fatalf("Ошибка при декодировании: %v", err)
	}

	// Вывод результата
	fmt.Println("Восстановленные данные:")
	fmt.Println("Matrix:", decoded.Matrix)
	fmt.Println("Mapping:", decoded.Mapping)
	fmt.Println("Timestamp:", decoded.Timestamp)
}

// Функция для сериализации
func SerializeData(data interface{}) ([]byte, error) {
	return msgpack.Marshal(data)
}

// Функция для десериализации
func DeserializeData(data []byte, target interface{}) error {
	return msgpack.Unmarshal(data, target)
}
