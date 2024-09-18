package lab0

import (
	"sync"
)

// Queue is a simple FIFO queue that is unbounded in size.
// Push may be called any number of times and is not
// expected to fail or overwrite existing entries.
type Queue[T any] struct {
	elements []T
	m        sync.Mutex
}

// NewQueue returns a new queue which is empty.
func NewQueue[T any]() *Queue[T] {
	empty := make([]T, 0)
	return &Queue[T]{
		elements: empty,
	}
}

// Push adds an item to the end of the queue.
func (q *Queue[T]) Push(item T) {
	q.m.Lock()
	defer q.m.Unlock()
	q.elements = append(q.elements, item)
}

// Pop removes an item from the beginning of the queue
// and returns it unless the queue is empty.
//
// If the queue is empty, returns the zero value for T and false.
//
// If you are unfamiliar with "zero values", consider revisiting
// this section of the Tour of Go: https://go.dev/tour/basics/12
func (q *Queue[T]) Pop() (T, bool) {
	q.m.Lock()
	defer q.m.Unlock()

	if len(q.elements) == 0 {
		var dflt T
		return dflt, false
	}
	item := q.elements[0]
	q.elements = q.elements[1:]
	return item, true
}

// ConcurrentQueue provides the same semantics as Queue but
// is safe to access from many goroutines at once.
//
// You can use your implementation of Queue[T] here.
//
// If you are stuck, consider revisiting this section of
// the Tour of Go: https://go.dev/tour/concurrency/9
type ConcurrentQueue[T any] struct {
	queue Queue[T]
	m     sync.Mutex
}

func NewConcurrentQueue[T any]() *ConcurrentQueue[T] {
	return &ConcurrentQueue[T]{
		queue: Queue[T]{},
	}
}

// Push adds an item to the end of the queue
func (q *ConcurrentQueue[T]) Push(t T) {
	q.m.Lock()
	defer q.m.Unlock()
	q.queue.Push(t)
}

func (q *ConcurrentQueue[T]) Pop() (T, bool) {
	q.m.Lock()
	defer q.m.Unlock()

	if len(q.queue.elements) == 0 {
		var dflt T
		return dflt, false
	}
	return q.queue.Pop()
}
