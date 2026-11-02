package layout

import (
	"context"
	"errors"
	"testing"

	"github.com/caelyreth/untie/display"
	"github.com/caelyreth/untie/ir"
	"github.com/caelyreth/untie/source"
)

func TestSingleLineEquationTagLayoutContract(t *testing.T) {
	t.Parallel()

	renderer, _, _ := newTestRenderer(t)
	formula := taggedTestFormula(glyph("x", ir.ClassNormal), tagGlyph("1"))
	box, err := renderer.LayoutFormulaContext(context.Background(), formula, ir.Display, testStyle(16))
	if err != nil {
		t.Fatalf("LayoutFormulaContext() error = %v", err)
	}
	if box.Tag == nil || box.Tag.Box == nil {
		t.Fatal("Tag = nil")
	}
	if !closeHost(box.Tag.X, 8) || !closeHost(box.Tag.Box.Width, 24) || !closeHost(box.Width, 32) {
		t.Fatalf("tag geometry = x:%v width:%v parent:%v, want 8/24/32",
			box.Tag.X, box.Tag.Box.Width, box.Width)
	}
	if len(box.Items) != 4 {
		t.Fatalf("parent items = %d, want formula plus parenthesized tag", len(box.Items))
	}
	if !closeHost(box.Items[1].Bound.X, 8) {
		t.Fatalf("tag aggregate item x = %v, want 8", box.Items[1].Bound.X)
	}
	foundTagHit := false
	for _, hit := range box.Hits {
		if hit.Source == source.NewRange(1, 2) {
			foundTagHit = true
		}
	}
	if !foundTagHit {
		t.Fatalf("parent hits = %+v, want tag body source", box.Hits)
	}
	if box.Alt != (display.Alt{}) {
		t.Fatalf("layout populated alt unexpectedly: %+v", box.Alt)
	}
	if err := box.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}

	starredFormula := taggedTestFormula(glyph("x", ir.ClassNormal), tagGlyph("1"))
	starredFormula.Tags[0].Starred = true
	starred, err := renderer.LayoutFormulaContext(context.Background(), starredFormula, ir.Display, testStyle(16))
	if err != nil {
		t.Fatalf("LayoutFormulaContext(starred) error = %v", err)
	}
	if !closeHost(starred.Tag.Box.Width, 8) || len(starred.Items) != 2 || !closeHost(starred.Width, 16) {
		t.Fatalf("starred tag geometry = width:%v items:%d parent:%v", starred.Tag.Box.Width, len(starred.Items), starred.Width)
	}
}

func TestEquationTagLayoutRejectsUnsupportedShapes(t *testing.T) {
	t.Parallel()

	renderer, _, _ := newTestRenderer(t)
	inline := taggedTestFormula(glyph("x", ir.ClassNormal), tagGlyph("1"))
	if _, err := renderer.LayoutFormulaContext(context.Background(), inline, ir.Inline, testStyle(16)); !errors.Is(err, ErrUnsupported) {
		t.Fatalf("inline tag error = %v, want ErrUnsupported", err)
	}

	multiline := &ir.Formula{
		Root: ir.NewMultiline(
			source.NewRange(0, 3),
			[]ir.Node{glyph("x", ir.ClassNormal)},
			[]ir.Node{glyph("y", ir.ClassNormal)},
		),
		Tags: taggedTestFormula(glyph("x", ir.ClassNormal), tagGlyph("1")).Tags,
	}
	if _, err := renderer.LayoutFormulaContext(context.Background(), multiline, ir.Display, testStyle(16)); !errors.Is(err, ErrUnsupported) {
		t.Fatalf("multiline tag error = %v, want ErrUnsupported", err)
	}
}

func TestEquationTagLayoutHonorsOutputLimit(t *testing.T) {
	t.Parallel()

	glyphs := &fakeGlyphSource{glyphs: map[string]Glyph{}}
	renderer, err := New(Fonts{Glyphs: glyphs, Metrics: fakeStyleFont{}}, Options{
		Limits: source.Limits{MaxOutputItems: 1},
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	formula := taggedTestFormula(glyph("x", ir.ClassNormal), tagGlyph("1"))
	if _, err := renderer.LayoutFormulaContext(context.Background(), formula, ir.Display, testStyle(16)); !errors.Is(err, ErrLimit) {
		t.Fatalf("tag output limit error = %v, want ErrLimit", err)
	}
}

func tagGlyph(text string) *ir.Glyph {
	node := ir.NewGlyph(source.NewRange(1, 2), text)
	node.Class = ir.ClassNormal
	return node
}

func taggedTestFormula(root, tagBody ir.Node) *ir.Formula {
	return &ir.Formula{
		Root: root,
		Tags: []ir.EquationTag{{
			Body:   tagBody,
			Span:   source.NewRange(1, 2),
			Origin: source.NewRange(1, 2),
			Manual: true,
			Line:   0,
		}},
	}
}
