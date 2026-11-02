package ir

import (
	"errors"
	"testing"

	"github.com/caelyreth/untie/source"
)

func TestChildrenReturnsIndependentSlices(t *testing.T) {
	t.Parallel()

	span := source.NewRange(0, 1)
	glyph := NewGlyph(span, "x")
	row := NewRow(span, glyph)
	children := Children(row)
	children[0] = NewGlyph(span, "y")
	if row.Items[0] != glyph {
		t.Fatal("Children(row) exposed the backing slice")
	}

	fenced := NewFenced(span, glyph)
	fenced.Open = NewGlyph(span, "(")
	fenced.Close = NewGlyph(span, ")")
	fencedChildren := Children(fenced)
	fencedChildren[1] = NewGlyph(span, "z")
	if fenced.Body[0] != glyph {
		t.Fatal("Children(fenced) exposed body storage")
	}

	multiline := NewMultiline(span, []Node{glyph}, []Node{glyph})
	multilineChildren := Children(multiline)
	multilineChildren[0] = NewGlyph(span, "q")
	if multiline.Rows[0][0] != glyph {
		t.Fatal("Children(multiline) exposed row storage")
	}
}

func TestGraphUtilitiesRejectCycles(t *testing.T) {
	t.Parallel()

	span := source.NewRange(0, 1)
	row := NewRow(span)
	row.Items = []Node{row}
	formula := &Formula{Root: row}

	if err := Validate(formula, source.Limits{}); !errors.Is(err, ErrCycle) {
		t.Fatalf("Validate() error = %v, want ErrCycle", err)
	}
	if err := WalkWithLimits(row, source.Limits{}, func(Node) bool { return true }); !errors.Is(err, ErrCycle) {
		t.Fatalf("WalkWithLimits() error = %v, want ErrCycle", err)
	}
	if _, err := MarshalCanonical(formula); !errors.Is(err, ErrCycle) {
		t.Fatalf("MarshalCanonical() error = %v, want ErrCycle", err)
	}
	if _, err := CloneFormula(formula); !errors.Is(err, ErrCycle) {
		t.Fatalf("CloneFormula() error = %v, want ErrCycle", err)
	}
}

func TestGraphUtilitiesEnforceBudgets(t *testing.T) {
	t.Parallel()

	span := source.NewRange(0, 2)
	root := NewRow(span, NewGlyph(source.NewRange(0, 1), "x"), NewGlyph(source.NewRange(1, 2), "y"))
	formula := &Formula{Root: root}
	limits := source.Limits{MaxNodes: 2, MaxNesting: 1}

	if err := Validate(formula, limits); !errors.Is(err, ErrTraversalLimit) {
		t.Fatalf("Validate() error = %v, want ErrTraversalLimit", err)
	}
	if err := WalkWithLimits(root, limits, func(Node) bool { return true }); !errors.Is(err, ErrTraversalLimit) {
		t.Fatalf("WalkWithLimits() error = %v, want ErrTraversalLimit", err)
	}
	if _, err := MarshalCanonical(formula, limits); !errors.Is(err, ErrTraversalLimit) {
		t.Fatalf("MarshalCanonical() error = %v, want ErrTraversalLimit", err)
	}
	if _, err := CloneFormula(formula, limits); !errors.Is(err, ErrTraversalLimit) {
		t.Fatalf("CloneFormula() error = %v, want ErrTraversalLimit", err)
	}
}

func TestWalkWithoutLimitsTerminatesOnCycle(t *testing.T) {
	t.Parallel()

	span := source.NewRange(0, 1)
	row := NewRow(span)
	row.Items = []Node{row}
	visited := 0
	Walk(row, func(Node) bool {
		visited++
		return true
	})
	if visited != 1 {
		t.Fatalf("Walk visited %d nodes, want 1", visited)
	}
}

func TestCloneFormulaIsDeep(t *testing.T) {
	t.Parallel()

	span := source.NewRange(0, 1)
	glyph := NewGlyph(span, "x")
	fraction := NewFraction(span, glyph, NewGlyph(span, "y"))
	root := NewRow(span, fraction)
	clone, err := CloneFormula(&Formula{Root: root})
	if err != nil {
		t.Fatalf("CloneFormula() error = %v", err)
	}
	clonedFraction := clone.Root.(*Row).Items[0].(*Fraction)
	if clonedFraction == fraction {
		t.Fatal("CloneFormula reused the fraction node")
	}
	clonedFraction.Numerator.(*Glyph).Text = "changed"
	if glyph.Text != "x" {
		t.Fatal("CloneFormula shared a descendant")
	}
}
