package ir

import (
	"bytes"
	"errors"
	"testing"

	"github.com/caelyreth/untie/source"
)

func TestFormulaTagsValidateCloneWalkAndCanonicalize(t *testing.T) {
	t.Parallel()

	span := source.NewRange(0, 8)
	body := NewGlyph(source.NewRange(5, 6), "1")
	formula := &Formula{
		Root: NewGlyph(source.NewRange(0, 1), "x"),
		Tags: []EquationTag{{
			Body:   body,
			Span:   span,
			Origin: span,
			Manual: true,
			Line:   FormulaTagLine,
		}},
	}
	if err := ValidateSource(formula, `x\tag{1}`, source.Limits{}); err != nil {
		t.Fatalf("ValidateSource() error = %v", err)
	}
	visited := 0
	if err := WalkFormula(formula, source.Limits{}, func(Node) bool {
		visited++
		return true
	}); err != nil {
		t.Fatalf("WalkFormula() error = %v", err)
	}
	if visited != 2 {
		t.Fatalf("WalkFormula visited %d nodes, want 2", visited)
	}
	clone, err := CloneFormula(formula)
	if err != nil {
		t.Fatalf("CloneFormula() error = %v", err)
	}
	if clone.Tags[0].Body == body {
		t.Fatal("CloneFormula shared the tag body")
	}
	clone.Tags[0].Body.(*Glyph).Text = "2"
	if body.Text != "1" {
		t.Fatal("mutating the clone changed the original tag body")
	}
	originalJSON, err := MarshalCanonical(formula)
	if err != nil {
		t.Fatalf("MarshalCanonical(original) error = %v", err)
	}
	if !bytes.Contains(originalJSON, []byte(`"tags"`)) ||
		!bytes.Contains(originalJSON, []byte(`"manual":true`)) {
		t.Fatalf("canonical formula omitted tag metadata: %s", originalJSON)
	}
	recloned, err := CloneFormula(formula)
	if err != nil {
		t.Fatalf("CloneFormula() error = %v", err)
	}
	cloneJSON, err := MarshalCanonical(recloned)
	if err != nil {
		t.Fatalf("MarshalCanonical(clone) error = %v", err)
	}
	if !bytes.Equal(originalJSON, cloneJSON) {
		t.Fatalf("clone canonicalization differs:\n%s\n%s", originalJSON, cloneJSON)
	}
}

func TestFormulaTagsRejectMalformedMetadata(t *testing.T) {
	t.Parallel()

	span := source.NewRange(0, 4)
	root := NewGlyph(source.NewRange(0, 1), "x")
	body := NewGlyph(source.NewRange(1, 2), "1")
	tests := []struct {
		name string
		tag  EquationTag
	}{
		{name: "manual without body", tag: EquationTag{Span: span, Origin: span, Manual: true}},
		{name: "neither manual nor suppressed", tag: EquationTag{Body: body, Span: span, Origin: span}},
		{name: "suppressed with body", tag: EquationTag{Body: body, Span: span, Origin: span, Suppressed: true}},
		{name: "manual and suppressed", tag: EquationTag{Body: body, Span: span, Origin: span, Manual: true, Suppressed: true}},
		{name: "starred suppression", tag: EquationTag{Span: span, Origin: span, Suppressed: true, Starred: true}},
		{name: "invalid line", tag: EquationTag{Body: body, Span: span, Origin: span, Manual: true, Line: -2}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if err := Validate(&Formula{Root: root, Tags: []EquationTag{tc.tag}}, source.Limits{}); err == nil {
				t.Fatal("Validate() error = nil")
			}
		})
	}
}

func TestFormulaTagsShareNodeAndSourceLimits(t *testing.T) {
	t.Parallel()

	span := source.NewRange(0, 2)
	formula := &Formula{
		Root: NewGlyph(source.NewRange(0, 1), "x"),
		Tags: []EquationTag{{
			Body:   NewGlyph(source.NewRange(1, 2), "1"),
			Span:   span,
			Origin: span,
			Manual: true,
			Line:   FormulaTagLine,
		}},
	}
	if err := Validate(formula, source.Limits{MaxNodes: 1}); !errors.Is(err, ErrTraversalLimit) {
		t.Fatalf("Validate() error = %v, want ErrTraversalLimit", err)
	}
	if _, err := MarshalCanonical(formula, source.Limits{MaxNodes: 1}); !errors.Is(err, ErrTraversalLimit) {
		t.Fatalf("MarshalCanonical() error = %v, want ErrTraversalLimit", err)
	}
	if _, err := CloneFormula(formula, source.Limits{MaxNodes: 1}); !errors.Is(err, ErrTraversalLimit) {
		t.Fatalf("CloneFormula() error = %v, want ErrTraversalLimit", err)
	}
	if err := ValidateSource(formula, "x", source.Limits{}); err == nil {
		t.Fatal("ValidateSource() accepted a tag range outside source")
	}
}

func TestFormulaAllowsSuppressionWithoutBody(t *testing.T) {
	t.Parallel()

	span := source.NewRange(0, 1)
	formula := &Formula{
		Root: NewGlyph(span, "x"),
		Tags: []EquationTag{{
			Span:       span,
			Origin:     span,
			Suppressed: true,
			Line:       FormulaTagLine,
		}},
	}
	if err := Validate(formula, source.Limits{}); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
	if err := WalkFormula(formula, source.Limits{}, func(Node) bool { return true }); err != nil {
		t.Fatalf("WalkFormula() error = %v", err)
	}
}
