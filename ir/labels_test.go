package ir

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/caelyreth/untie/source"
)

func TestFormulaLabelsValidateCloneWalkAndCanonicalize(t *testing.T) {
	t.Parallel()

	const src = `x\label{eq:x}\label{eq:y}`
	first := source.NewRange(1, 13)
	second := source.NewRange(13, 25)
	formula := &Formula{
		Root: NewGlyph(source.NewRange(0, 1), "x"),
		Labels: []EquationLabel{
			{Name: "eq:x", Span: first, Origin: first},
			{Name: "eq:y", Span: second, Origin: second},
		},
	}
	stats, err := ValidateSourceWithStats(formula, src, source.Limits{})
	if err != nil {
		t.Fatalf("ValidateSourceWithStats() error = %v", err)
	}
	if stats.Labels != 2 || stats.Nodes != 3 {
		t.Fatalf("Stats labels/nodes = %d/%d, want 2/3", stats.Labels, stats.Nodes)
	}

	visited := 0
	if err := WalkFormula(formula, source.Limits{}, func(Node) bool {
		visited++
		return true
	}); err != nil {
		t.Fatalf("WalkFormula() error = %v", err)
	}
	if visited != 1 {
		t.Fatalf("WalkFormula visited %d nodes, want only the root", visited)
	}

	clone, err := CloneFormula(formula)
	if err != nil {
		t.Fatalf("CloneFormula() error = %v", err)
	}
	clone.Labels[0].Name = "changed"
	if formula.Labels[0].Name != "eq:x" {
		t.Fatal("mutating the clone changed original label metadata")
	}

	originalJSON, err := MarshalCanonical(formula)
	if err != nil {
		t.Fatalf("MarshalCanonical(original) error = %v", err)
	}
	if !bytes.Contains(originalJSON, []byte(`"labels"`)) ||
		!bytes.Contains(originalJSON, []byte(`"name":"eq:x"`)) {
		t.Fatalf("canonical formula omitted label metadata: %s", originalJSON)
	}
	recloned, err := CloneFormula(formula)
	if err != nil {
		t.Fatalf("CloneFormula(second) error = %v", err)
	}
	cloneJSON, err := MarshalCanonical(recloned)
	if err != nil {
		t.Fatalf("MarshalCanonical(clone) error = %v", err)
	}
	if !bytes.Equal(originalJSON, cloneJSON) {
		t.Fatalf("label clone canonicalization differs:\n%s\n%s", originalJSON, cloneJSON)
	}
}

func TestFormulaLabelsRejectMalformedMetadata(t *testing.T) {
	t.Parallel()

	span := source.NewRange(0, 4)
	root := NewGlyph(source.NewRange(0, 1), "x")
	tests := []struct {
		name  string
		label EquationLabel
	}{
		{name: "empty name", label: EquationLabel{Span: span, Origin: span}},
		{name: "invalid UTF-8", label: EquationLabel{Name: string([]byte{0xff}), Span: span, Origin: span}},
		{name: "NUL", label: EquationLabel{Name: "eq\x00x", Span: span, Origin: span}},
		{name: "invalid span", label: EquationLabel{Name: "eq:x", Span: source.Range{Start: 3, End: 2}, Origin: span}},
		{name: "invalid origin", label: EquationLabel{Name: "eq:x", Span: span, Origin: source.Range{Start: -2, End: 1}}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if err := Validate(&Formula{Root: root, Labels: []EquationLabel{tc.label}}, source.Limits{}); err == nil {
				t.Fatal("Validate() error = nil")
			}
		})
	}

	duplicate := []EquationLabel{
		{Name: "eq:x", Span: span, Origin: span},
		{Name: "eq:x", Span: span, Origin: span},
	}
	if err := Validate(&Formula{Root: root, Labels: duplicate}, source.Limits{}); err == nil ||
		!strings.Contains(err.Error(), "duplicate label") {
		t.Fatalf("Validate(duplicate) error = %v, want duplicate label", err)
	}
}

func TestFormulaLabelsShareMetadataBudget(t *testing.T) {
	t.Parallel()

	span := source.NewRange(0, 2)
	formula := &Formula{
		Root:   NewGlyph(source.NewRange(0, 1), "x"),
		Labels: []EquationLabel{{Name: "eq:x", Span: span, Origin: span}},
	}
	if err := Validate(formula, source.Limits{MaxNodes: 1}); !errors.Is(err, ErrTraversalLimit) {
		t.Fatalf("Validate() error = %v, want ErrTraversalLimit", err)
	}
	if _, err := MarshalCanonical(formula, source.Limits{MaxNodes: 1}); !errors.Is(err, ErrTraversalLimit) {
		t.Fatalf("MarshalCanonical() error = %v, want ErrTraversalLimit", err)
	}
	if err := WalkFormula(formula, source.Limits{MaxNodes: 1}, func(Node) bool { return true }); !errors.Is(err, ErrTraversalLimit) {
		t.Fatalf("WalkFormula() error = %v, want ErrTraversalLimit", err)
	}
}
