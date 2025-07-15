package pad

import (
	"fmt"
	"math/rand"
	"testing"
	"time"
)

func TestMinBarrier_MultipleBarriers(t *testing.T) {
	var a1, a2, a3 AtomicUint64
	a1.Store(42)
	a2.Store(17)
	a3.Store(19)
	barriers := MinBarrier{&a1, &a2, &a3}
	if got := barriers.Load(); got != 17 {
		t.Fatalf("expected 17, got %d", got)
	}
}

func TestMinBarrier_AllEqual(t *testing.T) {
	var a1, a2, a3 AtomicUint64
	a1.Store(7)
	a2.Store(7)
	a3.Store(7)
	barriers := MinBarrier{&a1, &a2, &a3}
	if got := barriers.Load(); got != 7 {
		t.Fatalf("expected 7, got %d", got)
	}
}

func TestMinBarrier_EmptyPanic(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic for empty MinBarrier, but got none")
		}
	}()
	var barriers MinBarrier
	_ = barriers.Load()
}

// Branchless
func branchlessMin(m MinBarrier) uint64 {
	minimum := m[0].Load()
	for i := 1; i < len(m); i++ {
		seq := m[i].Load()
		diff := minimum - seq
		mask := diff >> 63
		minimum = seq + (diff & mask)
	}
	return minimum
}

func genAtomicUInt64s(vals []uint64) MinBarrier {
	res := make(MinBarrier, len(vals))
	for i, v := range vals {
		a := &AtomicUint64{}
		a.Store(v)
		res[i] = a
	}
	return res
}

func BenchmarkMinVariants(b *testing.B) {
	rnd := rand.New(rand.NewSource(time.Now().UnixNano()))

	for n := 1; n <= 4; n++ {
		// Prepare data for this size
		vals := make([]uint64, n)
		for i := range vals {
			vals[i] = uint64(rnd.Int63n(1000) - 500) // диапазон -500..499
		}
		barriers := genAtomicUInt64s(vals)
		b.ReportAllocs()
		b.Run(
			// Example: "Branchless-4", "Branch-4"
			fmt.Sprintf("Branch-if-%d", n),
			func(b *testing.B) {
				var r uint64
				for i := 0; i < b.N; i++ {
					r = barriers.Load()
				}
				_ = r
			})
		b.Run(
			fmt.Sprintf("Branch-less-%d", n),
			func(b *testing.B) {
				var r uint64
				for i := 0; i < b.N; i++ {
					r = branchlessMin(barriers)
				}
				_ = r
			})
	}
}
