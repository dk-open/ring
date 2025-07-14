package pad

import "sync/atomic"

// AtomicBool is an atomic boolean that is padded
type AtomicBool struct {
	atomic.Bool
	_ [60]byte
}

type AtomicInt32 struct {
	atomic.Int32
	_ [60]byte
}

type AtomicInt64 struct {
	atomic.Int64
	_ [56]byte
}

type AtomicUint32 struct {
	atomic.Uint32
	_ [60]byte
}

type AtomicUint64 struct {
	atomic.Uint64
	_ [56]byte
}
