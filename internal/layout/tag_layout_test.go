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
	if box.Tag == nil || box.Tag.Items.Count == 0 {
		t.Fatal("Tag = nil")
	}
	if !closeHost(box.Tag.X, 8) || !closeHost(box.Tag.Width, 24) || !closeHost(box.Width, 32) {
		t.Fatalf("tag geometry = x:%v width:%v parent:%v, want 8/24/32",
			box.Tag.X, box.Tag.Width, box.Width)
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
	if !closeHost(starred.Tag.Width, 8) || len(starred.Items) != 2 || !closeHost(starred.Width, 16) {
		t.Fatalf("starred tag geometry = width:%v items:%d parent:%v", starred.Tag.Width, len(starred.Items), starred.Width)
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
	box, err := renderer.LayoutFormulaContext(context.Background(), multiline, ir.Display, testStyle(16))
	if err != nil {
		t.Fatalf("nested multiline tag error = %v", err)
	}
	if box.Tag == nil || len(box.Lines) != 0 {
		t.Fatalf("nested multiline tag = %+v lines=%d", box.Tag, len(box.Lines))
	}
}

func TestTaggedConstrainedLayoutAlwaysUsesOneCompleteLine(t *testing.T) {
	t.Parallel()

	renderer, _, _ := newTestRenderer(t)
	root := ir.NewRow(source.NewRange(0, 7),
		glyph("a", ir.ClassNormal),
		glyph("+", ir.ClassBinary),
		glyph("b", ir.ClassNormal),
		glyph("+", ir.ClassBinary),
		glyph("c", ir.ClassNormal),
		glyph("+", ir.ClassBinary),
		glyph("d", ir.ClassNormal),
	)
	formula := taggedTestFormula(root, tagGlyph("1"))
	unconstrained, err := renderer.LayoutFormulaContext(context.Background(), formula, ir.Display, testStyle(16))
	if err != nil {
		t.Fatalf("unconstrained layout error = %v", err)
	}
	wrappingWidth := unconstrained.Tag.X / 3
	untaggedStyle := testStyle(16)
	untaggedStyle.MaxWidth = wrappingWidth
	untagged, err := renderer.Layout(root, ir.Display, untaggedStyle)
	if err != nil {
		t.Fatalf("untagged wrapping layout error = %v", err)
	}
	if len(untagged.Lines) < 2 {
		t.Fatalf("fixture does not wrap untagged: lines=%d width=%v", len(untagged.Lines), wrappingWidth)
	}
	for _, maxWidth := range []float32{1, wrappingWidth, unconstrained.Width / 2, unconstrained.Width - 1, unconstrained.Width + 20} {
		style := testStyle(16)
		style.MaxWidth = maxWidth
		style.Align = display.AlignCenter
		box, err := renderer.LayoutFormulaContext(context.Background(), formula, ir.Display, style)
		if err != nil {
			t.Fatalf("MaxWidth %v layout error = %v", maxWidth, err)
		}
		if len(box.Lines) != 1 || box.Tag == nil || box.Width != maxWidth {
			t.Fatalf("MaxWidth %v = lines:%d tag:%+v width:%v", maxWidth, len(box.Lines), box.Tag, box.Width)
		}
		logicalTagOverflow := maxFloat(0, box.Tag.X+box.Tag.Width-box.Width)
		if box.Overflow+1.0/1024 < logicalTagOverflow {
			t.Fatalf("MaxWidth %v overflow = %v, want at least tag logical overflow %v", maxWidth, box.Overflow, logicalTagOverflow)
		}
	}
}

func TestTaggedConstrainedAlignmentReservesTagWidth(t *testing.T) {
	t.Parallel()

	renderer, _, _ := newTestRenderer(t)
	formula := taggedTestFormula(glyph("x", ir.ClassNormal), tagGlyph("1"))
	fitting := []struct {
		name       string
		align      display.Align
		wantOffset float32
	}{
		{name: "start", align: display.AlignStart, wantOffset: 0},
		{name: "center", align: display.AlignCenter, wantOffset: 16},
		{name: "end", align: display.AlignEnd, wantOffset: 32},
	}
	for _, tc := range fitting {
		t.Run(tc.name, func(t *testing.T) {
			style := testStyle(16)
			style.MaxWidth = 64
			style.Align = tc.align
			box, err := renderer.LayoutFormulaContext(context.Background(), formula, ir.Display, style)
			if err != nil {
				t.Fatalf("LayoutFormulaContext() error = %v", err)
			}
			if box.Width != 64 || len(box.Lines) != 1 || box.Tag == nil {
				t.Fatalf("geometry = width:%v lines:%d tag:%+v", box.Width, len(box.Lines), box.Tag)
			}
			if !closeHost(box.Lines[0].OffsetX, tc.wantOffset) {
				t.Fatalf("OffsetX = %v, want %v", box.Lines[0].OffsetX, tc.wantOffset)
			}
			if !closeHost(box.Tag.X, 40) || !closeHost(box.Tag.X+box.Tag.Width, 64) {
				t.Fatalf("tag frame = x:%v right:%v, want x 40 right 64",
					box.Tag.X, box.Tag.X+box.Tag.Width)
			}
			untagged, err := renderer.Layout(glyph("x", ir.ClassNormal), ir.Display, style)
			if err != nil {
				t.Fatalf("untagged Layout() error = %v", err)
			}
			if box.Overflow != untagged.Overflow {
				t.Fatalf("Overflow = %v, untagged %v; reserved tag added no frame overflow",
					box.Overflow, untagged.Overflow)
			}
			if !closeHost(box.Items[0].Bound.X, tc.wantOffset) {
				t.Fatalf("formula item x = %v, want %v", box.Items[0].Bound.X, tc.wantOffset)
			}
			if err := box.Validate(); err != nil {
				t.Fatalf("Validate() error = %v", err)
			}
		})
	}

	t.Run("exact reserved width", func(t *testing.T) {
		style := testStyle(16)
		style.MaxWidth = 32
		style.Align = display.AlignEnd
		box, err := renderer.LayoutFormulaContext(context.Background(), formula, ir.Display, style)
		if err != nil {
			t.Fatalf("LayoutFormulaContext() error = %v", err)
		}
		untagged, err := renderer.Layout(glyph("x", ir.ClassNormal), ir.Display, style)
		if err != nil {
			t.Fatalf("untagged Layout() error = %v", err)
		}
		if box.Lines[0].OffsetX != 0 || box.Tag.X != 8 || box.Overflow != untagged.Overflow {
			t.Fatalf("exact geometry = offset:%v tag-x:%v overflow:%v, untagged overflow %v",
				box.Lines[0].OffsetX, box.Tag.X, box.Overflow, untagged.Overflow)
		}
	})

	t.Run("unbreakable overflow", func(t *testing.T) {
		style := testStyle(16)
		style.MaxWidth = 31
		style.Align = display.AlignEnd
		box, err := renderer.LayoutFormulaContext(context.Background(), formula, ir.Display, style)
		if err != nil {
			t.Fatalf("LayoutFormulaContext() error = %v", err)
		}
		if box.Lines[0].OffsetX != 0 || box.Tag.X != 8 || box.Overflow < 1 {
			t.Fatalf("overflow geometry = offset:%v tag-x:%v overflow:%v, want offset 0, tag-x 8, overflow >= 1",
				box.Lines[0].OffsetX, box.Tag.X, box.Overflow)
		}
	})

	t.Run("unconstrained unchanged", func(t *testing.T) {
		box, err := renderer.LayoutFormulaContext(context.Background(), formula, ir.Display, testStyle(16))
		if err != nil {
			t.Fatalf("LayoutFormulaContext() error = %v", err)
		}
		if len(box.Lines) != 0 || !closeHost(box.Tag.X, 8) || !closeHost(box.Width, 32) {
			t.Fatalf("unconstrained geometry = lines:%d tag-x:%v width:%v, want 0/8/32",
				len(box.Lines), box.Tag.X, box.Width)
		}
	})
}

func TestTagBecomesRightmostItalicRun(t *testing.T) {
	t.Parallel()

	renderer, glyphs, _ := newTestRenderer(t)
	rightParen, err := glyphs.Glyph(GlyphRequest{Text: ")"})
	if err != nil {
		t.Fatalf("Glyph()) error = %v", err)
	}
	rightParen.Italic = -em(1, 10)
	glyphs.glyphs[")"] = rightParen

	box, err := renderer.LayoutFormulaContext(context.Background(),
		taggedTestFormula(glyph("x", ir.ClassNormal), tagGlyph("1")),
		ir.Display, testStyle(16))
	if err != nil {
		t.Fatalf("LayoutFormulaContext() error = %v", err)
	}
	if !closeHost(box.Italic, -1.6) {
		t.Fatalf("parent Italic = %v, want rightmost tag run -1.6", box.Italic)
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

func TestEquationTagLimitsCountCanonicalStorageOnce(t *testing.T) {
	t.Parallel()

	formula := taggedTestFormula(glyph("x", ir.ClassNormal), tagGlyph("1"))
	fonts := Fonts{Glyphs: &fakeGlyphSource{glyphs: map[string]Glyph{}}, Metrics: fakeStyleFont{}}
	style := testStyle(16)
	style.MaxWidth = 64
	style.Align = display.AlignCenter
	baselineRenderer, err := New(fonts, Options{})
	if err != nil {
		t.Fatal(err)
	}
	baseline, err := baselineRenderer.LayoutFormulaContext(context.Background(), formula, ir.Display, style)
	if err != nil {
		t.Fatal(err)
	}
	if len(baseline.Lines) != 1 || baseline.Tag == nil {
		t.Fatalf("baseline tagged layout = lines:%d tag:%+v", len(baseline.Lines), baseline.Tag)
	}
	pathSegments := 0
	for _, item := range baseline.Items {
		if item.Kind == display.ItemGlyphPath {
			pathSegments += len(baseline.Paths[item.PathIndex].Segments)
		}
	}

	tests := []struct {
		name    string
		limits  source.Limits
		wantErr bool
	}{
		{name: "items exact", limits: source.Limits{MaxOutputItems: len(baseline.Items)}},
		{name: "items below", limits: source.Limits{MaxOutputItems: len(baseline.Items) - 1}, wantErr: true},
		{name: "paths exact", limits: source.Limits{MaxPathSegments: pathSegments}},
		{name: "paths below", limits: source.Limits{MaxPathSegments: pathSegments - 1}, wantErr: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			renderer, err := New(fonts, Options{Limits: test.limits})
			if err != nil {
				t.Fatal(err)
			}
			box, err := renderer.LayoutFormulaContext(context.Background(), formula, ir.Display, style)
			if test.wantErr {
				if !errors.Is(err, ErrLimit) || box != nil {
					t.Fatalf("LayoutFormulaContext() = %v, %v, want ErrLimit", box, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("LayoutFormulaContext() error = %v", err)
			}
			if len(box.Items) != len(baseline.Items) {
				t.Fatalf("items = %d, want %d", len(box.Items), len(baseline.Items))
			}
		})
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
			Line:   ir.FormulaTagLine,
		}},
	}
}
