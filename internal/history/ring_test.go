package history

import (
	"slices"
	"testing"
)

func TestRing(t *testing.T) {
	r := New[int](3)
	if _, ok := r.Last(); ok {
		t.Fatal("Last on empty ring reported ok")
	}

	for i := 1; i <= 5; i++ {
		r.Push(i)
	}

	if got, want := r.Len(), 3; got != want {
		t.Fatalf("Len = %d, want %d", got, want)
	}
	if got, want := r.Values(), []int{3, 4, 5}; !slices.Equal(got, want) {
		t.Fatalf("Values = %v, want %v", got, want)
	}
	if got, want := r.Tail(2), []int{4, 5}; !slices.Equal(got, want) {
		t.Fatalf("Tail(2) = %v, want %v", got, want)
	}
	if got := r.Tail(10); len(got) != 3 {
		t.Fatalf("Tail(10) len = %d, want 3", len(got))
	}
	if v, _ := r.Last(); v != 5 {
		t.Fatalf("Last = %d, want 5", v)
	}
}

func TestRingMinimumCapacity(t *testing.T) {
	r := New[string](0)
	r.Push("a")
	r.Push("b")
	if got := r.Values(); !slices.Equal(got, []string{"b"}) {
		t.Fatalf("Values = %v", got)
	}
}

func BenchmarkRingPush(b *testing.B) {
	r := New[float64](512)
	for i := range b.N {
		r.Push(float64(i))
	}
}
