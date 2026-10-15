// Package history provides fixed-size time series storage.
package history

// Ring is a fixed-capacity FIFO that overwrites its oldest element when full.
type Ring[T any] struct {
	buf  []T
	head int
	size int
}

// New returns a ring holding at most capacity elements.
func New[T any](capacity int) *Ring[T] {
	return &Ring[T]{buf: make([]T, max(capacity, 1))}
}

// Push appends v, evicting the oldest element if the ring is full.
func (r *Ring[T]) Push(v T) {
	r.buf[(r.head+r.size)%len(r.buf)] = v
	if r.size < len(r.buf) {
		r.size++
		return
	}
	r.head = (r.head + 1) % len(r.buf)
}

// Len reports the number of stored elements.
func (r *Ring[T]) Len() int { return r.size }

// Cap reports the ring capacity.
func (r *Ring[T]) Cap() int { return len(r.buf) }

// Last returns the newest element.
func (r *Ring[T]) Last() (T, bool) {
	if r.size == 0 {
		var zero T
		return zero, false
	}
	return r.buf[(r.head+r.size-1)%len(r.buf)], true
}

// Tail returns up to n of the newest elements, oldest first.
func (r *Ring[T]) Tail(n int) []T {
	n = min(max(n, 0), r.size)
	out := make([]T, n)
	start := r.head + r.size - n
	for i := range n {
		out[i] = r.buf[(start+i)%len(r.buf)]
	}
	return out
}

// Values returns all elements, oldest first.
func (r *Ring[T]) Values() []T { return r.Tail(r.size) }
