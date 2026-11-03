package layout

import (
	"errors"
	"testing"

	"github.com/caelyreth/untie/display"
	"github.com/caelyreth/untie/geometry"
	"github.com/caelyreth/untie/identity"
	"github.com/caelyreth/untie/ir"
	"github.com/caelyreth/untie/source"
)

func TestInlineBoxSplitPreservesSemantics(t *testing.T) {
	t.Parallel()

	renderer, _, _ := newTestRenderer(t)
	a := glyph("a", ir.ClassNormal)
	a.Span = source.NewRange(0, 1)
	a.Origin = a.Span
	a.NodeID = identity.NodeID(81)
	b := glyph("b", ir.ClassNormal)
	b.Span = source.NewRange(2, 3)
	b.Origin = b.Span
	b.NodeID = identity.NodeID(82)
	breakNode := ir.NewBreak(source.NewRange(1, 2), ir.BreakExplicit)
	root := ir.NewRow(source.NewRange(0, 3), a, breakNode, b)
	root.NodeID = identity.NodeID(80)

	box, err := renderer.Layout(root, ir.Inline, testStyle(16))
	if err != nil {
		t.Fatalf("Layout() error = %v", err)
	}
	if len(box.Breaks) != 1 {
		t.Fatalf("breaks = %#v, want one", box.Breaks)
	}
	before, err := box.MarshalCanonical()
	if err != nil {
		t.Fatalf("MarshalCanonical() error = %v", err)
	}
	left, right, err := box.Split(box.Breaks[0])
	if err != nil {
		t.Fatalf("Split() error = %v", err)
	}
	after, err := box.MarshalCanonical()
	if err != nil {
		t.Fatalf("MarshalCanonical(after split) error = %v", err)
	}
	if string(before) != string(after) {
		t.Fatal("Split mutated the source box")
	}
	if left.Width != 8 || right.Width != 8 {
		t.Fatalf("split widths = %v/%v, want 8/8", left.Width, right.Width)
	}
	if len(left.Items) != 1 || left.Items[0].NodeID != 81 {
		t.Fatalf("left items = %#v", left.Items)
	}
	if len(right.Items) != 1 || right.Items[0].NodeID != 82 || right.Items[0].At.X != 0 {
		t.Fatalf("right items = %#v", right.Items)
	}
	if len(left.Breaks) != 0 || len(right.Breaks) != 0 {
		t.Fatalf("split breaks = %#v / %#v, want none", left.Breaks, right.Breaks)
	}
	for _, offset := range []int{0, 1, 2} {
		if _, ok := caretExactlyAt(left.Carets(), offset); !ok {
			t.Fatalf("left caret %d missing: %#v", offset, left.Carets())
		}
	}
	for _, offset := range []int{2, 3} {
		if _, ok := caretExactlyAt(right.Carets(), offset); !ok {
			t.Fatalf("right caret %d missing: %#v", offset, right.Carets())
		}
	}
	boundary, ok := caretExactlyAt(left.Carets(), 2)
	if !ok || boundary.X != 8 {
		t.Fatalf("left boundary caret = %#v, %v", boundary, ok)
	}
	boundary, ok = caretExactlyAt(right.Carets(), 2)
	if !ok || boundary.X != 0 {
		t.Fatalf("right boundary caret = %#v, %v", boundary, ok)
	}
	if areas := left.Covering(a.Span); len(areas) == 0 {
		t.Fatal("left Covering() lost the first atom")
	}
	if areas := right.Covering(b.Span); len(areas) == 0 {
		t.Fatal("right Covering() lost the second atom")
	}
	if _, ok := right.At(geometry.Point{X: 4}); !ok {
		t.Fatal("right At() lost the translated atom")
	}

	pathIndex := left.Items[0].PathIndex
	left.Paths[0].Segments[0].Op = 99
	if box.Paths[pathIndex].Segments[0].Op == 99 {
		t.Fatal("Split shared path storage with the source box")
	}
}

func TestBoxSplitRejectsConstrainedAndInvalidBreaks(t *testing.T) {
	t.Parallel()

	breakPoint := display.Break{X: 1, Kind: display.BreakExplicit}
	for _, box := range []*display.Box{
		{Lines: []display.LineBox{{}}, Breaks: []display.Break{breakPoint}},
		{Tag: &display.TagBox{}, Breaks: []display.Break{breakPoint}},
	} {
		if _, _, err := box.Split(breakPoint); !errors.Is(err, display.ErrCannotSplit) {
			t.Fatalf("Split(%#v) error = %v, want ErrCannotSplit", box, err)
		}
	}
	endpointBox := &display.Box{
		Width:  2,
		Breaks: []display.Break{{X: 2, Kind: display.BreakExplicit}},
	}
	if _, _, err := endpointBox.Split(endpointBox.Breaks[0]); !errors.Is(err, display.ErrCannotSplit) {
		t.Fatalf("Split(endpoint) error = %v, want ErrCannotSplit", err)
	}
	box := &display.Box{Width: 2}
	if _, _, err := box.Split(breakPoint); !errors.Is(err, display.ErrCannotSplit) {
		t.Fatalf("Split(unowned break) error = %v, want ErrCannotSplit", err)
	}
}

func TestConstrainedDisplayBoxRejectsInlineSplit(t *testing.T) {
	t.Parallel()

	renderer, _, _ := newTestRenderer(t)
	root := ir.NewRow(source.NewRange(0, 3),
		glyph("a", ir.ClassNormal),
		ir.NewBreak(source.NewRange(0, 1), ir.BreakExplicit),
		glyph("b", ir.ClassNormal),
	)
	style := testStyle(16)
	style.MaxWidth = 9
	box, err := renderer.Layout(root, ir.Display, style)
	if err != nil {
		t.Fatalf("Layout() error = %v", err)
	}
	if len(box.Lines) < 2 {
		t.Fatalf("lines = %d, want wrapping", len(box.Lines))
	}
	if _, _, err := box.Split(display.Break{X: 8, Kind: display.BreakExplicit}); !errors.Is(err, display.ErrCannotSplit) {
		t.Fatalf("Split(constrained) error = %v, want ErrCannotSplit", err)
	}
}
