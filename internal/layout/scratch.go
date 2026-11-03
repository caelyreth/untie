package layout

import "sync"

const (
	// Bound retained scratch independently of user-selected layout limits.
	maxRetainedFragmentSlots = 4096
	maxRetainedGlyphSlots    = 4096
	maxRetainedPathEntries   = 64
	maxIdleLayoutScratch     = 16
)

// layoutScratch contains only reusable layout-local storage. A layoutState is
// never pooled because it retains request-specific contexts, styles, and IR.
type layoutScratch struct {
	fragments fragmentArena
	glyphs    glyphArena
	pathCache flattenPathCache
}

// layoutScratchPool is a hard-bounded pool. sync.Pool was deliberately avoided
// because it has no stable upper bound between collections, while a layout
// scratch can retain megabytes after a hostile or unusually large formula.
type layoutScratchPool struct {
	mu       sync.Mutex
	idle     []*layoutScratch
	capacity int
}

func newLayoutScratchPool(capacity int) *layoutScratchPool {
	if capacity < 0 {
		capacity = 0
	}
	return &layoutScratchPool{
		idle:     make([]*layoutScratch, 0, capacity),
		capacity: capacity,
	}
}

func (p *layoutScratchPool) acquire() *layoutScratch {
	if p == nil {
		return new(layoutScratch)
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	last := len(p.idle) - 1
	if last < 0 {
		return new(layoutScratch)
	}
	scratch := p.idle[last]
	p.idle[last] = nil
	p.idle = p.idle[:last]
	return scratch
}

func (p *layoutScratchPool) release(scratch *layoutScratch) {
	if scratch == nil {
		return
	}
	scratch.fragments.recycle(maxRetainedFragmentSlots)
	scratch.glyphs.recycle(maxRetainedGlyphSlots)
	scratch.pathCache.recycle(maxRetainedPathEntries)
	if p == nil {
		return
	}
	p.mu.Lock()
	if len(p.idle) < p.capacity {
		p.idle = append(p.idle, scratch)
	}
	p.mu.Unlock()
}

func (a *fragmentArena) recycle(maxSlots int) {
	a.chunks = recycleArenaChunks(a.chunks, maxSlots)
	a.chunk = 0
	a.used = 0
}

func (a *glyphArena) recycle(maxSlots int) {
	a.chunks = recycleArenaChunks(a.chunks, maxSlots)
	a.chunk = 0
	a.used = 0
}

func recycleArenaChunks[T any](chunks [][]T, maxSlots int) [][]T {
	if maxSlots < 0 {
		maxSlots = 0
	}
	retainedSlots := 0
	keep := 0
	for i := range chunks {
		clear(chunks[i])
		if keep != i || len(chunks[i]) > maxSlots-retainedSlots {
			continue
		}
		retainedSlots += len(chunks[i])
		keep++
	}
	clear(chunks[keep:])
	return chunks[:keep]
}

var defaultLayoutScratchPool = newLayoutScratchPool(maxIdleLayoutScratch)
