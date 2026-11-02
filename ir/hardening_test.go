package ir

import (
	"math"
	"strings"
	"testing"

	"github.com/caelyreth/untie/source"
)

func TestValidateSourceChecksOwnership(t *testing.T) {
	t.Parallel()

	formula := &Formula{Root: NewGlyph(source.NewRange(0, 2), "x")}
	if err := ValidateSource(formula, "x", source.Limits{}); err == nil {
		t.Fatal("ValidateSource() accepted a range beyond source")
	}
	if err := ValidateSource(&Formula{Root: NewGlyph(source.NewRange(0, 1), "x")}, "x", source.Limits{}); err != nil {
		t.Fatalf("ValidateSource() valid error = %v", err)
	}
	if err := ValidateSource(&Formula{Root: NewGlyph(source.NewRange(0, 2), "x")}, "xy", source.Limits{MaxSourceBytes: 1}); err == nil {
		t.Fatal("ValidateSource() accepted source beyond byte limit")
	}
}

func TestValidateRejectsKindAndFieldMismatches(t *testing.T) {
	t.Parallel()

	span := source.NewRange(0, 1)
	tests := []struct {
		name string
		root Node
	}{
		{
			name: "kind mismatch",
			root: func() Node {
				node := NewGlyph(span, "x")
				node.Base = NewBase(KindRow, span)
				return node
			}(),
		},
		{
			name: "class",
			root: func() Node {
				node := NewGlyph(span, "x")
				node.Class = MathClass(255)
				return node
			}(),
		},
		{
			name: "paint role",
			root: func() Node {
				node := NewGlyph(span, "x")
				node.Paint.Role = PaintRole(255)
				return node
			}(),
		},
		{
			name: "explicit paint with role",
			root: func() Node {
				node := NewGlyph(span, "x")
				node.Paint = Paint{Role: PaintError, HasExplicit: true}
				return node
			}(),
		},
		{
			name: "non-explicit space amount",
			root: func() Node {
				node := NewSpace(span, SpaceInterword, RawLength(LengthEm, 1_000_000))
				return node
			}(),
		},
		{
			name: "style cramped flag",
			root: func() Node {
				node := NewStyle(span, NewGlyph(span, "x"))
				node.Cramped = true
				return node
			}(),
		},
		{
			name: "box border without flag",
			root: func() Node {
				node := NewBox(span, NewGlyph(span, "x"))
				node.BorderWidth = RawLength(LengthEm, 1_000_000)
				return node
			}(),
		},
		{
			name: "balanced fence missing close",
			root: func() Node {
				node := NewFenced(span, NewGlyph(span, "x"))
				node.Open = NewGlyph(span, "(")
				node.Balanced = true
				return node
			}(),
		},
		{
			name: "table row without cells",
			root: NewTable(span, TableRow{}),
		},
		{
			name: "table line position",
			root: func() Node {
				node := NewTable(span, TableRow{Cells: []Node{NewGlyph(span, "x")}})
				node.Horizontal = []TableLine{{Position: 2, Count: 1}}
				return node
			}(),
		},
		{
			name: "table line count",
			root: func() Node {
				node := NewTable(span, TableRow{Cells: []Node{NewGlyph(span, "x")}})
				node.Vertical = []TableLine{{Position: 0, Count: 0}}
				return node
			}(),
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if err := Validate(&Formula{Root: tc.root}, source.Limits{}); err == nil {
				t.Fatal("Validate() error = nil")
			}
		})
	}
}

func TestValidateAppliesTextAndDimensionLimits(t *testing.T) {
	t.Parallel()

	span := source.NewRange(0, 1)
	nodes := []Node{
		NewError(span, "long", source.CodeSyntax),
		NewOperator(span, "long"),
		NewPlaceholder(span, "long reason"),
	}
	for _, node := range nodes {
		if err := Validate(&Formula{Root: node}, source.Limits{MaxTextBytes: 3}); err == nil {
			t.Fatalf("Validate(%T) accepted text beyond MaxTextBytes", node)
		}
	}
	line := NewLine(span, NewGlyph(span, "x"), PositionAbove)
	line.Gap = RawLength(LengthEm, 2_000_000)
	if err := Validate(&Formula{Root: line}, source.Limits{MaxDimension: 1}); err == nil {
		t.Fatal("Validate() accepted a length beyond MaxDimension")
	}
}

func TestLengthConstructorsRejectInvalidValues(t *testing.T) {
	t.Parallel()

	constructors := []func(float64) (Length, error){
		EmLength, ExLength, MuLength, PtLength, DipLength, PercentLength,
	}
	for _, constructor := range constructors {
		for _, value := range []float64{math.NaN(), math.Inf(1), math.Inf(-1), 1 << 41} {
			if _, err := constructor(value); err == nil {
				t.Fatalf("constructor accepted %v", value)
			}
		}
	}
	length, err := EmLength(1.5)
	if err != nil || length.Float() != 1.5 {
		t.Fatalf("EmLength(1.5) = %+v, %v", length, err)
	}
	if err := RawLength(LengthEm, math.MinInt64).Validate(); err == nil {
		t.Fatal("Validate() accepted MinInt64 length")
	}
	if err := (Length{Unit: LengthZero, Value: 1}).Validate(); err == nil {
		t.Fatal("Validate() accepted non-zero LengthZero value")
	}
}

func TestValidateErrorMessagesAreActionable(t *testing.T) {
	t.Parallel()

	node := NewGlyph(source.NewRange(0, 1), "x")
	node.Base = NewBase(KindRow, source.NewRange(0, 1))
	err := Validate(&Formula{Root: node}, source.Limits{})
	if err == nil || !strings.Contains(err.Error(), "glyph") || !strings.Contains(err.Error(), "row") {
		t.Fatalf("Validate() error = %v", err)
	}
}
