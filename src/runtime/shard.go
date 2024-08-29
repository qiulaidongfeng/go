package runtime

import (
	"internal/runtime/atomic"
	"unsafe"
)

type ShardedValue struct {
	// NewShard is a function that produces new shards of type T.
	NewShard func()

	id uintptr
}

func findShrad(p *p, s uintptr) (shard unsafe.Pointer, index int) {
	for i := range p.shardp {
		if p.shardp[i].pool == s {
			return p.shardp[i].shard, i
		}
	}
	return nil, -1
}

var genShardId uintptr

func updateShard(v unsafe.Pointer, f func(unsafe.Pointer) unsafe.Pointer) {
	s := (*ShardedValue)(v)
	id := atomic.Loaduintptr(&s.id)
	if id == 0 {
		atomic.Casuintptr(&s.id, 0, atomic.Xadduintptr(&genShardId, 1))
		id = atomic.Loaduintptr(&s.id)
	}
	p := getg().m.p.ptr()
	shard, index := findShrad(p, id)
	if index == -1 {
		p.shardp = append(p.shardp, struct {
			shard unsafe.Pointer
			pool  uintptr
		}{
			pool:  id,
			shard: f(nil),
		})
		return
	}
	p.shardp[index].shard = f(shard)
	KeepAlive(s)
}

func valueShard(v unsafe.Pointer, yield func(unsafe.Pointer), Getret func() unsafe.Pointer) {
	s := (*ShardedValue)(v)
	stw := stopTheWorld(stwShardRead)
	once := false
	for i := range allp {
		v, index := findShrad(allp[i], s.id)
		if v != nil {
			yield(v)
		}
		if index != -1 {
			if once {
				allp[i].shardp[index].shard = nil
			} else {
				allp[i].shardp[index].shard = Getret()
				once = true
			}
		}
	}
	startTheWorld(stw)
}

func drainShard(v unsafe.Pointer, yield func(unsafe.Pointer)) {
	s := (*ShardedValue)(v)
	stw := stopTheWorld(stwShardRead)
	for i := range allp {
		v, _ := findShrad(allp[i], s.id)
		if v != nil {
			yield(v)
		}
	}
	startTheWorld(stw)
}
