package untie

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/caelyreth/untie/ir"
)

func TestExprCarriesImmutableEquationTags(t *testing.T) {
	t.Parallel()

	expr := ParseTeX(`x\tag{1}`, Display, ParseOptions{})
	for _, diagnostic := range expr.Diagnostics() {
		if diagnostic.Severity == SeverityError {
			t.Fatalf("unexpected parse diagnostic: %+v", diagnostic)
		}
	}
	tags := expr.Tags()
	if len(tags) != 1 || !tags[0].Manual || tags[0].Suppressed {
		t.Fatalf("Tags() = %+v, want one manual tag", tags)
	}
	if got := tags[0].Body.NodeBase().Span; got != (Range{Start: 5, End: 8}) {
		t.Fatalf("tag body span = %v, want [5,8)", got)
	}
	tags[0].Body.(*ir.Row).Items[0].(*ir.Glyph).Text = "changed"
	again := expr.Tags()
	if got := again[0].Body.(*ir.Row).Items[0].(*ir.Glyph).Text; got != "1" {
		t.Fatalf("mutating Tags() changed expression tag body to %q", got)
	}
	snapshot, err := expr.RootSnapshot()
	if err != nil {
		t.Fatalf("RootSnapshot() error = %v", err)
	}
	if len(snapshot.Tags) != 1 {
		t.Fatalf("RootSnapshot().Tags = %+v, want one", snapshot.Tags)
	}
	if plain := ParseTeX(`x`, Display, ParseOptions{}); expr.Key() == plain.Key() {
		t.Fatal("Expr.Key() did not include source tag metadata")
	}
}

func TestEquationTagLayoutAndAltContract(t *testing.T) {
	t.Parallel()

	fonts := loadTestFonts(t)
	engine, err := NewEngine(fonts, Limits{})
	if err != nil {
		t.Fatalf("NewEngine() error = %v", err)
	}
	expr := ParseTeX(`x\tag{1}`, Display, ParseOptions{})
	box, err := engine.LayoutContext(context.Background(), expr, Style{Size: 16, GlyphRefs: true})
	if err != nil {
		t.Fatalf("LayoutContext() error = %v", err)
	}
	if box.Tag == nil || box.Tag.Box == nil {
		t.Fatal("Box.Tag = nil")
	}
	if !box.Tag.Manual || box.Tag.Starred || box.Tag.Line != 0 {
		t.Fatalf("TagBox metadata = %+v", box.Tag)
	}
	if box.Tag.X <= 0 || box.Tag.Box.Width <= 0 || len(box.Tag.Box.Items) == 0 {
		t.Fatalf("TagBox geometry = %+v", box.Tag)
	}
	if len(box.Lines) != 0 || box.Tag.Y != 0 {
		t.Fatalf("unexpected unconstrained tag line metadata: lines=%+v tag=%+v", box.Lines, box.Tag)
	}
	if !tagItemsAreAggregatedAtEnd(box) {
		t.Fatal("parent Items do not contain the translated tag aggregate")
	}
	if box.Alt != expr.Alt() {
		t.Fatalf("Box.Alt = %#v, want %#v", box.Alt, expr.Alt())
	}
	if box.Alt.Text != "x equation (1)" || box.Alt.Speech != "x equation 1" {
		t.Fatalf("tag alt = %#v", box.Alt)
	}
	canonical, err := box.MarshalCanonical()
	if err != nil {
		t.Fatalf("MarshalCanonical() error = %v", err)
	}
	if !strings.Contains(string(canonical), `"tag":{`) {
		t.Fatalf("canonical box omitted tag metadata: %s", canonical)
	}
	clone := box.Clone()
	clone.Tag.Box.Items[0].At.X++
	if box.Tag.Box.Items[0].At.X == clone.Tag.Box.Items[0].At.X {
		t.Fatal("Box.Clone shared tag box storage")
	}
}

func TestEquationTagConstrainedAlignmentAndOverflow(t *testing.T) {
	t.Parallel()

	fonts := loadTestFonts(t)
	engine, err := NewEngine(fonts, Limits{})
	if err != nil {
		t.Fatalf("NewEngine() error = %v", err)
	}
	expr := ParseTeX(`x\tag{1}`, Display, ParseOptions{})

	fitting, err := engine.LayoutContext(context.Background(), expr, Style{
		Size: 16, MaxWidth: 120, Align: AlignCenter, GlyphRefs: true,
	})
	if err != nil {
		t.Fatalf("LayoutContext(fitting) error = %v", err)
	}
	if len(fitting.Lines) != 1 || fitting.Lines[0].Tag == nil {
		t.Fatalf("constrained lines/tag = %+v", fitting.Lines)
	}
	if fitting.Width != 120 || !closeFloat(fitting.Lines[0].Tag.X+fitting.Tag.Box.Width, 120) {
		t.Fatalf("tag right edge = %v, parent width = %v", fitting.Lines[0].Tag.X+fitting.Tag.Box.Width, fitting.Width)
	}
	if fitting.Lines[0].Tag.Source != fitting.Tag.Source ||
		fitting.Lines[0].Tag.X != fitting.Tag.X ||
		fitting.Lines[0].Tag.Y != fitting.Tag.Y {
		t.Fatalf("line tag metadata = %+v, parent tag = %+v", fitting.Lines[0].Tag, fitting.Tag)
	}
	if fitting.Overflow != 0 {
		t.Fatalf("fitting tag overflow = %v, want zero", fitting.Overflow)
	}

	overfull, err := engine.LayoutContext(context.Background(), expr, Style{
		Size: 16, MaxWidth: 1, Align: AlignEnd, GlyphRefs: true,
	})
	if err != nil {
		t.Fatalf("LayoutContext(overfull) error = %v", err)
	}
	if overfull.Overflow <= 0 {
		t.Fatalf("overfull tag Overflow = %v, want positive", overfull.Overflow)
	}
	if overfull.Tag == nil || overfull.Tag.X <= 0 {
		t.Fatalf("overfull tag placement = %+v", overfull.Tag)
	}
}

func TestEquationTagSuppressionDoesNotRender(t *testing.T) {
	t.Parallel()

	fonts := loadTestFonts(t)
	engine, err := NewEngine(fonts, Limits{})
	if err != nil {
		t.Fatalf("NewEngine() error = %v", err)
	}
	expr := ParseTeX(`\nonumber x`, Display, ParseOptions{})
	tags := expr.Tags()
	if len(tags) != 1 || !tags[0].Suppressed || tags[0].Body != nil {
		t.Fatalf("Tags() = %+v, want suppression event", tags)
	}
	box, err := engine.LayoutContext(context.Background(), expr, Style{Size: 16})
	if err != nil {
		t.Fatalf("LayoutContext() error = %v", err)
	}
	if box.Tag != nil {
		t.Fatalf("Box.Tag = %+v, want none", box.Tag)
	}
	if box.Alt.Text != "x" {
		t.Fatalf("Alt.Text = %q, want x", box.Alt.Text)
	}
}

func TestEquationTagLayoutRejectsInlineAndMultiline(t *testing.T) {
	t.Parallel()

	fonts := loadTestFonts(t)
	engine, err := NewEngine(fonts, Limits{})
	if err != nil {
		t.Fatalf("NewEngine() error = %v", err)
	}
	inline := ParseTeX(`x`, Inline, ParseOptions{})
	inline.root.Tags = []ir.EquationTag{{
		Body:   ir.NewGlyph(Range{Start: 0, End: 1}, "1"),
		Span:   Range{Start: 0, End: 1},
		Origin: Range{Start: 0, End: 1},
		Manual: true,
		Line:   0,
	}}
	if _, err := engine.Layout(inline, Style{Size: 16}); !errors.Is(err, ErrInvalidStyle) && !strings.Contains(err.Error(), "display mode") {
		t.Fatalf("inline tag layout error = %v, want display-mode rejection", err)
	}

	multiline := ParseTeX(`\begin{aligned}x&=1\end{aligned}\tag{2}`, Display, ParseOptions{})
	if tags := multiline.Tags(); len(tags) != 0 {
		t.Fatalf("multiline parser tags = %+v, want deferred/discarded", tags)
	}
}

func tagItemsAreAggregatedAtEnd(box *Box) bool {
	if box == nil || box.Tag == nil || box.Tag.Box == nil ||
		len(box.Tag.Box.Items) == 0 || len(box.Items) < len(box.Tag.Box.Items) {
		return false
	}
	offset := len(box.Items) - len(box.Tag.Box.Items)
	for i, item := range box.Tag.Box.Items {
		if !closeFloat(box.Items[offset+i].Bound.X, item.Bound.X+box.Tag.X) ||
			!closeFloat(box.Items[offset+i].Bound.Y, item.Bound.Y+box.Tag.Y) {
			return false
		}
	}
	return true
}

func closeFloat(got, want float32) bool {
	diff := got - want
	if diff < 0 {
		diff = -diff
	}
	return diff <= 1.0/1024
}
