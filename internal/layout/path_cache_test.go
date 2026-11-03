package layout

import (
	"context"
	"testing"

	"github.com/caelyreth/untie/display"
	"github.com/caelyreth/untie/geometry"
)

func TestFlattenPathCachePreservesPointerIdentity(t *testing.T) {
	t.Parallel()

	state := newPathCacheTestState()
	path := Path{{
		Op:     geometry.SegMoveTo,
		Points: [3]Point{{X: 1, Y: 2}},
	}}
	key, cacheable := pathTemplateKeyFor(path)
	first, err := state.pathTemplate(key, cacheable, path)
	if err != nil {
		t.Fatalf("pathTemplate(first) error = %v", err)
	}
	again, err := state.pathTemplate(key, cacheable, path)
	if err != nil {
		t.Fatalf("pathTemplate(again) error = %v", err)
	}
	if first != again {
		t.Fatalf("repeated path indexes = %d, %d; want same index", first, again)
	}

	copied := append(Path(nil), path...)
	copiedKey, cacheable := pathTemplateKeyFor(copied)
	distinct, err := state.pathTemplate(copiedKey, cacheable, copied)
	if err != nil {
		t.Fatalf("pathTemplate(copied) error = %v", err)
	}
	if distinct == first {
		t.Fatalf("path with distinct backing array reused index %d", first)
	}
}

func TestFlattenPathCacheFallsBackPastInlineCapacity(t *testing.T) {
	t.Parallel()

	state := newPathCacheTestState()
	path := Path{{
		Op:     geometry.SegMoveTo,
		Points: [3]Point{},
	}}
	indexes := make([]int, 0, inlinePathCacheSize+2)
	for i := 0; i < inlinePathCacheSize+2; i++ {
		key, cacheable := glyphPathTemplateKey(Glyph{
			ID:            uint16(i + 1),
			FontID:        "test-face",
			ExactGlyphRef: true,
			Outline:       path,
		})
		index, err := state.pathTemplate(key, cacheable, path)
		if err != nil {
			t.Fatalf("pathTemplate(%d) error = %v", i, err)
		}
		indexes = append(indexes, index)
	}
	if state.layout.pathCache.inline != inlinePathCacheSize {
		t.Fatalf("inline cache entries = %d, want %d", state.layout.pathCache.inline, inlinePathCacheSize)
	}
	if state.layout.pathCache.overflow == nil {
		t.Fatal("overflow cache is nil")
	}

	key, cacheable := glyphPathTemplateKey(Glyph{
		ID:            1,
		FontID:        "test-face",
		ExactGlyphRef: true,
		Outline:       path,
	})
	got, err := state.pathTemplate(key, cacheable, path)
	if err != nil {
		t.Fatalf("pathTemplate(inline hit) error = %v", err)
	}
	if got != indexes[0] {
		t.Fatalf("inline hit index = %d, want %d", got, indexes[0])
	}

	key, cacheable = glyphPathTemplateKey(Glyph{
		ID:            uint16(inlinePathCacheSize + 2),
		FontID:        "test-face",
		ExactGlyphRef: true,
		Outline:       path,
	})
	got, err = state.pathTemplate(key, cacheable, path)
	if err != nil {
		t.Fatalf("pathTemplate(overflow hit) error = %v", err)
	}
	if want := indexes[len(indexes)-1]; got != want {
		t.Fatalf("overflow hit index = %d, want %d", got, want)
	}
	if len(state.box.Paths) != len(indexes) {
		t.Fatalf("path templates = %d, want %d", len(state.box.Paths), len(indexes))
	}
}

func TestFlattenPathCacheSeparatesExactGlyphOutlineIdentity(t *testing.T) {
	t.Parallel()

	state := newPathCacheTestState()
	firstPath := Path{{
		Op:     geometry.SegMoveTo,
		Points: [3]Point{{X: em(1, 1), Y: em(2, 1)}},
	}}
	shiftedPath := Path{{
		Op:     geometry.SegMoveTo,
		Points: [3]Point{{X: em(1, 1), Y: em(3, 1)}},
	}}
	key, cacheable := glyphPathTemplateKey(Glyph{
		ID:            7,
		FontID:        "test-face",
		ExactGlyphRef: true,
		Outline:       firstPath,
	})
	first, err := state.pathTemplate(key, cacheable, firstPath)
	if err != nil {
		t.Fatalf("pathTemplate(first) error = %v", err)
	}
	key, cacheable = glyphPathTemplateKey(Glyph{
		ID:            7,
		FontID:        "test-face",
		ExactGlyphRef: true,
		Outline:       shiftedPath,
	})
	shifted, err := state.pathTemplate(key, cacheable, shiftedPath)
	if err != nil {
		t.Fatalf("pathTemplate(shifted) error = %v", err)
	}
	if shifted == first {
		t.Fatalf("different outlines with the same glyph reference reused index %d", first)
	}
	if got := state.box.Paths[shifted].Bound.Y; got != 3 {
		t.Fatalf("shifted template bound y = %v, want 3", got)
	}
}

func TestFlattenPathCacheIsSharedAcrossFlattenStates(t *testing.T) {
	t.Parallel()

	layout := &layoutState{ctx: context.Background()}
	box := &display.Box{}
	firstState := newFlattenState(layout, box, 0, 0)
	secondState := newFlattenState(layout, box, 8, 16)
	path := Path{{
		Op:     geometry.SegMoveTo,
		Points: [3]Point{{X: 1, Y: 2}},
	}}
	key, cacheable := pathTemplateKeyFor(path)
	first, err := firstState.pathTemplate(key, cacheable, path)
	if err != nil {
		t.Fatalf("pathTemplate(first) error = %v", err)
	}
	second, err := secondState.pathTemplate(key, cacheable, path)
	if err != nil {
		t.Fatalf("pathTemplate(second) error = %v", err)
	}
	if second != first {
		t.Fatalf("flatten states created duplicate templates: %d and %d", first, second)
	}
	if len(box.Paths) != 1 {
		t.Fatalf("path templates = %d, want 1", len(box.Paths))
	}
}

func TestFlattenPathCacheResetsForDifferentBox(t *testing.T) {
	t.Parallel()

	layout := &layoutState{ctx: context.Background()}
	firstBox := &display.Box{}
	secondBox := &display.Box{}
	path := Path{{
		Op:     geometry.SegMoveTo,
		Points: [3]Point{{X: 1, Y: 2}},
	}}
	key, cacheable := pathTemplateKeyFor(path)
	firstState := newFlattenState(layout, firstBox, 0, 0)
	if _, err := firstState.pathTemplate(key, cacheable, path); err != nil {
		t.Fatalf("pathTemplate(first box) error = %v", err)
	}
	secondState := newFlattenState(layout, secondBox, 0, 0)
	if _, err := secondState.pathTemplate(key, cacheable, path); err != nil {
		t.Fatalf("pathTemplate(second box) error = %v", err)
	}
	if len(secondBox.Paths) != 1 {
		t.Fatalf("second box path templates = %d, want 1", len(secondBox.Paths))
	}
	if layout.pathCache.box != secondBox {
		t.Fatal("path cache remained bound to the first box")
	}
}

func newPathCacheTestState() *flattenState {
	return &flattenState{
		layout: &layoutState{ctx: context.Background()},
		box:    &display.Box{},
	}
}
