package runtime

import (
	"internal/abi"
	"unsafe"
)

type bumpPtr struct {
	typs map[*abi.Type]*bumpPtrMemPool
}

// getMemPool get [*bumpPtrMemPool] of rtype
func (b *bumpPtr) getMemPool(rtype *abi.Type, bufsize ...uintptr) *bumpPtrMemPool {
	if b.typs == nil {
		b.typs = make(map[*abi.Type]*bumpPtrMemPool)
	}
	// Beware of ideas here becoming reality in the future
	// https://github.com/golang/go/issues/62483#issuecomment-1800913220
	// Can we put all the runtime._types in one of these weak maps?
	// So when no more pointers to that runtime._type exist, the entry itself can be garbage collected.
	// Happen *rtype of the same type is different
	mempool := b.typs[rtype]
	if mempool == nil {
		mempool = newbumpPtrMemPool(rtype, bufsize...)
		b.typs[rtype] = mempool
	}
	return mempool
}

// Alloc from [*bumpPtr] make a go value , and return ptr
//
//   - bufsize is optional.
//
// It determines the Sizeof memory allocated by [*bumpPtrMemPool.Alloc] for automatic expansion.
// The default value is 8Mb and the value must be greater than rtype.Size()
func (b *bumpPtr) Alloc(size uintptr, rtype *abi.Type, bufsize ...uintptr) unsafe.Pointer {
	mempool := b.getMemPool(rtype, bufsize...)
	valueSize := rtype.Size()
	if size == valueSize {
		return mempool.Alloc()
	}
	num := size % valueSize
	if num*valueSize != size {
		throw("size error")
	}
	return mempool.AllocNum(num)
}

// bumpPtrbuf holds a continuous set of go values and works with [bumpPtrMemPool] to implement memory pool
type bumpPtrbuf struct {
	buf      unsafe.Pointer
	index    uintptr
	dataSize uintptr
	maxIndex uintptr
}

var bumpPtrMem struct {
	f fixalloc
	l mutex
}

func init() {
	lock(&bumpPtrMem.l)
	bumpPtrMem.f.init(defaultmem, nil, unsafe.Pointer(nil), &memstats.mspan_sys)
	unlock(&bumpPtrMem.l)
}

func newbumpPtrbuf(bufsize uintptr, rtype *abi.Type) *bumpPtrbuf {
	var p unsafe.Pointer
	systemstack(func() {
		lock(&bumpPtrMem.l)
		p = bumpPtrMem.f.alloc()
		unlock(&bumpPtrMem.l)
	})
	// TODO: support bufsize!=defaultmem
	return &bumpPtrbuf{
		buf:      p,
		index:    0,
		dataSize: rtype.Size(),
		maxIndex: defaultmem / rtype.Size(),
	}
}

// move Allocates a Go value of a fixed size, and automatically expands when the capacity is insufficient
func (b *bumpPtrbuf) move(a *bumpPtrMemPool, allocNum uintptr) unsafe.Pointer {
	old := b.index
	b.index += allocNum
	if b.index >= b.maxIndex {
		a.reAlloc()
		return a.Alloc()
	}
	return unsafe.Add(b.buf, old*b.dataSize)
}

type bumpPtrMemPool struct {
	bufs  []*bumpPtrbuf
	rtype *abi.Type
	// bufsize Indicates the memory size allocated for automatic expansion. The default value is 8Mb
	bufsize uintptr
}

const defaultmem = 8 * 1024 * 1024 //8Mib

// newbumpPtrMemPool Create a set of memory pools representing Go values that are allocated and released together
//
//   - bufsize is optional.
//
// It determines the Sizeof memory allocated by [*bumpPtrMemPool.Alloc] for automatic expansion.
// The default value is 8Mb and the value must be greater than rtype.Size()
func newbumpPtrMemPool(rtype *abi.Type, bufsize ...uintptr) *bumpPtrMemPool {
	ret := &bumpPtrMemPool{rtype: rtype}
	var nbuf *bumpPtrbuf
	if len(bufsize) == 0 {
		ret.bufsize = defaultmem
		nbuf = newbumpPtrbuf(defaultmem, rtype)
	} else {
		ret.bufsize = bufsize[0]
		nbuf = newbumpPtrbuf(bufsize[0], rtype)
	}
	bufs := make([]*bumpPtrbuf, 1)
	bufs[0] = nbuf
	ret.bufs = bufs
	return ret
}

// reAlloc perform expansion
func (b *bumpPtrMemPool) reAlloc() {
	nbuf := newbumpPtrbuf(b.bufsize, b.rtype)
	b.bufs = append([]*bumpPtrbuf{nbuf}, b.bufs...)
}

// Alloc make a Go value and returns a pointer
func (b *bumpPtrMemPool) Alloc() unsafe.Pointer {
	buf := b.bufs[0]
	return buf.move(b, 1)
}

// AllocNum Create a set of consecutive Go values and return a pointer
func (b *bumpPtrMemPool) AllocNum(num uintptr) unsafe.Pointer {
	buf := b.bufs[0]
	return buf.move(b, num)
}
