package ring

import (
	"context"
	"fmt"
	"github.com/dk-open/ring/pad"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestDisruptor_BasicEnqueue(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var received []int
	var mu sync.Mutex

	reader := func(value int) {
		mu.Lock()
		received = append(received, value)
		mu.Unlock()
	}

	d, err := Disruptor(ctx, 8, reader)
	if err != nil {
		t.Fatalf("Failed to create disruptor: %v", err)
	}
	// Test basic enqueue
	for i := 0; i < 5; i++ {
		if !d.Enqueue(i) {
			t.Fatalf("Failed to enqueue item %d", i)
		}
	}

	// Wait for readers to process
	time.Sleep(100 * time.Millisecond)

	mu.Lock()
	if len(received) != 5 {
		t.Errorf("Expected 5 items, got %d", len(received))
	}
	mu.Unlock()
}

func TestDisruptor_MultipleReaders(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var received1, received2 []int
	var mu1, mu2 sync.Mutex

	reader1 := func(value int) {
		mu1.Lock()
		received1 = append(received1, value)
		mu1.Unlock()
	}

	reader2 := func(value int) {
		mu2.Lock()
		received2 = append(received2, value)
		mu2.Unlock()
	}

	d, err := Disruptor(ctx, 16, reader1, reader2)
	if err != nil {
		t.Fatalf("Failed to create disruptor: %v", err)
	}
	// Enqueue items
	for i := 0; i < 10; i++ {
		if !d.Enqueue(i) {
			t.Fatalf("Failed to enqueue item %d", i)
		}
	}

	// Wait for processing
	time.Sleep(200 * time.Millisecond)

	mu1.Lock()
	mu2.Lock()

	// Both readers should receive all items
	if len(received1) != 10 || len(received2) != 10 {
		t.Errorf("Expected both readers to receive 10 items, got %d and %d",
			len(received1), len(received2))
	}

	mu1.Unlock()
	mu2.Unlock()
}

func TestDisruptor_ConcurrentEnqueue(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var received []int
	var mu sync.Mutex

	reader := func(value int) {
		mu.Lock()
		received = append(received, value)
		mu.Unlock()
	}

	d, err := Disruptor(ctx, 32, reader)
	if err != nil {
		t.Fatalf("Failed to create disruptor: %v", err)
	}
	// Concurrent enqueuers
	var wg sync.WaitGroup
	itemsPerGoroutine := 10
	numGoroutines := 3

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(start int) {
			defer wg.Done()
			for j := 0; j < itemsPerGoroutine; j++ {
				d.Enqueue(start*100 + j)
			}
		}(i)
	}

	wg.Wait()
	time.Sleep(100 * time.Millisecond)
	mu.Lock()

	if len(received) == 0 {
		t.Error("Expected to receive some items from concurrent enqueue")
	}
	mu.Unlock()
}

func BenchmarkDisruptorMultiReader(b *testing.B) {
	readerCounts := []int{1, 2, 4}

	for _, numReaders := range readerCounts {
		b.Run(fmt.Sprintf("Disruptor_%d_readers", numReaders), func(b *testing.B) {
			ctx, cancel := context.WithCancel(b.Context())
			defer cancel()

			var wg sync.WaitGroup
			var readers []ReaderCallback[int]

			counter := pad.AtomicUint64{}
			totalNum := uint64(b.N * numReaders)
			wg.Add(1)
			for i := 0; i < numReaders; i++ {
				readers = append(readers, func(value int) {
					if counter.Add(1) == totalNum {
						wg.Done()
					}
				})
			}
			time.Sleep(100 * time.Millisecond) // Allow time for setup

			d, err := Disruptor(ctx, 1024, readers...)
			if err != nil {
				b.Fatalf("Failed to create disruptor: %v", err)
			}

			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_ = d.MustEnqueue(i)
			}
			// Wait for all readers to process
			wg.Wait()
		})

		b.Run(fmt.Sprintf("Channel_%d_readers", numReaders), func(b *testing.B) {
			chx := make(chan int, 1024)
			var wg sync.WaitGroup

			counter := pad.AtomicUint64{}
			totalNum := uint64(b.N * numReaders)
			wg.Add(1)
			// Start consumers
			for i := 0; i < numReaders; i++ {
				go func(idx int) {
					for range chx {
						if v := counter.Add(1); v == totalNum {
							wg.Done()
						}
					}
				}(i)
			}
			time.Sleep(100 * time.Millisecond)

			b.ReportAllocs()
			b.ResetTimer()

			// The same number of items as in the disruptor benchmark
			n := b.N * numReaders
			for i := 0; i < n; i++ {
				select {
				case chx <- i:
				default:
					// Channel full, retry
					i-- // retry this iteration
				}
			}

			wg.Wait()
			close(chx)
		})
	}
}

func TestDisruptorSingleThreaded(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var counter int64
	reader := func(value int) {
		atomic.AddInt64(&counter, 1)
	}

	d, err := Disruptor(ctx, 512, reader)
	if err != nil {
		t.Fatalf("Failed to create disruptor: %v", err)
	}
	n := 10000
	for i := 0; i < n; i++ {
		if err := d.MustEnqueue(i); err != nil {
			t.Fatalf("MustEnqueue failed: %v", err)
		}
	}
	// Wait for processing
	time.Sleep(10 * time.Millisecond)
}

func TestDisruptorTwoThreaded(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var wg sync.WaitGroup
	counter := pad.AtomicUint64{}
	var readers []ReaderCallback[int]

	nReaders := 3
	n := 100000
	totalNum := uint64(n * nReaders)
	wg.Add(1)
	for i := 0; i < nReaders; i++ {
		readers = append(readers, func(value int) {
			if counter.Add(1) == totalNum {
				wg.Done()
			}
		})
	}

	time.Sleep(1 * time.Microsecond) // Allow time for setup
	d, err := Disruptor[int](ctx, 1024, readers...)
	if err != nil {
		t.Fatalf("Failed to create disruptor: %v", err)
	}
	for i := 0; i < n; i++ {
		if err := d.MustEnqueue(i); err != nil {
			t.Fatalf("MustEnqueue failed: %v", err)
		}
	}

	wg.Wait()
}

// Additional benchmark for single-threaded performance
func BenchmarkDisruptorMultiThreaded(b *testing.B) {
	ctx, cancel := context.WithCancel(b.Context())
	defer cancel()

	var wg sync.WaitGroup
	counter := pad.AtomicUint64{}
	var readers []ReaderCallback[int]

	nReaders := 2
	totalNum := uint64(b.N * nReaders)
	wg.Add(1)
	for i := 0; i < nReaders; i++ {
		readers = append(readers, func(value int) {
			if counter.Add(1) == totalNum {
				wg.Done()
				return
			}
		})
	}

	d, err := Disruptor[int](ctx, 512, readers...)
	if err != nil {
		b.Fatalf("Failed to create disruptor: %v", err)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := d.MustEnqueue(i); err != nil {
			b.Fatalf("MustEnqueue failed: %v", err)
		}
	}
	wg.Wait()
}
