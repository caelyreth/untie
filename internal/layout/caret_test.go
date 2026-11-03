package layout

import (
	"context"
	"testing"

	"github.com/caelyreth/untie/display"
	"github.com/caelyreth/untie/identity"
	"github.com/caelyreth/untie/ir"
	"github.com/caelyreth/untie/source"
)

func TestCaretStopsFollowRTLSourceOrder(t *testing.T) {
	t.Parallel()

	renderer := newTextClusterRenderer(t, visualClusterTextSource{
		sources: []source.Range{
			source.NewRange(3, 4),
			source.NewRange(2, 3),
			source.NewRange(1, 2),
			source.NewRange(0, 1),
		},
		rtl: []bool{true, true, true, true},
	})
	text := ir.NewText(source.NewRange(0, 4), "سلام")
	text.NodeID = identity.NodeID(71)
	text.Clusters = []ir.TextClusterSource{
		{TextStart: 0, TextEnd: 2, Source: source.NewRange(0, 1)},
		{TextStart: 2, TextEnd: 4, Source: source.NewRange(1, 2)},
		{TextStart: 4, TextEnd: 6, Source: source.NewRange(2, 3)},
		{TextStart: 6, TextEnd: 8, Source: source.NewRange(3, 4)},
	}

	box, err := renderer.Layout(text, ir.Inline, testStyle(16))
	if err != nil {
		t.Fatalf("Layout() error = %v", err)
	}
	carets := box.Carets()
	for i := 1; i < len(carets); i++ {
		if carets[i-1].Source > carets[i].Source {
			t.Fatalf("carets are not source ordered: %#v", carets)
		}
	}
	wantX := map[int]float32{0: 32, 1: 24, 2: 16, 4: 0}
	for sourceOffset, want := range wantX {
		caret, ok := caretExactlyAt(carets, sourceOffset)
		if !ok || !closeHost(caret.X, want) {
			t.Fatalf("CaretAt(%d) = %#v, %v; want x=%v in %#v", sourceOffset, caret, ok, want, carets)
		}
	}
	stops := caretsAt(carets, 3)
	if len(stops) != 2 || !closeHost(stops[0].X, 8) || !closeHost(stops[1].X, 8) {
		t.Fatalf("offset 3 stops = %#v, want adjacent-cluster meeting at x=8", stops)
	}
	for _, caret := range carets {
		if caret.NodeID != text.NodeID {
			t.Fatalf("caret NodeID = %d, want %d", caret.NodeID, text.NodeID)
		}
	}
}

func TestCaretStopsCoverMissingGlyphPlaceholder(t *testing.T) {
	t.Parallel()

	renderer := newTextClusterRenderer(t, visualClusterTextSource{
		sources: []source.Range{source.NewRange(40, 44)},
		missing: true,
	})
	text := ir.NewText(source.NewRange(0, 4), "😀")
	text.NodeID = identity.NodeID(72)
	text.Clusters = []ir.TextClusterSource{{
		TextStart: 0,
		TextEnd:   4,
		Source:    source.NewRange(40, 44),
	}}
	box, err := renderer.LayoutFormulaContextOptions(
		context.Background(),
		&ir.Formula{Root: text},
		ir.Inline,
		testStyle(16),
		LayoutOptions{RecoverMissingGlyph: true},
	)
	if err != nil {
		t.Fatalf("LayoutFormulaContextOptions() error = %v", err)
	}
	carets := box.Carets()
	start, ok := caretExactlyAt(carets, 40)
	if !ok || !closeHost(start.X, 0) {
		t.Fatalf("placeholder start caret = %#v, %v", start, ok)
	}
	end, ok := caretExactlyAt(carets, 44)
	if !ok || !closeHost(end.X, 8) {
		t.Fatalf("placeholder end caret = %#v, %v", end, ok)
	}
	if start.Top > start.Bottom || !closeHost(start.Top, end.Top) || !closeHost(start.Bottom, end.Bottom) {
		t.Fatalf("placeholder caret extents = %#v / %#v", start, end)
	}
}

func TestCaretStopsCoverScriptsAndFractions(t *testing.T) {
	t.Parallel()

	renderer, _, _ := newTestRenderer(t)
	base := glyph("x", ir.ClassNormal)
	base.Span = source.NewRange(0, 1)
	base.Origin = base.Span
	superscript := glyph("2", ir.ClassNormal)
	superscript.Span = source.NewRange(2, 3)
	superscript.Origin = superscript.Span
	scripts := ir.NewScripts(source.NewRange(0, 3), base)
	scripts.TopRight = superscript

	box, err := renderer.Layout(scripts, ir.Inline, testStyle(16))
	if err != nil {
		t.Fatalf("Layout(scripts) error = %v", err)
	}
	for _, offset := range []int{0, 1, 2, 3} {
		if _, ok := caretExactlyAt(box.Carets(), offset); !ok {
			t.Fatalf("script caret for offset %d missing: %#v", offset, box.Carets())
		}
	}

	numerator := glyph("1", ir.ClassNormal)
	numerator.Span = source.NewRange(1, 2)
	numerator.Origin = numerator.Span
	denominator := glyph("2", ir.ClassNormal)
	denominator.Span = source.NewRange(3, 4)
	denominator.Origin = denominator.Span
	fraction := ir.NewFraction(source.NewRange(0, 5), numerator, denominator)
	box, err = renderer.Layout(fraction, ir.Inline, testStyle(20))
	if err != nil {
		t.Fatalf("Layout(fraction) error = %v", err)
	}
	for _, offset := range []int{1, 2, 3, 4} {
		if _, ok := caretExactlyAt(box.Carets(), offset); !ok {
			t.Fatalf("fraction caret for offset %d missing: %#v", offset, box.Carets())
		}
	}
	numCaret, _ := caretExactlyAt(box.Carets(), 1)
	denCaret, _ := caretExactlyAt(box.Carets(), 3)
	if numCaret.Top >= denCaret.Top {
		t.Fatalf("fraction numerator/denominator caret extents are not distinct: %#v / %#v", numCaret, denCaret)
	}
}

func TestCaretStopsFollowAlignedRows(t *testing.T) {
	t.Parallel()

	renderer, _, _ := newTestRenderer(t)
	x := glyph("x", ir.ClassNormal)
	x.Span = source.NewRange(0, 1)
	x.Origin = x.Span
	y := glyph("y", ir.ClassNormal)
	y.Span = source.NewRange(1, 2)
	y.Origin = y.Span
	z := glyph("z", ir.ClassNormal)
	z.Span = source.NewRange(2, 3)
	z.Origin = z.Span
	aligned := ir.NewMultiline(source.NewRange(0, 3), []ir.Node{x, y}, []ir.Node{z})

	box, err := renderer.Layout(aligned, ir.Inline, testStyle(16))
	if err != nil {
		t.Fatalf("Layout() error = %v", err)
	}
	for _, offset := range []int{0, 1, 2, 3} {
		if _, ok := caretExactlyAt(box.Carets(), offset); !ok {
			t.Fatalf("aligned caret for offset %d missing: %#v", offset, box.Carets())
		}
	}
}

func TestCaretStopsRemainLineAwareInConstrainedDisplay(t *testing.T) {
	t.Parallel()

	renderer, _, _ := newTestRenderer(t)
	a := glyph("a", ir.ClassNormal)
	a.Span = source.NewRange(0, 1)
	a.Origin = a.Span
	b := glyph("b", ir.ClassNormal)
	b.Span = source.NewRange(2, 3)
	b.Origin = b.Span
	c := glyph("c", ir.ClassNormal)
	c.Span = source.NewRange(4, 5)
	c.Origin = c.Span
	breakOne := ir.NewBreak(source.NewRange(1, 2), ir.BreakExplicit)
	breakTwo := ir.NewBreak(source.NewRange(3, 4), ir.BreakExplicit)
	root := ir.NewRow(source.NewRange(0, 5), a, breakOne, b, breakTwo, c)
	style := testStyle(16)
	style.MaxWidth = 9

	box, err := renderer.Layout(root, ir.Display, style)
	if err != nil {
		t.Fatalf("Layout() error = %v", err)
	}
	if len(box.Lines) < 2 {
		t.Fatalf("lines = %d, want constrained wrapping", len(box.Lines))
	}
	for _, line := range box.Lines {
		found := false
		for _, caret := range box.Carets() {
			if caret.Top >= line.Baseline-line.Ascent-1e-5 &&
				caret.Bottom <= line.Baseline+line.Descent+1e-5 {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("line %#v has no caret stop; carets=%#v", line, box.Carets())
		}
	}
	for _, offset := range []int{0, 1, 2, 3, 4, 5} {
		if _, ok := caretExactlyAt(box.Carets(), offset); !ok {
			t.Fatalf("display caret for offset %d missing: %#v", offset, box.Carets())
		}
	}
}

func caretExactlyAt(carets []display.CaretStop, sourceOffset int) (display.CaretStop, bool) {
	stops := caretsAt(carets, sourceOffset)
	if len(stops) == 0 {
		return display.CaretStop{}, false
	}
	return stops[0], true
}

func caretsAt(carets []display.CaretStop, sourceOffset int) []display.CaretStop {
	var out []display.CaretStop
	for _, caret := range carets {
		if caret.Source == sourceOffset {
			out = append(out, caret)
		}
	}
	return out
}
