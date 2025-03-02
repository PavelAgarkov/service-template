package data_struct

import "errors"

type Stack[T any] struct {
	items []T
}

func NewStack[T any]() *Stack[T] {
	return &Stack[T]{
		items: []T{},
	}
}

func (s *Stack[T]) Push(value T) {
	s.items = append(s.items, value)
}

func (s *Stack[T]) Pop() (T, error) {
	var zero T
	if len(s.items) == 0 {
		return zero, errors.New("empty stack")
	}
	topIndex := len(s.items) - 1
	value := s.items[topIndex]
	s.items = s.items[:topIndex]
	return value, nil
}

func (s *Stack[T]) Peek() (T, error) {
	var zero T
	if len(s.items) == 0 {
		return zero, errors.New("empty stack")
	}
	return s.items[len(s.items)-1], nil
}

func (s *Stack[T]) Size() int {
	return len(s.items)
}

func (s *Stack[T]) IsEmpty() bool {
	return len(s.items) == 0
}
