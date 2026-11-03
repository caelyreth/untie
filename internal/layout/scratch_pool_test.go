package layout

import (
	"bytes"
	"context"
	"errors"
	"reflect"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/caelyreth/untie/display"
	"github.com/caelyreth/untie/geometry"
	"github.com/caelyreth/untie/ir"
	"github.com/caelyreth/untie/source"
)

func TestFragmentArenaRecycleClearsReferencesAndReusesBlocks(t *testing.T) {
	t.Parallel()

	var arena fragmentArena
	fragment := arena.allocate()
	probeKern := &MathKernInfo{TopLeft: &MathKern{
		Heights: []geometry.Em{1},
		Kerns:   []geometry.Em{2, 3},
	}}
	probeGroup := emptyFragment(Text, geometry.Em(geometry.EmScale))
	fragment.Metrics = Metrics{Width: 1, Ascent: 2, Descent: 3}
	fragment.kerns = probeKern
	fragment.moreDraws = []placedDraw{{kind: drawGroup, group: probeGroup}}
	fragment.moreHits = []sourceHit{{source: source.NewRange(1, 2)}}
	fragment.breaks = []fragmentBreak{{kind: display.BreakExplicit}}
	chunk := &arena.chunks[0][0]

	arena.recycle(1024)
	if arena.chunk != 0 || arena.used != 0 {
		t.Fatalf("arena position after recycle = chunk %d used %d", arena.chunk, arena.used)
	}
	assertArenaCleared(t, arena.chunks)
	reused := arena.allocate()
	if reused != chunk {
		t.Fatal("fragment arena did not reuse its cleared first block")
	}
}

func TestGlyphArenaRecycleClearsReferencesAndReusesBlocks(t *testing.T) {
	t.Parallel()

	var arena glyphArena
	fragment := arena.allocate()
	fragment.Scale = geometry.Em(geometry.EmScale)
	fragment.Glyph = Glyph{
		ID:       7,
		FontID:   "scratch-face",
		Text:     "x",
		Kerns:    MathKernInfo{TopLeft: &MathKern{Heights: []geometry.Em{1}, Kerns: []geometry.Em{2, 3}}},
		Outline:  Path{{Op: geometry.SegMoveTo, Points: [3]Point{{X: 1, Y: 2}}}},
		Parts:    []PositionedGlyph{{At: Point{X: 3, Y: 4}}},
		TextLike: true,
	}
	chunk := &arena.chunks[0][0]

	arena.recycle(1024)
	if arena.chunk != 0 || arena.used != 0 {
		t.Fatalf("arena position after recycle = chunk %d used %d", arena.chunk, arena.used)
	}
	assertArenaCleared(t, arena.chunks)
	reused := arena.allocate()
	if reused != chunk {
		t.Fatal("glyph arena did not reuse its cleared first block")
	}
}

func TestArenaRecycleBoundsRetainedStorage(t *testing.T) {
	t.Parallel()

	var fragments fragmentArena
	for range 200 {
		fragments.allocate()
	}
	fragments.recycle(10)
	if got := arenaSlots(fragments.chunks); got > 10 {
		t.Fatalf("fragment arena retained %d slots, want at most 10", got)
	}
	assertArenaCleared(t, fragments.chunks)

	var glyphs glyphArena
	for range 200 {
		glyphs.allocate()
	}
	glyphs.recycle(33)
	if got := arenaSlots(glyphs.chunks); got > 33 {
		t.Fatalf("glyph arena retained %d slots, want at most 33", got)
	}
	assertArenaCleared(t, glyphs.chunks)
}

func TestFlattenPathCacheRecycleClearsIdentity(t *testing.T) {
	t.Parallel()

	cache := flattenPathCache{
		box:    &display.Box{},
		inline: 2,
	}
	cache.inlineKeys[0] = pathTemplateKey{
		first:  &Segment{},
		length: 1,
		fontID: "scratch-face",
	}
	cache.inlineKeys[1] = pathTemplateKey{glyphID: 9, exact: true}
	cache.inlineIDs[0] = 4
	cache.inlineIDs[1] = 7
	cache.overflow = map[pathTemplateKey]int{{glyphID: 11}: 2}

	cache.recycle(maxRetainedPathEntries)
	if cache.box != nil || cache.inline != 0 {
		t.Fatalf("path cache identity after recycle = box %p inline %d", cache.box, cache.inline)
	}
	if cache.inlineKeys != [inlinePathCacheSize]pathTemplateKey{} {
		t.Fatalf("inline keys after recycle = %+v", cache.inlineKeys)
	}
	if cache.inlineIDs != [inlinePathCacheSize]int{} {
		t.Fatalf("inline IDs after recycle = %+v", cache.inlineIDs)
	}
	if len(cache.overflow) != 0 {
		t.Fatalf("overflow entries after recycle = %d", len(cache.overflow))
	}

	cache.overflow = make(map[pathTemplateKey]int, maxRetainedPathEntries+1)
	for i := 0; i <= maxRetainedPathEntries; i++ {
		cache.overflow[pathTemplateKey{glyphID: uint16(i + 1), exact: true}] = i
	}
	cache.recycle(maxRetainedPathEntries)
	if cache.overflow != nil {
		t.Fatal("oversized overflow cache was retained")
	}
}

func TestLayoutScratchPoolBoundsAndClearsReuse(t *testing.T) {
	t.Parallel()

	pool := newLayoutScratchPool(1)
	first := pool.acquire()
	poisonScratch(first)
	pool.release(first)

	reused := pool.acquire()
	if reused != first {
		t.Fatal("bounded scratch pool did not reuse its idle scratch")
	}
	assertScratchCleared(t, reused)
	pool.release(reused)

	held := pool.acquire()
	pool.release(new(layoutScratch))
	pool.mu.Lock()
	idle := len(pool.idle)
	pool.mu.Unlock()
	if idle != 1 {
		t.Fatalf("idle scratch count = %d, want hard cap 1", idle)
	}
	pool.release(held)
}

func TestLayoutScratchReleasesReferencesToCollectedObjects(t *testing.T) {
	var fragments fragmentArena
	var glyphs glyphArena
	var cache flattenPathCache
	fragmentFinalized := make(chan struct{})
	glyphFinalized := make(chan struct{})
	boxFinalized := make(chan struct{})

	poisonFragmentForFinalizer(&fragments, fragmentFinalized)
	poisonGlyphForFinalizer(&glyphs, glyphFinalized)
	poisonPathCacheForFinalizer(&cache, boxFinalized)

	fragments.recycle(maxRetainedFragmentSlots)
	glyphs.recycle(maxRetainedGlyphSlots)
	cache.recycle(maxRetainedPathEntries)

	waitForFinalizers(t, fragmentFinalized, glyphFinalized, boxFinalized)
}

func TestLayoutScratchPoolErrorAndPanicCleanup(t *testing.T) {
	t.Run("cancellation", func(t *testing.T) {
		pool := newLayoutScratchPool(1)
		ctx, cancel := context.WithCancel(context.Background())
		source := &scratchHookGlyphSource{
			base: &fakeGlyphSource{glyphs: map[string]Glyph{}},
			after: func(GlyphRequest) {
				cancel()
			},
		}
		renderer, err := New(Fonts{Glyphs: source, Metrics: fakeStyleFont{}}, Options{})
		if err != nil {
			t.Fatal(err)
		}
		renderer.scratchPool = pool
		root := ir.NewRow(sourceRange(0, 2), glyph("x", ir.ClassNormal), glyph("y", ir.ClassNormal))

		_, err = renderer.LayoutContext(ctx, root, ir.Inline, testStyle(16))
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("LayoutContext() error = %v, want context.Canceled", err)
		}
		assertPooledScratchCleared(t, pool)
	})

	t.Run("panic", func(t *testing.T) {
		pool := newLayoutScratchPool(1)
		source := &scratchHookGlyphSource{
			base: &fakeGlyphSource{glyphs: map[string]Glyph{}},
			after: func(req GlyphRequest) {
				if req.Text == "y" {
					panic("scratch panic")
				}
			},
		}
		renderer, err := New(Fonts{Glyphs: source, Metrics: fakeStyleFont{}}, Options{})
		if err != nil {
			t.Fatal(err)
		}
		renderer.scratchPool = pool
		root := ir.NewRow(sourceRange(0, 2), glyph("x", ir.ClassNormal), glyph("y", ir.ClassNormal))

		var recovered any
		func() {
			defer func() {
				recovered = recover()
			}()
			_, _ = renderer.Layout(root, ir.Inline, testStyle(16))
		}()
		if recovered != "scratch panic" {
			t.Fatalf("recovered panic = %v", recovered)
		}
		assertPooledScratchCleared(t, pool)
	})
}

func TestLayoutScratchDoesNotShareReturnedBoxes(t *testing.T) {
	pool := newLayoutScratchPool(4)
	renderer, glyphs, _ := newTestRenderer(t)
	renderer.scratchPool = pool
	glyphs.glyphs["x"] = Glyph{
		ID:            120,
		FontID:        "test-math",
		Advance:       em(1, 2),
		Ascent:        em(7, 10),
		Descent:       em(2, 10),
		ExactGlyphRef: true,
		Outline: Path{
			{Op: geometry.SegMoveTo, Points: [3]Point{{X: 0, Y: 0}}},
			{Op: geometry.SegLineTo, Points: [3]Point{{X: em(1, 2), Y: 0}}},
		},
	}
	root := glyph("x", ir.ClassNormal)
	first, err := renderer.Layout(root, ir.Inline, testStyle(16))
	if err != nil {
		t.Fatal(err)
	}
	firstSnapshot, err := first.MarshalCanonical()
	if err != nil {
		t.Fatal(err)
	}
	second, err := renderer.Layout(root, ir.Inline, testStyle(16))
	if err != nil {
		t.Fatal(err)
	}
	afterSecond, err := first.MarshalCanonical()
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(afterSecond, firstSnapshot) {
		t.Fatal("second layout changed the first returned box")
	}
	if len(second.Paths) != 1 || second.Items[0].PathIndex != 0 {
		t.Fatalf("second layout paths/items = %d/%+v", len(second.Paths), second.Items)
	}
	second.Paths[0].Segments[0].Points[0].X += 1
	second.Items[0].Bound.X += 1
	afterMutation, err := first.MarshalCanonical()
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(afterMutation, firstSnapshot) {
		t.Fatal("returned boxes share mutable path or item geometry")
	}
}

func TestLayoutScratchConcurrentReuse(t *testing.T) {
	pool := newLayoutScratchPool(4)
	renderer, glyphs, _ := newTestRenderer(t)
	renderer.scratchPool = pool
	glyphs.glyphs["x"] = Glyph{
		ID:            120,
		FontID:        "test-math",
		Advance:       em(1, 2),
		Ascent:        em(7, 10),
		Descent:       em(2, 10),
		ExactGlyphRef: true,
		Outline: Path{
			{Op: geometry.SegMoveTo, Points: [3]Point{{X: 0, Y: 0}}},
			{Op: geometry.SegLineTo, Points: [3]Point{{X: em(1, 2), Y: 0}}},
		},
	}
	root := glyph("x", ir.ClassNormal)
	want, err := renderer.Layout(root, ir.Inline, testStyle(16))
	if err != nil {
		t.Fatal(err)
	}

	const (
		workers = 16
		rounds  = 32
	)
	results := make([]*display.Box, workers*rounds)
	var wg sync.WaitGroup
	for worker := 0; worker < workers; worker++ {
		wg.Add(1)
		go func(worker int) {
			defer wg.Done()
			for round := 0; round < rounds; round++ {
				box, layoutErr := renderer.Layout(root, ir.Inline, testStyle(16))
				if layoutErr != nil {
					t.Errorf("worker %d layout error = %v", worker, layoutErr)
					return
				}
				results[worker*rounds+round] = box
			}
		}(worker)
	}
	wg.Wait()
	for i, box := range results {
		if !reflect.DeepEqual(box, want) {
			t.Fatalf("concurrent box %d differs from baseline", i)
		}
	}
}

func assertArenaCleared[T any](t *testing.T, chunks [][]T) {
	t.Helper()
	for chunkIndex, chunk := range chunks {
		for slotIndex := range chunk {
			if !reflect.ValueOf(chunk[slotIndex]).IsZero() {
				t.Fatalf("arena slot %d/%d retained a value: %+v", chunkIndex, slotIndex, chunk[slotIndex])
			}
		}
	}
}

func arenaSlots[T any](chunks [][]T) int {
	total := 0
	for _, chunk := range chunks {
		total += len(chunk)
	}
	return total
}

func poisonScratch(scratch *layoutScratch) {
	fragment := scratch.fragments.allocate()
	fragment.kerns = &MathKernInfo{TopLeft: &MathKern{
		Heights: []geometry.Em{1},
		Kerns:   []geometry.Em{2, 3},
	}}
	glyph := scratch.glyphs.allocate()
	glyph.Glyph = Glyph{
		ID:      1,
		FontID:  "scratch-face",
		Text:    "x",
		Outline: Path{{Op: geometry.SegMoveTo, Points: [3]Point{{X: 1, Y: 2}}}},
	}
	scratch.pathCache = flattenPathCache{
		box:      &display.Box{},
		inline:   1,
		overflow: map[pathTemplateKey]int{{glyphID: 1}: 0},
	}
	scratch.pathCache.inlineKeys[0] = pathTemplateKey{fontID: "scratch-face", glyphID: 1}
	scratch.pathCache.inlineIDs[0] = 0
}

func assertScratchCleared(t *testing.T, scratch *layoutScratch) {
	t.Helper()
	assertArenaCleared(t, scratch.fragments.chunks)
	assertArenaCleared(t, scratch.glyphs.chunks)
	if scratch.fragments.chunk != 0 || scratch.fragments.used != 0 ||
		scratch.glyphs.chunk != 0 || scratch.glyphs.used != 0 {
		t.Fatal("scratch arena cursor was not reset")
	}
	if scratch.pathCache.box != nil || scratch.pathCache.inline != 0 ||
		scratch.pathCache.inlineKeys != [inlinePathCacheSize]pathTemplateKey{} ||
		scratch.pathCache.inlineIDs != [inlinePathCacheSize]int{} {
		t.Fatal("scratch path cache retained identity")
	}
	if len(scratch.pathCache.overflow) != 0 {
		t.Fatalf("scratch path cache overflow entries = %d", len(scratch.pathCache.overflow))
	}
}

func assertPooledScratchCleared(t *testing.T, pool *layoutScratchPool) {
	t.Helper()
	pool.mu.Lock()
	idle := len(pool.idle)
	pool.mu.Unlock()
	if idle != 1 {
		t.Fatalf("idle scratch after layout cleanup = %d, want 1", idle)
	}
	scratch := pool.acquire()
	assertScratchCleared(t, scratch)
	pool.release(scratch)
}

func poisonFragmentForFinalizer(arena *fragmentArena, finalized chan<- struct{}) {
	fragment := arena.allocate()
	probe := &Fragment{}
	runtime.SetFinalizer(probe, func(*Fragment) {
		close(finalized)
	})
	fragment.moreDraws = []placedDraw{{kind: drawGroup, group: probe}}
}

func poisonGlyphForFinalizer(arena *glyphArena, finalized chan<- struct{}) {
	fragment := arena.allocate()
	probe := &MathKern{}
	runtime.SetFinalizer(probe, func(*MathKern) {
		close(finalized)
	})
	fragment.Glyph.Kerns.TopLeft = probe
}

func poisonPathCacheForFinalizer(cache *flattenPathCache, finalized chan<- struct{}) {
	probe := &display.Box{}
	runtime.SetFinalizer(probe, func(*display.Box) {
		close(finalized)
	})
	cache.box = probe
}

func waitForFinalizers(t *testing.T, channels ...<-chan struct{}) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for remaining := len(channels); remaining > 0; {
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for %d scratch references to be collected", remaining)
		}
		runtime.GC()
		progress := false
		for i := 0; i < len(channels); i++ {
			select {
			case <-channels[i]:
				channels = append(channels[:i], channels[i+1:]...)
				remaining--
				progress = true
				i--
			default:
			}
		}
		if !progress {
			runtime.Gosched()
			time.Sleep(time.Millisecond)
		}
	}
}

type scratchHookGlyphSource struct {
	base  GlyphSource
	after func(GlyphRequest)
}

func (s *scratchHookGlyphSource) Glyph(req GlyphRequest) (Glyph, error) {
	glyph, err := s.base.Glyph(req)
	if s.after != nil {
		s.after(req)
	}
	return glyph, err
}

func sourceRange(start, end int) source.Range {
	return source.NewRange(start, end)
}
