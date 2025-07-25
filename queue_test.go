package ring

import (
	"fmt"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// Unit Tests
func TestQueue_BasicOperations(t *testing.T) {
	q, err := Queue[int](8)
	if err != nil {
		t.Fatalf("Failed to create queue: %v", err)
	}

	var v *int
	// Test empty queue
	if _, ok := q.Dequeue(); ok {
		t.Errorf("Expected empty queue, got item: %v", v)
	}

	// Test enqueue
	err = q.MustEnqueue(42)
	if err != nil {
		t.Error("Failed to enqueue item", err)
	}

	// Test dequeue
	if item, ok := q.Dequeue(); ok {
		if item != 42 {
			t.Errorf("Expected 42, got %d", item)
		}
	} else {
		t.Error("Failed to dequeue item")
	}

	// Test empty queue after dequeue
	if _, ok := q.Dequeue(); ok {
		t.Errorf("Expected empty queue after dequeue, got item: %v", v)
	}
}

func TestQueue_CapacityLimits(t *testing.T) {
	capacity := uint64(4)
	q, err := Queue[string](capacity)
	if err != nil {
		t.Fatalf("Failed to create queue: %v", err)
	}
	// Fill the queue to capacity
	for i := 0; i < int(capacity); i++ {
		v := "test"
		success := q.Enqueue(v)
		if !success {
			t.Errorf("Failed to enqueue item %d", i)
		}
	}
	v := "overflow"
	// Try to enqueue one more item - should fail
	success := q.Enqueue(v)
	if success {
		t.Error("Expected enqueue to fail when queue is full")
	}

	// Dequeue one item
	if _, ok := q.Dequeue(); !ok {
		t.Error("Failed to dequeue item from full queue")
	}
	item := "new"
	// Now enqueue should succeed
	success = q.Enqueue(item)
	if !success {
		t.Error("Failed to enqueue after dequeue")
	}
}

func TestQueue_FIFO(t *testing.T) {
	q, err := Queue[int](16)
	if err != nil {
		t.Fatalf("Failed to create queue: %v", err)
	}
	// Enqueue items in order
	for i := 0; i < 5; i++ {
		val := i
		success := q.Enqueue(val)
		if !success {
			t.Errorf("Failed to enqueue item %d", i)
		}
	}

	// Dequeue items and verify order
	for i := 0; i < 5; i++ {
		if item, ok := q.Dequeue(); ok {
			if item != i {
				t.Errorf("Expected %d, got %d", i, item)
			}
		} else {
			t.Errorf("Failed to dequeue item %d", i)
		}
	}
}

func TestQueue_SingleProducerMultipleConsumers(t *testing.T) {
	q, err := Queue[int](1024)
	if err != nil {
		t.Fatalf("Failed to create queue: %v", err)
	}
	const numItems = 1000
	const numConsumers = 2
	time.Sleep(1 * time.Second)
	// Producer goroutine
	go func() {
		for i := 0; i < numItems; i++ {
			val := i + 1
			//fmt.Println("Enqueuing item:", val)
			if err = q.MustEnqueue(val); err != nil {
				fmt.Printf("Producer failed to enqueue item %d: %v\n", val, err)
				continue
			}
			runtime.Gosched()
		}
	}()

	time.Sleep(50 * time.Microsecond)
	// Consumer goroutines
	var wg sync.WaitGroup
	var consumed int64
	results := make([][]int, numConsumers)

	for c := 0; c < numConsumers; c++ {
		wg.Add(1)
		go func(consumerID int) {
			defer wg.Done()
			var localResults []int

			for {
				if item, ok := q.Dequeue(); ok {
					localResults = append(localResults, item)
					if atomic.AddInt64(&consumed, 1) >= numItems {
						break
					}
				}
				if atomic.LoadInt64(&consumed) >= numItems {
					break
				}
			}
			results[consumerID] = localResults
		}(c)
	}

	wg.Wait()

	// Verify all items were consumed
	totalConsumed := int(atomic.LoadInt64(&consumed))
	if totalConsumed != numItems {
		t.Errorf("Expected %d items consumed, got %d", numItems, totalConsumed)
	}

	// Verify no duplicates
	seen := make(map[int]bool)
	for _, consumerResults := range results {
		for _, val := range consumerResults {
			if seen[val] {
				t.Errorf("Duplicate value found: %d", val)
			}
			seen[val] = true
		}
	}
}

func TestQueue_BitMaskOperations(t *testing.T) {
	q, err := Queue[int](8)
	if err != nil {
		t.Fatalf("Failed to create queue: %v", err)
	}

	qInternal := q.(*queue[int])

	// Test that bit mask works correctly
	testCases := []struct {
		head     uint64
		expected uint64
	}{
		{0, 0},  // 0>>1 & 7 = 0
		{2, 1},  // 2>>1 & 7 = 1
		{4, 2},  // 4>>1 & 7 = 2
		{14, 7}, // 14>>1 & 7 = 7
		{16, 0}, // 16>>1 & 7 = 0 (wraps around)
		{18, 1}, // 18>>1 & 7 = 1
	}

	for _, tc := range testCases {
		index := tc.head >> 1 & qInternal.capMask
		if index != tc.expected {
			t.Errorf("For head=%d, expected index=%d, got %d", tc.head, tc.expected, index)
		}
	}
}

// Test 1: Success path - MustEnqueue должен работать как обычный Enqueue в нормальных условиях
func TestMustEnqueue_SuccessPath(t *testing.T) {
	q, err := Queue[int](8)
	if err != nil {
		t.Fatalf("Failed to create queue: %v", err)
	}

	// Test successful enqueue to empty queue
	err = q.MustEnqueue(42)
	if err != nil {
		t.Errorf("MustEnqueue failed on empty queue: %v", err)
	}

	// Verify the item was enqueued correctly
	if result, success := q.Dequeue(); success {
		if result != 42 {
			t.Errorf("Expected 42, got %d", result)
		}
	} else {
		t.Error("Failed to dequeue after MustEnqueue")
	}

	// Test successful enqueue to partially filled queue
	for i := 0; i < 3; i++ {
		val := i
		err = q.MustEnqueue(val)
		if err != nil {
			t.Errorf("MustEnqueue failed on partially filled queue at item %d: %v", i, err)
		}
	}

	// Verify all items were enqueued correctly
	for i := 0; i < 3; i++ {
		if item, success := q.Dequeue(); success {
			if item != i {
				t.Errorf("Expected %d, got %d", i, item)
			}
		} else {
			t.Errorf("Failed to dequeue item %d", i)
		}
	}
}
