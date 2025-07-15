package ring

import (
	"fmt"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
)

// BenchmarkQueue_CompareGoImplementations сравнивает нашу реализацию с другими Go вариантами
func BenchmarkQueue_CompareGoImplementations(b *testing.B) {
	b.Run("RingQueue Capacity: 256 Reader: 1", func(b *testing.B) {
		benchmarkOurImplementation(b, 256, 1, 1)
	})

	b.Run("RingQueue Capacity: 256 Reader: 2", func(b *testing.B) {
		benchmarkOurImplementation(b, 256, 1, 2)
	})

	b.Run("RingQueue Capacity: 256 Reader: 4", func(b *testing.B) {
		benchmarkOurImplementation(b, 256, 1, 4)
	})

	b.Run("GoChannels Capacity: 256 Reader: 1", func(b *testing.B) {
		benchmarkGoChannels(b, 256, 1, 1)
	})

	b.Run("GoChannels Capacity: 256 Reader: 2", func(b *testing.B) {
		benchmarkGoChannels(b, 256, 1, 2)
	})

	b.Run("GoChannels Capacity: 256 Reader: 4", func(b *testing.B) {
		benchmarkGoChannels(b, 256, 1, 4)
	})

	b.Run("RingQueue Capacity: 1024 Reader: 1", func(b *testing.B) {
		benchmarkOurImplementation(b, 1024, 1, 1)
	})

	b.Run("RingQueue Capacity: 1024 Reader: 2", func(b *testing.B) {
		benchmarkOurImplementation(b, 1024, 1, 2)
	})

	b.Run("RingQueue Capacity: 1024 Reader: 4", func(b *testing.B) {
		benchmarkOurImplementation(b, 1024, 1, 4)
	})

	b.Run("GoChannels Capacity: 1024 Reader: 1", func(b *testing.B) {
		benchmarkGoChannels(b, 1024, 1, 1)
	})

	b.Run("GoChannels Capacity: 1024 Reader: 2", func(b *testing.B) {
		benchmarkGoChannels(b, 1024, 1, 2)
	})

	b.Run("GoChannels Capacity: 1024 Reader: 4", func(b *testing.B) {
		benchmarkGoChannels(b, 1024, 1, 4)
	})
}

// Наша реализация
func benchmarkOurImplementation(b *testing.B, capacity int, numProducers, numConsumers int) {
	q, err := Queue[int](uint64(capacity))
	if err != nil {
		b.Fatalf("Failed to create queue: %v", err)
	}

	var consumed int64
	var wg sync.WaitGroup

	// Consumers
	for i := 0; i < numConsumers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				if _, success := q.Dequeue(); success {
					if atomic.AddInt64(&consumed, 1) >= int64(b.N) {
						return
					}
				}
				if atomic.LoadInt64(&consumed) >= int64(b.N) {
					return
				}
			}
		}()
	}

	b.ReportAllocs()
	b.ResetTimer()

	// Producers
	for i := 0; i < numProducers; i++ {
		wg.Add(1)
		go func(producerID int) {
			defer wg.Done()
			itemsPerProducer := b.N / numProducers
			for j := 0; j < itemsPerProducer; j++ {
				val := producerID*itemsPerProducer + j
				if err = q.MustEnqueue(val); err != nil {
					fmt.Printf("Producer %d failed to enqueue item %d: %v\n", producerID, val, err)
				}
			}
		}(i)
	}

	wg.Wait()
}

// Go channels (baseline)
func benchmarkGoChannels(b *testing.B, capacity int, numProducers, numConsumers int) {
	ch := make(chan int, capacity)
	var consumed int64
	var wg sync.WaitGroup

	// Consumers
	for i := 0; i < numConsumers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case val := <-ch:
					_ = val
					if atomic.AddInt64(&consumed, 1) >= int64(b.N) {
						return
					}
				default:
					if atomic.LoadInt64(&consumed) >= int64(b.N) {
						return
					}
				}
			}
		}()
	}

	b.ReportAllocs()
	b.ResetTimer()

	// Producers
	for i := 0; i < numProducers; i++ {
		wg.Add(1)
		go func(producerID int) {
			defer wg.Done()
			itemsPerProducer := b.N / numProducers
			for j := 0; j < itemsPerProducer; j++ {
				val := producerID*itemsPerProducer + j
				for {
					select {
					case ch <- val:
						goto next
					default:
						runtime.Gosched()
					}
				}
			next:
			}
		}(i)
	}

	wg.Wait()
}
