package reflex

import (
	"runtime"
	"sync/atomic"
	"time"
)

type GarbageCollector struct {
	shouldCheck atomic.Int64
	refCount    map[Node]uint64
	lastBytes   uint64
	shutdown    chan struct{}
}

func NewGarbageCollector() *GarbageCollector {
	var stats runtime.MemStats
	runtime.ReadMemStats(&stats)
	res := &GarbageCollector{
		refCount:  map[Node]uint64{},
		lastBytes: stats.Alloc,
		shutdown:  make(chan struct{}, 0),
	}
	res.shouldCheck.Store(0)
	go res.worker()
	return res
}

func (g *GarbageCollector) Retain(n Node) {
	x := g.refCount[n]
	g.refCount[n] = x + 1
}

func (g *GarbageCollector) Release(n Node) {
	x := g.refCount[n]
	if x <= 0 {
		panic("negative ref count")
	}
	if x == 1 {
		delete(g.refCount, n)
	} else {
		g.refCount[n] = x - 1
	}
}

func (g *GarbageCollector) MaybeCollect() {
	if !g.shouldCheck.CompareAndSwap(1, 0) {
		return
	}
	for n := range g.refCount {
		n.Flatten()
	}
	runtime.GC()
}

func (g *GarbageCollector) Shutdown() {
	close(g.shutdown)
}

func (g *GarbageCollector) worker() {
	for {
		select {
		case <-g.shutdown:
			return
		case <-time.After(time.Millisecond * 100):
			g.checkGarbage()
		}
	}
}

func (g *GarbageCollector) checkGarbage() {
	var stats runtime.MemStats
	runtime.ReadMemStats(&stats)
	if stats.Alloc > 2*g.lastBytes {
		g.lastBytes = stats.Alloc
		g.shouldCheck.Store(1)
	} else if stats.Alloc < g.lastBytes {
		g.lastBytes = stats.Alloc
	}
}
