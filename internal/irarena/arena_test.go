package irarena

import (
	"math"
	"runtime"
	"strconv"
	"testing"

	"github.com/caelyreth/untie/ir"
	"github.com/caelyreth/untie/source"
)

func TestArenaBlockPointersRemainStable(t *testing.T) {
	t.Parallel()

	arena := New(4_096)
	glyphs := make([]*ir.Glyph, 0, 3_000)
	rows := make([]*ir.Row, 0, 1_000)
	for i := 0; i < cap(glyphs); i++ {
		glyphs = append(glyphs, arena.NewGlyph(source.NewRange(i, i+1), strconv.Itoa(i)))
	}
	for i := 0; i < cap(rows); i++ {
		rows = append(rows, arena.NewRow(source.NewRange(i, i+1), glyphs[i]))
	}

	runtime.GC()
	for i, glyph := range glyphs {
		if glyph.Text != strconv.Itoa(i) || glyph.Span != source.NewRange(i, i+1) {
			t.Fatalf("glyph %d changed after arena growth: %#v", i, glyph)
		}
	}
	for i, row := range rows {
		if len(row.Items) != 1 || row.Items[0] != glyphs[i] {
			t.Fatalf("row %d changed after arena growth: %#v", i, row)
		}
	}
}

func TestArenaDirectPointersRemainStable(t *testing.T) {
	t.Parallel()

	arena := New(1)
	first := arena.NewGlyph(source.NewRange(0, 1), "x")
	second := arena.NewGlyph(source.NewRange(1, 2), "y")
	runtime.GC()
	if first.Text != "x" || second.Text != "y" {
		t.Fatalf("direct arena nodes changed: %#v %#v", first, second)
	}
}

func TestArenaShortInputUsesBlocks(t *testing.T) {
	t.Parallel()

	arena := New(arenaMinHint)
	if !arena.glyphs.enabled || !arena.rows.enabled {
		t.Fatal("short input did not enable IR node blocks")
	}
}

func TestArenaCapsInitialHint(t *testing.T) {
	t.Parallel()

	arena := New(math.MaxInt)
	if len(arena.glyphs.block) > maxArenaInitialHint {
		t.Fatalf("glyph block = %d slots, want at most %d", len(arena.glyphs.block), maxArenaInitialHint)
	}
	if len(arena.rows.block) > maxArenaInitialHint {
		t.Fatalf("row block = %d slots, want at most %d", len(arena.rows.block), maxArenaInitialHint)
	}
}
