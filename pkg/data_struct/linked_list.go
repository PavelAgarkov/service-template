package data_struct

import (
	"errors"
	"fmt"
)

type Node[T any] struct {
	Value T
	next  *Node[T]
}

type LinkedList[T any] struct {
	head *Node[T]
	tail *Node[T]
	size int
}

func NewLinkedList[T any]() *LinkedList[T] {
	return &LinkedList[T]{}
}

func (ll *LinkedList[T]) Size() int {
	return ll.size
}

func (ll *LinkedList[T]) IsEmpty() bool {
	return ll.size == 0
}

func (ll *LinkedList[T]) String() string {
	if ll.IsEmpty() {
		return "[]"
	}
	result := "["
	current := ll.head
	for current != nil {
		result += fmt.Sprintf("%v", current.Value)
		if current.next != nil {
			result += " -> "
		}
		current = current.next
	}
	result += "]"
	return result
}

func (ll *LinkedList[T]) PushFront(value T) {
	newNode := &Node[T]{Value: value}

	if ll.IsEmpty() {
		ll.head = newNode
		ll.tail = newNode
	} else {
		newNode.next = ll.head
		ll.head = newNode
	}
	ll.size++
}

func (ll *LinkedList[T]) PushBack(value T) {
	newNode := &Node[T]{Value: value}

	if ll.IsEmpty() {
		ll.head = newNode
		ll.tail = newNode
	} else {
		ll.tail.next = newNode
		ll.tail = newNode
	}
	ll.size++
}

func (ll *LinkedList[T]) PopFront() (T, error) {
	var zero T
	if ll.IsEmpty() {
		return zero, errors.New("linked list is empty")
	}

	value := ll.head.Value
	ll.head = ll.head.next
	ll.size--

	// Если после удаления элемент отсутствует, то tail = nil
	if ll.size == 0 {
		ll.tail = nil
	}

	return value, nil
}

func (ll *LinkedList[T]) PopBack() (T, error) {
	var zero T
	if ll.IsEmpty() {
		return zero, errors.New("linked list is empty")
	}

	// Если в списке один элемент
	if ll.head == ll.tail {
		value := ll.head.Value
		ll.head = nil
		ll.tail = nil
		ll.size = 0
		return value, nil
	}

	current := ll.head
	for current.next != nil && current.next != ll.tail {
		current = current.next
	}
	value := ll.tail.Value
	current.next = nil
	ll.tail = current
	ll.size--

	return value, nil
}

func (ll *LinkedList[T]) Get(index int) (T, error) {
	var zero T
	if index < 0 || index >= ll.size {
		return zero, errors.New("index out of range")
	}

	current := ll.head
	for i := 0; i < index; i++ {
		current = current.next
	}
	return current.Value, nil
}

func (ll *LinkedList[T]) Insert(index int, value T) error {
	if index < 0 || index > ll.size {
		return errors.New("index out of range")
	}
	if index == 0 {
		ll.PushFront(value)
		return nil
	}
	if index == ll.size {
		ll.PushBack(value)
		return nil
	}

	newNode := &Node[T]{Value: value}
	current := ll.head
	for i := 0; i < index-1; i++ {
		current = current.next
	}

	newNode.next = current.next
	current.next = newNode
	ll.size++
	return nil
}

func (ll *LinkedList[T]) Remove(index int) (T, error) {
	var zero T
	if index < 0 || index >= ll.size {
		return zero, errors.New("index out of range")
	}

	if index == 0 {
		return ll.PopFront()
	}

	current := ll.head
	for i := 0; i < index-1; i++ {
		current = current.next
	}
	removed := current.next
	current.next = removed.next
	ll.size--

	if removed == ll.tail {
		ll.tail = current
	}
	return removed.Value, nil
}
