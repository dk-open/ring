package ring

import (
	"context"
	"github.com/dk-open/ring/pad"
	"runtime"
	"time"
)

type disruptorReader[T any] struct {
	tail pad.AtomicUint64
	d    *disruptor[T]
	f    ReaderCallback[T]
}

func runReader[T any](ctx context.Context, d *disruptor[T], f ReaderCallback[T]) pad.Barrier {
	r := &disruptorReader[T]{
		d: d,
		f: f,
	}
	go func() {
		var attempt uint64
		for {
			select {
			case <-ctx.Done():
				return
			default:
				tail := r.tail.Load()
				if head := r.d.writerCursor.Load(); tail+1 < head {
					for tail < head {
						r.f(r.d.buffer[tail>>1&r.d.capMask])
						tail += 2
					}
					r.tail.Store(tail)
					attempt = 0 // reset attempt counter after successful read
					continue
				}
				readerYield(attempt)
				attempt++
			}

		}
	}()

	return &r.tail
}

func readerYield(attempt uint64) {
	switch {
	case attempt < 20:
		runtime.Gosched() // Let Go scheduler run another goroutine
	default:
		d := time.Microsecond << uint(attempt-20)
		if d > time.Millisecond {
			d = time.Millisecond
		}
		time.Sleep(d)
	}
}
