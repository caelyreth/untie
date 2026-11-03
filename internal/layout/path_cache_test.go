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
		})
		index, err := state.pathTemplate(key, cacheable, path)
		if err != nil {
			t.Fatalf("pathTemplate(%d) error = %v", i, err)
		}
		indexes = append(indexes, index)
	}
	if state.inlinePaths != inlinePathCacheSize {
		t.Fatalf("inline cache entries = %d, want %d", state.inlinePaths, inlinePathCacheSize)
	}
	if state.pathCache == nil {
		t.Fatal("overflow cache is nil")
	}

	key, cacheable := glyphPathTemplateKey(Glyph{
		ID:            1,
		FontID:        "test-face",
		ExactGlyphRef: true,
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

func newPathCacheTestState() *flattenState {
	return &flattenState{
		layout: &layoutState{ctx: context.Background()},
		box:    &display.Box{},
	}
}
