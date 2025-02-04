package main

import (
	"fmt"
	"golang.org/x/exp/constraints"
)

//  1. Определим ограничение для чисел.
//     Оно объединяет int, float64 и все их «псевдонимы» (например, type MyInt int).
type Number interface {
	constraints.Integer | constraints.Float | string
}

// 2. Для второго типа используем стандартный интерфейс fmt.Stringer.
//    Он требует, чтобы тип имел метод String() string.

// 3. Определяем свой интерфейс Keyer для третьего типа.
type Keyer interface {
	Key() string
}

// Summarize — обобщённая функция, которая принимает три разных типа:
// A — это числовой тип (Number),
// B — это любой тип, реализующий fmt.Stringer,
// C — это любой тип, реализующий Keyer.
func Summarize[A Number, B fmt.Stringer, C Keyer](a A, b B, c C) {
	fmt.Printf("Number (A):   %v\n", a)
	fmt.Printf("String (B):   %s\n", b.String())
	fmt.Printf("Key (C):      %s\n", c.Key())
	fmt.Println()
}

// -------- Пример пользовательских типов --------

// MyFloat — свой тип на базе float64 (подходит под Number).
type MyFloat float64

// MyString — свой тип на базе string, он реализует fmt.Stringer.
type MyString string

func (s MyString) String() string {
	return string(s) // Просто возвращаем строку
}

// Person — структура, которая реализует Keyer.
type Person struct {
	Name string
}

func (p Person) Key() string {
	return p.Name
}

func main() {
	// Вариант 1: Передаём int (A), MyString (B), Person (C)
	Summarize(42, MyString("hello"), Person{Name: "Alice"})
	//   A = int, B = MyString, C = Person

	// Вариант 2: Передаём MyFloat (A), MyString (B), Person (C)
	Summarize(MyFloat(3.14), MyString("example"), Person{Name: "Bob"})
	//   A = MyFloat, B = MyString, C = Person

	Summarize("name", MyString("example"), Person{Name: "Bob"})
	// Вы можете комбинировать любые подходящие типы,
	// главное — соблюсти ограничения: Number, fmt.Stringer и Keyer.
}
