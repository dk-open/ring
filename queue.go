package ring

import (
	"fmt"
	"github.com/dk-open/ring/pad"
	"runtime"
	"time"
)

type IQueue[T any] interface {
	MustEnqueue(item T) error
	Enqueue(v T) bool
	Dequeue() (res T, ok bool)
}

var (
	ErrCapacity = fmt.Errorf("capacity must be a power of two")
)

type queue[T any] struct {
	buffer     []T
	cap        uint64
	capMask    uint64
	capX2      uint64
	head, tail pad.AtomicUint64
}

func Queue[T any](capacity uint64) (IQueue[T], error) {
	if capacity <= 0 || capacity&(capacity-1) != 0 {
		return nil, ErrCapacity
	}
	return &queue[T]{
		buffer:  make([]T, capacity),
		capMask: capacity - 1,
		cap:     capacity,
		capX2:   capacity*2 - 1,
	}, nil
}

func (q *queue[T]) Enqueue(item T) bool {
	head := q.head.Load()
	if head-q.tail.Load() >= q.capX2 {
		return false
	}

	nextHead := head + 1
	if q.head.CompareAndSwap(head, nextHead) {
		q.buffer[head>>1&q.capMask] = item
		q.head.Store(nextHead + 1)
		return true
	}

	return false
}

func (q *queue[T]) MustEnqueue(item T) error {
	attempt := 0
	for {
		head := q.head.Load()
		if head-q.tail.Load() >= q.capX2 {
			attempt++
			if err := enqueueBackoff(attempt); err != nil {
				return fmt.Errorf("enqueue failed after %d attempts: %w", attempt, err)
			}
			continue
		}

		nextHead := head + 1
		if q.head.CompareAndSwap(head, nextHead) {
			q.buffer[head>>1&q.capMask] = item
			q.head.Store(nextHead + 1)
			return nil
		}
		attempt++
		if err := enqueueBackoff(attempt); err != nil {
			return fmt.Errorf("enqueue failed after %d attempts: %w", attempt, err)
		}
		continue
	}
}

func (q *queue[T]) Dequeue() (res T, ok bool) {
	//attempt := 0
	for {
		tail := q.tail.Load()
		head := q.head.Load()
		if tail == head {
			return
		}
		if tail&1 == 1 || head-tail < 2 {
			runtime.Gosched()
			continue
		}

		nextTail := tail + 1
		if q.tail.CompareAndSwap(tail, nextTail) {
			res = q.buffer[tail>>1&q.capMask]
			q.tail.Store(nextTail + 1)
			return res, true
		}
		runtime.Gosched()
	}
}

func enqueueBackoff(attempt int) error {
	switch {
	case attempt < 5:
		// On modern CPUs, can hint with a PAUSE (Go does not expose directly)
		// Just an empty loop does nothing, but you could do:
		// runtime_procPin()... // not exposed
		// For real, just do nothing
	case attempt < 20:
		runtime.Gosched() // Let Go scheduler run another goroutine
	case attempt < 10000:
		// Exponential backoff, up to a max
		d := time.Microsecond << uint(attempt-20)
		if d > 5*time.Millisecond {
			d = 5 * time.Millisecond
		}
		time.Sleep(d)
	default:
		return fmt.Errorf("enqueue failed after %d attempts", attempt)
	}
	return nil
}
