package sync_test

import (
	. "sync"
	"sync/atomic"
	"testing"
)

func TestShardCounter(t *testing.T) {
	var s Counter
	s.sp.NewShard = func() counterInt {
		return counterInt(0)
	}
	s.Add(1)
	if v := s.Value(); v != 1 {
		t.Fatalf("got %d , want %d", v, 1)
	}
	var wg WaitGroup
	for range 100 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			s.Add(1)
		}()
	}
	wg.Wait()
	if v := s.Value(); v != 101 {
		t.Fatalf("got %d , want %d", v, 101)
	}
}

type Counter struct {
	sp ShardedValue[counterInt]
}

func (c *Counter) Add(value int) {
	c.sp.Update(func(v counterInt) counterInt {
		return counterInt(int(v) + value)
	})
}

func (c *Counter) Value() int {
	return int(c.sp.Value())
}

type counterInt int

func (a counterInt) Merge(b counterInt) counterInt {
	return a + b
}

func TestShardDrain(t *testing.T) {
	var s Counter
	s.sp.NewShard = func() counterInt {
		return counterInt(0)
	}
	s.Add(1)
	if v := s.sp.Drain(); v != 1 {
		t.Fatalf("got %d , want %d", v, 1)
	}
	var wg WaitGroup
	for range 100 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			s.Add(1)
		}()
	}
	wg.Wait()
	if v := s.sp.Drain(); v != 101 {
		t.Fatalf("got %d , want %d", v, 101)
	}
}

func BenchmarkCounter(b *testing.B) {
	b.Run("atomic/int64", func(b *testing.B) {
		i := int64(0)
		b.RunParallel(func(p *testing.PB) {
			for p.Next() {
				for range 100000 {
					atomic.AddInt64(&i, 1)
				}

			}
		})
	})
	b.Run("sync/int64", func(b *testing.B) {
		b.RunParallel(func(p *testing.PB) {
			c := Counter{}
			c.sp.NewShard = func() counterInt {
				return counterInt(0)
			}
			for p.Next() {
				for range 100000 {
					c.Add(1)
				}
			}
		})
	})
}
