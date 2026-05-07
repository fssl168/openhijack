package utils

import (
	"sync"
)

type CircularBuffer[T any] struct {
	buffer     []T
	head       int
	tail       int
	size       int
	capacity   int
	mu         sync.RWMutex
	onEvicted func(T)
}

func NewCircularBuffer[T any](capacity int) *CircularBuffer[T] {
	if capacity <= 0 {
		capacity = 100
	}
	return &CircularBuffer[T]{
		buffer:   make([]T, capacity),
		capacity: capacity,
	}
}

func (cb *CircularBuffer[T]) Push(item T) {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	if cb.size == cb.capacity {
		if cb.onEvicted != nil {
			cb.onEvicted(cb.buffer[cb.tail])
		}
		cb.tail = (cb.tail + 1) % cb.capacity
	} else {
		cb.size++
	}

	cb.buffer[cb.head] = item
	cb.head = (cb.head + 1) % cb.capacity
}

func (cb *CircularBuffer[T]) PushMany(items ...T) {
	for _, item := range items {
		cb.Push(item)
	}
}

func (cb *CircularBuffer[T]) Pop() (T, bool) {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	var zero T

	if cb.size == 0 {
		return zero, false
	}

	item := cb.buffer[cb.tail]
	var empty T
	cb.buffer[cb.tail] = empty
	cb.tail = (cb.tail + 1) % cb.capacity
	cb.size--

	return item, true
}

func (cb *CircularBuffer[T]) Peek() (T, bool) {
	cb.mu.RLock()
	defer cb.mu.RUnlock()

	var zero T

	if cb.size == 0 {
		return zero, false
	}

	return cb.buffer[cb.tail], true
}

func (cb *CircularBuffer[T]) ToArray() []T {
	cb.mu.RLock()
	defer cb.mu.RUnlock()

	result := make([]T, 0, cb.size)
	
	for i := 0; i < cb.size; i++ {
		idx := (cb.tail + i) % cb.capacity
		result = append(result, cb.buffer[idx])
	}
	
	return result
}

func (cb *CircularBuffer[T]) ToReversedArray() []T {
	cb.mu.RLock()
	defer cb.mu.RUnlock()

	result := make([]T, 0, cb.size)
	
	for i := cb.size - 1; i >= 0; i-- {
		idx := (cb.tail + i) % cb.capacity
		result = append(result, cb.buffer[idx])
	}
	
	return result
}

func (cb *CircularBuffer[T]) Len() int {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return cb.size
}

func (cb *CircularBuffer[T]) Capacity() int {
	return cb.capacity
}

func (cb *CircularBuffer[T]) IsEmpty() bool {
	return cb.Len() == 0
}

func (cb *CircularBuffer[T]) IsFull() bool {
	return cb.size == cb.capacity
}

func (cb *CircularBuffer[T]) Clear() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	for i := 0; i < cb.capacity; i++ {
		var zero T
		cb.buffer[i] = zero
	}
	cb.head = 0
	cb.tail = 0
	cb.size = 0
}

func (cb *CircularBuffer[T]) SetOnEvicted(callback func(T)) {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	cb.onEvicted = callback
}

func (cb *CircularBuffer[T]) ForEach(fn func(int, T) bool) {
	cb.mu.RLock()
	defer cb.mu.RUnlock()

	count := 0
	for i := 0; i < cb.size; i++ {
		idx := (cb.tail + i) % cb.capacity
		if !fn(count, cb.buffer[idx]) {
			break
		}
		count++
	}
}

func (cb *CircularBuffer[T]) Find(predicate func(T) bool) (T, bool) {
	cb.mu.RLock()
	defer cb.mu.RUnlock()

	var zero T
	
	for i := 0; i < cb.size; i++ {
		idx := (cb.tail + i) % cb.capacity
		if predicate(cb.buffer[idx]) {
			return cb.buffer[idx], true
		}
	}
	
	return zero, false
}

func (cb *CircularBuffer[T]) Filter(predicate func(T) bool) []T {
	cb.mu.RLock()
	defer cb.mu.RUnlock()

	var result []T
	
	for i := 0; i < cb.size; i++ {
		idx := (cb.tail + i) % cb.capacity
		item := cb.buffer[idx]
		if predicate(item) {
			result = append(result, item)
		}
	}
	
	return result
}

func (cb *CircularBuffer[T]) Slice(start, end int) []T {
	cb.mu.RLock()
	defer cb.mu.RUnlock()

	if start < 0 {
		start = 0
	}
	if end > cb.size {
		end = cb.size
	}
	if start >= end {
		return []T{}
	}

	result := make([]T, 0, end-start)
	for i := start; i < end; i++ {
		idx := (cb.tail + i) % cb.capacity
		result = append(result, cb.buffer[idx])
	}
	return result
}

func (cb *CircularBuffer[T]) Last(n int) []T {
	if n <= 0 {
		return []T{}
	}
	start := cb.size - n
	if start < 0 {
		start = 0
	}
	return cb.Slice(start, cb.size)
}

func (cb *CircularBuffer[T]) Reduce(initial any, reducer func(any, T) any) any {
	cb.mu.RLock()
	defer cb.mu.RUnlock()

	acc := initial
	
	for i := 0; i < cb.size; i++ {
		idx := (cb.tail + i) % cb.capacity
		acc = reducer(acc, cb.buffer[idx])
	}
	
	return acc
}
