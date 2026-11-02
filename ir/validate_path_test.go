package ir

import (
	"testing"

	"github.com/caelyreth/untie/source"
)

func TestValidatePreservesDiagnosticPaths(t *testing.T) {
	t.Parallel()

	span := source.NewRange(0, 1)
	glyph := func(text string) Node { return NewGlyph(span, text) }

	cycle := NewRow(span)
	cycle.Items = []Node{cycle}

	box := NewBox(span, glyph("x"))
	box.Padding[2] = RawLength(LengthEm, -1)

	table := NewTable(span, TableRow{Cells: []Node{glyph("x")}})
	table.Horizontal = []TableLine{{Position: 0, Count: 0}}

	tableThickness := NewTable(span, TableRow{Cells: []Node{glyph("x")}})
	tableThickness.Horizontal = []TableLine{{
		Position:  0,
		Count:     1,
		Thickness: RawLength(LengthUnit(99), 0),
	}}

	tests := []struct {
		name    string
		formula *Formula
		limits  source.Limits
		want    string
	}{
		{
			name:    "nested node",
			formula: &Formula{Root: NewRow(span, NewFraction(span, glyph(""), glyph("b")))},
			want:    "ir: invalid: root.row[0].fraction[0]: empty glyph",
		},
		{
			name:    "span field",
			formula: &Formula{Root: NewRow(span, NewGlyph(source.Range{Start: 2, End: 1}, "x"))},
			want:    "ir: invalid: root.row[0].span: invalid range",
		},
		{
			name:    "component field",
			formula: &Formula{Root: box},
			want:    "ir: invalid: root.padding[2]: negative length",
		},
		{
			name:    "table line",
			formula: &Formula{Root: table},
			want:    "ir: invalid: root.horizontal[0]: horizontal line count must be positive",
		},
		{
			name:    "table line field",
			formula: &Formula{Root: tableThickness},
			want:    "ir: invalid: root.horizontal[0]: ir: invalid: thickness: ir: invalid length: unknown unit 99",
		},
		{
			name: "tag body",
			formula: &Formula{
				Root: glyph("x"),
				Tags: []EquationTag{{
					Body: NewRow(span, nil), Span: span, Origin: span, Manual: true, Line: FormulaTagLine,
				}},
			},
			want: "ir: invalid: tags[0].body.row[0]: nil child",
		},
		{
			name:    "traversal limit",
			formula: &Formula{Root: NewRow(span, glyph("x"))},
			limits:  source.Limits{MaxNodes: 1},
			want:    "ir: invalid: ir: traversal limit exceeded: root.row[0]: node limit exceeded",
		},
		{
			name:    "nesting limit",
			formula: &Formula{Root: NewRow(span, NewRow(span, glyph("x")))},
			limits:  source.Limits{MaxNesting: 1},
			want:    "ir: invalid: ir: traversal limit exceeded: root.row[0].row[0]: nesting limit exceeded",
		},
		{
			name:    "cycle",
			formula: &Formula{Root: cycle},
			want:    "ir: invalid: ir: cycle detected: root.row[0]",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			err := Validate(tc.formula, tc.limits)
			if err == nil {
				t.Fatal("Validate() error = nil")
			}
			if got := err.Error(); got != tc.want {
				t.Fatalf("Validate() error = %q, want %q", got, tc.want)
			}
		})
	}
}
