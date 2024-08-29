package sync

import (
	"unsafe"
)

// ShardedValue is a pool of values that all represent a small piece of a single
// conceptual value. These values must implement Shard on themselves.
//
// The purpose of a ShardedValue is to enable the creation of scalable data structures
// that may be updated in shards that are local to the goroutine without any
// additional synchronization. In practice, shards are bound to lower-level
// scheduling resources, such as OS threads and CPUs, for efficiency.
//
// The zero value is ready for use.
type ShardedValue[T Shard[T]] struct {
	// NewShard is a function that produces new shards of type T.
	NewShard func() T

	id uintptr
}

// Update acquires a shard and passes it to the provided function.
// The function returns the new shard value to return to the pool.
// Update is safe to call from multiple goroutines.
// Callers are encouraged to keep the provided function as short
// as possible, and are discouraged from blocking within them.
func (s *ShardedValue[T]) Update(f func(value T) T) {
	updateShard(unsafe.Pointer(s), func(p unsafe.Pointer) unsafe.Pointer {
		if p == nil {
			t := s.NewShard()
			t = f(t)
			return unsafe.Pointer(&t)
		}
		t := (*T)(p)
		*t = f(*t)
		return p
	})
}

// Value snapshots all values in the pool and returns the result of merging them all into
// a single value. This single value is guaranteed to represent a consistent snapshot of
// merging all outstanding shards at some point in time between when the call is made
// and when it returns. This single value is immediately added back into the pool as a
// single shard before being returned.
func (s *ShardedValue[T]) Value() (ret T) {
	valueShard(unsafe.Pointer(s), func(p unsafe.Pointer) {
		t := (*T)(p)
		ret = ret.Merge(*t)
	}, func() unsafe.Pointer { return unsafe.Pointer(&ret) })
	return
}

// Drain snapshots all values in the pool and returns the result of merging them all into
// a single value. This single value is guaranteed to represent a consistent snapshot of
// merging all outstanding shards at some point in time between when the call is made
// and when it returns. Unlike Value, this single value is not added back to the pool.
func (s *ShardedValue[T]) Drain() (ret T) {
	drainShard(unsafe.Pointer(s), func(p unsafe.Pointer) {
		t := (*T)(p)
		ret = ret.Merge(*t)
	})
	return
}

// Shard is an interface implemented by types whose values can be merged
// with values of the same type.
type Shard[T any] interface {
	Merge(other T) T
}

//go:linkname updateShard runtime.updateShard
func updateShard(s unsafe.Pointer, f func(unsafe.Pointer) unsafe.Pointer)

//go:linkname valueShard runtime.valueShard
func valueShard(v unsafe.Pointer, yield func(unsafe.Pointer), Getret func() unsafe.Pointer)

//go:linkname drainShard runtime.drainShard
func drainShard(v unsafe.Pointer, yield func(unsafe.Pointer))
