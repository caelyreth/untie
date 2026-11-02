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

func TestGraphUtilitiesTreatTypedNilAsNil(t *testing.T) {
	t.Parallel()

	var row *Row
	if children := Children(row); children != nil {
		t.Fatalf("Children(typed nil) = %v", children)
	}
	if err := WalkWithLimits(row, source.Limits{}, func(Node) bool { return true }); err != nil {
		t.Fatalf("WalkWithLimits(typed nil) error = %v", err)
	}
	if err := Validate(&Formula{Root: row}, source.Limits{}); err == nil {
		t.Fatal("Validate() accepted a typed nil root")
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

func TestCloneFormulaCoversAllNodeKinds(t *testing.T) {
	t.Parallel()

	span := source.NewRange(0, 1)
	glyph := func(text string) Node { return NewGlyph(span, text) }
	body := func() Node { return NewRow(span, glyph("x")) }
	fraction := NewFraction(span, glyph("1"), glyph("2"))
	fraction.Role = FractionRoleBinomial
	fraction.Style = FractionNoBar
	fraction.Bar = false
	scripts := &Scripts{Base: NewBase(KindScripts, span), BaseItem: glyph("x"), Top: glyph("2"), Role: ScriptRoleLimits}
	cancel := NewCancel(span, glyph("x"))
	cancel.Role = CancelRoleNegation
	table := &Table{
		Base: NewBase(KindTable, span),
		Rows: []TableRow{{Cells: []Node{glyph("x")}}},
		Role: TableRoleCases,
	}
	nodes := map[string]Node{
		"row":      body(),
		"glyph":    glyph("x"),
		"text":     NewText(span, "x"),
		"operator": NewOperator(span, "sin"),
		"space":    NewSpace(span, SpaceExplicit, RawLength(LengthEm, 1_000_000)),
		"fraction": fraction,
		"radical":  NewRadical(span, glyph("x"), nil),
		"scripts":  scripts,
		"fenced":   &Fenced{Base: NewBase(KindFenced, span), Open: glyph("("), Close: glyph(")"), Body: []Node{glyph("x")}, Balanced: true},
		"sized-delimiter": &SizedDelimiter{
			Base:   NewBase(KindSizedDelimiter, span),
			Text:   "(",
			Sizing: AbsoluteDelimiterSizing(RawLength(LengthEm, 1_200_000)),
		},
		"accent":      NewAccent(span, glyph("x"), glyph("^"), PositionAbove),
		"line":        NewLine(span, glyph("x"), PositionAbove),
		"cancel":      cancel,
		"table":       table,
		"multiline":   NewMultiline(span, []Node{glyph("x")}),
		"style":       NewStyle(span, glyph("x")),
		"color":       NewColor(span, glyph("x"), Paint{Explicit: RGBA{A: 255}, HasExplicit: true}),
		"box":         NewBox(span, glyph("x")),
		"phantom":     NewPhantom(span, glyph("x")),
		"error":       NewError(span, "bad", source.CodeSyntax),
		"placeholder": NewPlaceholder(span, "missing"),
		"break":       NewBreak(span, BreakRelation, 0),
		"align":       NewAlignMarker(span),
	}
	for name, node := range nodes {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			formula := &Formula{Root: node}
			clone, err := CloneFormula(formula)
			if err != nil {
				t.Fatalf("CloneFormula() error = %v", err)
			}
			originalJSON, err := MarshalCanonical(formula)
			if err != nil {
				t.Fatalf("MarshalCanonical(original) error = %v", err)
			}
			cloneJSON, err := MarshalCanonical(clone)
			if err != nil {
				t.Fatalf("MarshalCanonical(clone) error = %v", err)
			}
			if string(originalJSON) != string(cloneJSON) {
				t.Fatalf("clone differs:\noriginal: %s\nclone: %s", originalJSON, cloneJSON)
			}
		})
	}
}
