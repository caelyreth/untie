package layout

import (
	"context"
	"math/bits"
	"testing"

	"github.com/caelyreth/untie/geometry"
	"github.com/caelyreth/untie/ir"
	"github.com/caelyreth/untie/source"
)

func TestFragmentInlineDrawAndHitStorage(t *testing.T) {
	t.Parallel()

	fragment := emptyFragment(Text, geometry.Em(geometry.EmScale))
	if fragment.drawCount != 0 || fragment.hitCount != 0 || fragment.ownHit != -1 {
		t.Fatalf("empty fragment counters = draws:%d hits:%d own:%d", fragment.drawCount, fragment.hitCount, fragment.ownHit)
	}

	firstDraw := placedDraw{x: 1, y: 2, kind: drawRule}
	secondDraw := placedDraw{x: 3, y: 4, kind: drawPath}
	fragment.appendDraw(firstDraw)
	fragment.appendDraw(secondDraw)
	if fragment.drawCount != 2 {
		t.Fatalf("drawCount = %d, want 2", fragment.drawCount)
	}
	if got := fragment.drawAt(0); got != firstDraw {
		t.Fatalf("drawAt(0) = %+v, want %+v", got, firstDraw)
	}
	if got := fragment.drawAt(1); got != secondDraw {
		t.Fatalf("drawAt(1) = %+v, want %+v", got, secondDraw)
	}

	firstHit := sourceHit{source: source.NewRange(1, 2), depth: 1}
	secondHit := sourceHit{source: source.NewRange(3, 4), depth: 2}
	if index := fragment.appendHit(firstHit); index != 0 {
		t.Fatalf("first appendHit index = %d, want 0", index)
	}
	if index := fragment.appendHit(secondHit); index != 1 {
		t.Fatalf("second appendHit index = %d, want 1", index)
	}
	if fragment.hitCount != 2 || fragment.flatHits != 2 {
		t.Fatalf("hit counters = inline:%d flat:%d, want 2/2", fragment.hitCount, fragment.flatHits)
	}
	if got, ok := fragment.hitAt(0); !ok || *got != firstHit {
		t.Fatalf("hitAt(0) = %+v, %v; want %+v, true", got, ok, firstHit)
	}
	if got, ok := fragment.hitAt(1); !ok || *got != secondHit {
		t.Fatalf("hitAt(1) = %+v, %v; want %+v, true", got, ok, secondHit)
	}
	if _, ok := fragment.hitAt(2); ok {
		t.Fatal("hitAt(2) unexpectedly succeeded")
	}

	reserved := emptyFragment(Text, geometry.Em(geometry.EmScale))
	reserved.reserveDraws(4)
	if cap(reserved.moreDraws) != 3 {
		t.Fatalf("reserved tail capacity = %d, want 3", cap(reserved.moreDraws))
	}
	for i := 0; i < 4; i++ {
		reserved.appendDraw(placedDraw{x: geometry.Em(i), kind: drawRule})
	}
	for i := 0; i < 4; i++ {
		if got := reserved.drawAt(i).x; got != geometry.Em(i) {
			t.Fatalf("reserved drawAt(%d).x = %d, want %d", i, got, i)
		}
	}
}

func TestSourceHitDepthStorageBounds(t *testing.T) {
	t.Parallel()
	if bits.UintSize <= 32 {
		t.Skip("depth overflow cannot be represented in int on 32-bit platforms")
	}

	state := &layoutState{}
	node := glyph("x", ir.ClassNormal)
	maxDepth := int(^uint32(0))
	fragment := emptyFragment(Text, geometry.Em(geometry.EmScale))
	if err := state.attachNodeHit(fragment, node, maxDepth); err != nil {
		t.Fatalf("attachNodeHit(max depth) error = %v", err)
	}
	if hit, ok := fragment.hitAt(0); !ok || hit.depth != uint32(maxDepth) {
		t.Fatalf("stored hit = %+v, %v; want depth %d", hit, ok, maxDepth)
	}
	if err := state.attachNodeHit(fragment, node, maxDepth+1); err == nil {
		t.Fatal("attachNodeHit() accepted a depth wider than storage")
	}
}

func TestFragmentArenaKeepsPointersStableAcrossChunks(t *testing.T) {
	t.Parallel()

	var arena fragmentArena
	first := arena.allocate()
	first.Width = 11
	first.ownHit = 17
	allocated := []*Fragment{first}
	for i := 1; i < firstFragmentChunk+3; i++ {
		fragment := arena.allocate()
		fragment.Width = geometry.Em(i + 1)
		fragment.ownHit = -1
		allocated = append(allocated, fragment)
	}

	if len(arena.chunks) < 2 {
		t.Fatalf("arena chunks = %d, want at least 2", len(arena.chunks))
	}
	if first.Width != 11 || first.ownHit != 17 {
		t.Fatalf("first fragment changed after chunk growth: width=%d ownHit=%d", first.Width, first.ownHit)
	}
	seen := make(map[*Fragment]struct{}, len(allocated))
	for i, fragment := range allocated {
		if _, exists := seen[fragment]; exists {
			t.Fatalf("fragment %d reused arena storage", i)
		}
		seen[fragment] = struct{}{}
		if i > 0 && (fragment.Width != geometry.Em(i+1) || fragment.ownHit != -1) {
			t.Fatalf("fragment %d = width:%d ownHit:%d", i, fragment.Width, fragment.ownHit)
		}
	}
}

func TestFragmentOutputCountsMatchFlattenedDisplay(t *testing.T) {
	t.Parallel()

	renderer, _, _ := newTestRenderer(t)
	scripts := ir.NewScripts(source.NewRange(0, 3), glyph("x", ir.ClassNormal))
	scripts.TopRight = glyph("2", ir.ClassNormal)
	scripts.BottomRight = glyph("1", ir.ClassNormal)
	tests := []struct {
		name string
		node ir.Node
	}{
		{
			name: "row",
			node: ir.NewRow(
				source.NewRange(0, 3),
				glyph("x", ir.ClassNormal),
				glyph("+", ir.ClassBinary),
				glyph("y", ir.ClassNormal),
			),
		},
		{name: "scripts", node: scripts},
		{
			name: "fraction",
			node: ir.NewFraction(
				source.NewRange(0, 3),
				glyph("1", ir.ClassNormal),
				glyph("2", ir.ClassNormal),
			),
		},
		{
			name: "radical",
			node: ir.NewRadical(
				source.NewRange(0, 3),
				glyph("x", ir.ClassNormal),
				glyph("3", ir.ClassNormal),
			),
		},
		{
			name: "fenced",
			node: ir.NewFenced(source.NewRange(0, 3), glyph("x", ir.ClassNormal)),
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			state := newFragmentAllocLayoutState(t, renderer)
			fragment, err := state.layoutNode(test.node, state.environment(), 0)
			if err != nil {
				t.Fatalf("layoutNode() error = %v", err)
			}
			box, err := state.flattenFragment(fragment)
			if err != nil {
				t.Fatalf("flattenFragment() error = %v", err)
			}
			if len(box.Items) != fragment.flatItems || cap(box.Items) != fragment.flatItems {
				t.Fatalf("items len/cap = %d/%d, flat count = %d", len(box.Items), cap(box.Items), fragment.flatItems)
			}
			if len(box.Hits) != fragment.flatHits || cap(box.Hits) != fragment.flatHits {
				t.Fatalf("hits len/cap = %d/%d, flat count = %d", len(box.Hits), cap(box.Hits), fragment.flatHits)
			}
		})
	}
}

func newFragmentAllocLayoutState(t *testing.T, renderer *Renderer) *layoutState {
	t.Helper()
	style := testStyle(16)
	hostSize, err := hostSizeFromDisplay(style.Size)
	if err != nil {
		t.Fatal(err)
	}
	rootStyle, err := rootStyle(ir.Inline, 0)
	if err != nil {
		t.Fatal(err)
	}
	return &layoutState{
		ctx:       context.Background(),
		renderer:  renderer,
		request:   style,
		mode:      ir.Inline,
		rootStyle: rootStyle,
		metrics:   fakeStyleFont{}.MathMetrics(rootStyle, ir.VariantAuto).normalized(),
		hostSize:  hostSize,
		limits:    source.DefaultLimits(),
	}
}

func (s *layoutState) environment() environment {
	return environment{
		Style:   s.rootStyle,
		Variant: ir.VariantAuto,
		Font:    ir.FontMath,
	}
}
