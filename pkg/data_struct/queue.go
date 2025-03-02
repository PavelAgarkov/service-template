package data_struct

import "errors"

type Queue[T any] struct {
	items []T
}

func NewQueue[T any]() *Queue[T] {
	return &Queue[T]{
		items: []T{},
	}
}

func (q *Queue[T]) Enqueue(value T) {
	q.items = append(q.items, value)
}

func (q *Queue[T]) Dequeue() (T, error) {
	var zero T
	if len(q.items) == 0 {
		return zero, errors.New("empty queue")
	}
	value := q.items[0]
	q.items = q.items[1:]
	return value, nil
}

func (q *Queue[T]) Peek() (T, error) {
	var zero T
	if len(q.items) == 0 {
		return zero, errors.New("empty queue")
	}
	return q.items[0], nil
}

func (q *Queue[T]) Size() int {
	return len(q.items)
}

func (q *Queue[T]) IsEmpty() bool {
	return len(q.items) == 0
}
