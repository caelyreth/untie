package typst

import (
	"runtime"
	"strconv"
	"strings"
	"testing"

	"github.com/caelyreth/untie/ir"
	"github.com/caelyreth/untie/source"
)

func TestASTArenaPointersRemainStableAcrossBlocks(t *testing.T) {
	t.Parallel()

	arena := newASTArena(1)
	nodes := make([]*astNode, 0, astArenaMaxBlock*2+3)
	for i := 0; i < cap(nodes); i++ {
		node := arena.alloc(astGlyph, source.NewRange(i, i+1))
		node.text = strconv.Itoa(i)
		nodes = append(nodes, node)
	}

	runtime.GC()
	for i, node := range nodes {
		if node.kind != astGlyph || node.span != source.NewRange(i, i+1) || node.text != strconv.Itoa(i) {
			t.Fatalf("node %d changed after arena growth: %#v", i, node)
		}
	}
}

func TestParserArenaOwnsReturnedTree(t *testing.T) {
	t.Parallel()

	src := strings.Repeat("x + ", 300) + "y"
	root, diagnostics := parseSource(src, ir.Inline, source.DefaultLimits())
	if len(diagnostics) != 0 {
		t.Fatalf("parse diagnostics = %#v", diagnostics)
	}

	runtime.GC()
	stack := []*astNode{root}
	visited := 0
	for len(stack) > 0 {
		node := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if node == nil {
			t.Fatal("arena tree contains a nil node")
		}
		visited++
		if !node.span.IsValidFor(len(src)) {
			t.Fatalf("node %d has invalid span %#v", visited, node.span)
		}
		stack = append(stack, node.items...)
		for _, child := range [...]*astNode{node.base, node.top, node.bottom, node.topLeft, node.bottomLeft, node.topRight, node.bottomRight} {
			if child != nil {
				stack = append(stack, child)
			}
		}
	}
	if visited < 600 {
		t.Fatalf("visited %d nodes, want the complete arena-backed tree", visited)
	}
}
