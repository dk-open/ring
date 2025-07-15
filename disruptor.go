package ring

import (
	"context"
	"fmt"
	"github.com/dk-open/ring/pad"
	"runtime"
	"time"
)

type IDisruptor[T any] interface {
	Enqueue(item T) bool
	MustEnqueue(item T) error
}

type IDisruptorRing[T any] interface {
	Dequeue() (res T, ok bool)
}

type ReaderCallback[T any] func(value T)

type disruptor[T any] struct {
	buffer        []T
	cap           uint64
	capMask       uint64
	capX2         uint64
	writerCursor  pad.AtomicUint64
	readerBarrier pad.Barrier
}

func Disruptor[T any](ctx context.Context, capacity uint64, readers ...ReaderCallback[T]) (IDisruptor[T], error) {
	if capacity <= 0 || capacity&(capacity-1) != 0 {
		return nil, ErrCapacity
	}
	res := &disruptor[T]{
		buffer:  make([]T, capacity),
		capMask: capacity - 1,
		cap:     capacity,
		capX2:   capacity*2 - 1,
	}
	barriers := pad.MinBarrier{}
	for _, o := range readers {
		barriers = append(barriers, runReader(ctx, res, o))
	}

	res.readerBarrier = barriers
	return res, nil
}

func (d *disruptor[T]) Enqueue(item T) bool {
	head := d.writerCursor.Load()
	if head-d.readerBarrier.Load() >= d.capX2 {
		return false
	}

	nextHead := head + 1
	if d.writerCursor.CompareAndSwap(head, nextHead) {
		d.buffer[head>>1&d.capMask] = item
		d.writerCursor.Store(nextHead + 1)
		return true
	}
	return false
}

func (d *disruptor[T]) MustEnqueue(item T) error {
	attempt := 0
	for {
		head := d.writerCursor.Load()
		if head-d.readerBarrier.Load() >= d.capX2 {
			if err := backoff(attempt); err != nil {
				return fmt.Errorf("enqueue failed after %d attempts: %w", attempt, err)
			}
			continue
		}

		nextHead := head + 1
		if d.writerCursor.CompareAndSwap(head, nextHead) {
			d.buffer[head>>1&d.capMask] = item
			d.writerCursor.Store(nextHead + 1)
			return nil
		}
		attempt++
		if err := backoff(attempt); err != nil {
			return fmt.Errorf("enqueue failed after %d attempts: %w", attempt, err)
		}
		continue
	}
}

func backoff(attempt int) error {
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
