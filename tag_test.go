package untie

import (
	"context"
	"strings"
	"testing"

	"github.com/caelyreth/untie/ir"
)

func TestFormulaTagLinePublicContract(t *testing.T) {
	t.Parallel()

	if FormulaTagLine != ir.FormulaTagLine {
		t.Fatalf("public FormulaTagLine = %d, neutral = %d", FormulaTagLine, ir.FormulaTagLine)
	}
}

func TestExprCarriesImmutableEquationTags(t *testing.T) {
	t.Parallel()

	expr := ParseTeX(`x\tag{1}`, Display, ParseOptions{})
	for _, diagnostic := range expr.Diagnostics() {
		if diagnostic.Severity == SeverityError {
			t.Fatalf("unexpected parse diagnostic: %+v", diagnostic)
		}
	}
	tags := mustRoot(t, expr).Tags
	if len(tags) != 1 || !tags[0].Manual || tags[0].Suppressed {
		t.Fatalf("Tags() = %+v, want one manual tag", tags)
	}
	if got := tags[0].Body.NodeBase().Span; got != (Range{Start: 5, End: 8}) {
		t.Fatalf("tag body span = %v, want [5,8)", got)
	}
	tags[0].Body.(*ir.Row).Items[0].(*ir.Glyph).Text = "changed"
	again := mustRoot(t, expr).Tags
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
	box, err := engine.LayoutContext(context.Background(), expr, Style{Size: 16, GlyphRefs: true, GenerateAlt: true})
	if err != nil {
		t.Fatalf("LayoutContext() error = %v", err)
	}
	if box.Tag == nil || box.Tag.Items.Count == 0 {
		t.Fatal("Box.Tag = nil")
	}
	if !box.Tag.Manual || box.Tag.Starred || box.Tag.Line != FormulaTagLine {
		t.Fatalf("TagBox metadata = %+v", box.Tag)
	}
	if box.Tag.X <= 0 || box.Tag.Width <= 0 || box.Tag.Items.Count == 0 {
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
	clone.Items[clone.Tag.Items.Start].At.X++
	if box.Items[box.Tag.Items.Start].At.X == clone.Items[clone.Tag.Items.Start].At.X {
		t.Fatal("Box.Clone shared tag item storage")
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
	if fitting.Width != 120 || !closeFloat(fitting.Lines[0].Tag.X+fitting.Tag.Width, 120) {
		t.Fatalf("tag right edge = %v, parent width = %v", fitting.Lines[0].Tag.X+fitting.Tag.Width, fitting.Width)
	}
	if fitting.Lines[0].Tag.Source != fitting.Tag.Source ||
		fitting.Lines[0].Tag.X != fitting.Tag.X ||
		fitting.Lines[0].Tag.Y != fitting.Tag.Y {
		t.Fatalf("line tag metadata = %+v, parent tag = %+v", fitting.Lines[0].Tag, fitting.Tag)
	}
	if fitting.Overflow != 0 {
		t.Fatalf("fitting tag overflow = %v, want zero", fitting.Overflow)
	}

	end, err := engine.LayoutContext(context.Background(), expr, Style{
		Size: 16, MaxWidth: 120, Align: AlignEnd, GlyphRefs: true,
	})
	if err != nil {
		t.Fatalf("LayoutContext(end) error = %v", err)
	}
	if end.Overflow != 0 {
		t.Fatalf("end-aligned tag overflow = %v, want zero", end.Overflow)
	}
	if !closeFloat(end.Tag.X+end.Tag.Width, 120) {
		t.Fatalf("end-aligned tag right edge = %v, want 120", end.Tag.X+end.Tag.Width)
	}
	if !closeFloat(end.Lines[0].OffsetX+end.Lines[0].Width, end.Tag.X) {
		t.Fatalf("end-aligned formula right edge = %v, want tag left edge %v",
			end.Lines[0].OffsetX+end.Lines[0].Width, end.Tag.X)
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
	tags := mustRoot(t, expr).Tags
	if len(tags) != 1 || !tags[0].Suppressed || tags[0].Body != nil {
		t.Fatalf("Tags() = %+v, want suppression event", tags)
	}
	box, err := engine.LayoutContext(context.Background(), expr, Style{Size: 16, GenerateAlt: true})
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

func TestEquationTagLayoutRejectsInlineButAllowsNestedMultilineValues(t *testing.T) {
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
		Line:   FormulaTagLine,
	}}
	if _, err := engine.LayoutContextWithOptions(
		context.Background(),
		inline,
		Style{Size: 16},
		LayoutOptions{Recovery: RecoveryStrict},
	); err == nil || !strings.Contains(err.Error(), "display mode") {
		t.Fatalf("strict inline tag layout error = %v, want display-mode rejection", err)
	}
	inlineBox, err := engine.Layout(inline, Style{Size: 16})
	if err != nil {
		t.Fatalf("visible inline tag layout error = %v", err)
	}
	if inlineBox.Tag != nil || len(inlineBox.Diagnostics) != 1 ||
		inlineBox.Diagnostics[0].Code != CodeUnsupported {
		t.Fatalf("visible inline tag = tag:%+v diagnostics:%+v", inlineBox.Tag, inlineBox.Diagnostics)
	}

	nested := ParseTeX(`x+\substack{a\\b}\tag{2}`, Display, ParseOptions{})
	if tags := mustRoot(t, nested).Tags; len(tags) != 1 {
		t.Fatalf("nested multiline tags = %+v, want top-level tag retained", tags)
	}
	box, err := engine.Layout(nested, Style{Size: 16, MaxWidth: 1, Align: AlignCenter})
	if err != nil {
		t.Fatalf("nested multiline tag layout error = %v", err)
	}
	if box.Tag == nil || len(box.Lines) != 1 || box.Overflow <= 0 {
		t.Fatalf("nested multiline constrained fallback = tag:%+v lines:%d overflow:%v", box.Tag, len(box.Lines), box.Overflow)
	}
}

func tagItemsAreAggregatedAtEnd(box *Box) bool {
	if box == nil || box.Tag == nil || box.Tag.Items.Count == 0 {
		return false
	}
	if box.Tag.Items.Start != len(box.Items)-box.Tag.Items.Count ||
		box.Tag.Hits.Start != len(box.Hits)-box.Tag.Hits.Count {
		return false
	}
	for _, item := range box.Items[box.Tag.Items.Start:box.Tag.Items.End()] {
		if item.Bound.X < box.Tag.X || item.Bound.Y < box.Tag.Y-box.Tag.Ascent ||
			item.Bound.Y+item.Bound.Height > box.Tag.Y+box.Tag.Descent {
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
