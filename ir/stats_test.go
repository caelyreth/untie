package ir

import (
	"testing"

	"github.com/caelyreth/untie/source"
)

func TestValidateWithStatsEnvelope(t *testing.T) {
	t.Parallel()

	shrink := NewGlyph(source.NewRange(0, 1), "x")
	scripts := NewScripts(source.NewRange(0, 2), shrink)
	scripts.Top = NewGlyph(source.NewRange(1, 2), "2")

	boxed := NewBox(source.NewRange(0, 4), NewGlyph(source.NewRange(0, 4), "abcd"))
	boxed.Padding[0] = RawLength(LengthEm, 2_000_000)
	table := NewTable(source.NewRange(0, 3),
		TableRow{Cells: []Node{NewGlyph(source.NewRange(0, 1), "a")}},
		TableRow{Cells: []Node{
			NewGlyph(source.NewRange(1, 2), "b"),
			NewGlyph(source.NewRange(2, 3), "c"),
		}},
	)
	root := NewRow(source.NewRange(0, 8), scripts, boxed, table, NewText(source.NewRange(0, 5), "hello"))
	tagBody := NewRow(source.NewRange(7, 8), NewGlyph(source.NewRange(7, 8), "1"))
	formula := &Formula{
		Root: root,
		Tags: []EquationTag{{
			Body:   tagBody,
			Span:   source.NewRange(7, 8),
			Origin: source.NewRange(7, 8),
			Manual: true,
			Line:   FormulaTagLine,
		}},
	}

	stats, err := ValidateWithStats(formula, source.DefaultLimits())
	if err != nil {
		t.Fatalf("ValidateWithStats() error = %v", err)
	}
	if stats.Nodes != 14 || stats.Tags != 1 {
		t.Fatalf("Stats nodes/tags = %d/%d, want 14/1", stats.Nodes, stats.Tags)
	}
	if stats.MaxDepth < 2 || stats.MaxTextBytes != 5 || stats.MaxTableCells != 3 || stats.MaxDimensionValue != 2_000_000 {
		t.Fatalf("Stats envelope = %+v", stats)
	}
	if !stats.WithinLimits(source.DefaultLimits()) {
		t.Fatal("WithinLimits() rejected the source limits used for validation")
	}

	for name, limits := range map[string]source.Limits{
		"nodes":      {MaxNodes: stats.Nodes - 1},
		"nesting":    {MaxNesting: stats.MaxDepth - 1},
		"text":       {MaxTextBytes: stats.MaxTextBytes - 1},
		"dimension":  {MaxDimension: 1},
		"table cell": {MaxTableCells: stats.MaxTableCells - 1},
	} {
		if stats.WithinLimits(limits) {
			t.Errorf("WithinLimits(%s) = true, want false", name)
		}
	}
}

func TestValidateSourceWithStatsRejectsSourceWithNoEnvelope(t *testing.T) {
	t.Parallel()

	formula := &Formula{Root: NewGlyph(source.NewRange(0, 2), "x")}
	stats, err := ValidateSourceWithStats(formula, "x", source.Limits{MaxSourceBytes: 8})
	if err == nil || stats != (Stats{}) {
		t.Fatalf("ValidateSourceWithStats() = %+v, %v; want zero stats and error", stats, err)
	}
}
