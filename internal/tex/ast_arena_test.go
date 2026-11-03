package tex

import (
	"runtime"
	"strconv"
	"testing"

	"github.com/caelyreth/untie/ir"
	"github.com/caelyreth/untie/source"
)

func TestASTArenaGlyphPointersRemainStable(t *testing.T) {
	t.Parallel()

	arena := newASTArena(1_024)
	nodes := make([]*astGlyphNode, 0, 1_000)
	for i := 0; i < cap(nodes); i++ {
		nodes = append(nodes, arena.allocGlyph(
			source.NewRange(i, i+1), source.NewRange(i, i+1),
			strconv.Itoa(i), ir.ClassNormal, ir.VariantAuto, ir.LimitsAuto,
		))
	}

	runtime.GC()
	for i, node := range nodes {
		if node.text != strconv.Itoa(i) || node.span != source.NewRange(i, i+1) || node.origin != node.span {
			t.Fatalf("glyph %d changed after arena growth: %#v", i, node)
		}
	}
}

func TestASTArenaSmallInputUsesStableDirectNodes(t *testing.T) {
	t.Parallel()

	arena := newASTArena(1)
	first := arena.allocGlyph(source.NewRange(0, 1), source.NewRange(0, 1), "x", ir.ClassNormal, ir.VariantAuto, ir.LimitsAuto)
	second := arena.allocGlyph(source.NewRange(1, 2), source.NewRange(1, 2), "y", ir.ClassNormal, ir.VariantAuto, ir.LimitsAuto)
	runtime.GC()
	if first.text != "x" || second.text != "y" {
		t.Fatalf("direct AST nodes changed: %#v %#v", first, second)
	}
}
