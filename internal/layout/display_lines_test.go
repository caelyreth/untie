package layout

import (
	"context"
	"errors"
	"testing"

	"github.com/caelyreth/untie/display"
	"github.com/caelyreth/untie/geometry"
	"github.com/caelyreth/untie/ir"
	"github.com/caelyreth/untie/source"
)

func TestDisplayLinesWrapAtNeutralBreaks(t *testing.T) {
	t.Parallel()

	renderer, _, _ := newTestRenderer(t)
	root := ir.NewRow(source.NewRange(0, 7),
		glyph("a", ir.ClassNormal),
		glyph("+", ir.ClassBinary),
		glyph("b", ir.ClassNormal),
		glyph("+", ir.ClassBinary),
		glyph("c", ir.ClassNormal),
		glyph("+", ir.ClassBinary),
		glyph("ddddddddddd", ir.ClassNormal),
	)
	unwrapped, err := renderer.Layout(root, ir.Display, testStyle(16))
	if err != nil {
		t.Fatal(err)
	}
	inline, err := renderer.Layout(root, ir.Inline, testStyle(16))
	if err != nil {
		t.Fatal(err)
	}
	if len(inline.Breaks) != 3 {
		t.Fatalf("inline breaks = %+v, want three", inline.Breaks)
	}
	style := testStyle(16)
	style.MaxWidth = inline.Breaks[0].X
	box, err := renderer.Layout(root, ir.Display, style)
	if err != nil {
		t.Fatal(err)
	}
	if len(box.Lines) != 4 {
		t.Fatalf("Lines = %d, want 4", len(box.Lines))
	}
	if len(box.Breaks) != 0 {
		t.Fatalf("display Breaks = %+v, want none", box.Breaks)
	}
	if err := box.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
	if box.Alt != unwrapped.Alt {
		t.Fatalf("parent Alt = %+v, want %+v", box.Alt, unwrapped.Alt)
	}

	wantWidths := []float32{
		inline.Breaks[0].X,
		inline.Breaks[1].X - inline.Breaks[0].X,
		inline.Breaks[2].X - inline.Breaks[1].X,
		unwrapped.Width - inline.Breaks[2].X,
	}
	itemCount := 0
	hitCount := 0
	for i, line := range box.Lines {
		if line.Box == nil {
			t.Fatalf("line %d has nil box", i)
		}
		if err := line.Box.Validate(); err != nil {
			t.Fatalf("line %d Validate() error = %v", i, err)
		}
		if len(line.Box.Lines) != 0 {
			t.Fatalf("line %d recursively owns lines", i)
		}
		if line.Box.Alt != (display.Alt{}) {
			t.Fatalf("line %d Alt = %+v, want empty", i, line.Box.Alt)
		}
		if !closeHost(line.Box.Width, wantWidths[i]) {
			t.Fatalf("line %d Width = %v, want %v", i, line.Box.Width, wantWidths[i])
		}
		if len(line.Box.Items) == 0 {
			t.Fatalf("line %d has no items", i)
		}
		if got := line.Box.Items[0].Bound.X; !closeHost(got, 0) {
			t.Fatalf("line %d first item x = %v, want 0", i, got)
		}
		if i == 0 && !closeHost(line.Box.Width, style.MaxWidth) {
			t.Fatalf("boundary-fitting line width = %v, want exact max %v", line.Box.Width, style.MaxWidth)
		}
		if i == len(box.Lines)-1 && line.Box.Overflow <= 0 {
			t.Fatal("overfull final line did not report overflow")
		}
		itemCount += len(line.Box.Items)
		hitCount += len(line.Box.Hits)
	}
	if itemCount != len(unwrapped.Items) {
		t.Fatalf("line items = %d, want %d", itemCount, len(unwrapped.Items))
	}
	if len(box.Items) != itemCount || len(box.Hits) != hitCount {
		t.Fatalf("parent aggregate = %d items/%d hits, want %d/%d", len(box.Items), len(box.Hits), itemCount, hitCount)
	}
	if box.Lines[0].Baseline != 0 {
		t.Fatalf("first baseline = %v, want 0", box.Lines[0].Baseline)
	}
	wantBaseline := box.Lines[0].Box.Descent + box.Lines[1].Box.Ascent
	if !closeHost(box.Lines[1].Baseline, wantBaseline) {
		t.Fatalf("second baseline = %v, want %v", box.Lines[1].Baseline, wantBaseline)
	}
	secondLineItem := box.Lines[1].Box.Items[0].Bound.Y
	parentItem := box.Items[len(box.Lines[0].Box.Items)].Bound.Y
	if got := parentItem - secondLineItem; !closeHost(got, box.Lines[1].Baseline) {
		t.Fatalf("second-line parent item offset = %v, want baseline %v", got, box.Lines[1].Baseline)
	}
	if !closeHost(box.Width, wantWidths[3]) {
		t.Fatalf("parent Width = %v, want widest line %v", box.Width, wantWidths[3])
	}
	if box.Overflow != box.Lines[3].Box.Overflow {
		t.Fatalf("parent Overflow = %v, want %v", box.Overflow, box.Lines[3].Box.Overflow)
	}
	firstCanonical, err := box.MarshalCanonical()
	if err != nil {
		t.Fatal(err)
	}
	clone := box.Clone()
	secondCanonical, err := clone.MarshalCanonical()
	if err != nil {
		t.Fatal(err)
	}
	if string(firstCanonical) != string(secondCanonical) {
		t.Fatal("clone changed line canonical output")
	}
	clone.Lines[0].Box.Items[0].Bound.X++
	if box.Lines[0].Box.Items[0].Bound.X == clone.Lines[0].Box.Items[0].Bound.X {
		t.Fatal("clone shared line item storage")
	}
}

func TestDisplayLinesLifecycleAndEdgeCases(t *testing.T) {
	t.Parallel()

	renderer, _, _ := newTestRenderer(t)
	root := ir.NewRow(source.NewRange(0, 3),
		glyph("x", ir.ClassNormal),
		glyph("=", ir.ClassRelation),
		glyph("y", ir.ClassNormal),
	)

	t.Run("unconstrained display", func(t *testing.T) {
		defaultStyle := testStyle(16)
		box, err := renderer.Layout(root, ir.Display, defaultStyle)
		if err != nil {
			t.Fatal(err)
		}
		if len(box.Lines) != 0 {
			t.Fatalf("Lines = %+v, want none", box.Lines)
		}
		zeroWidthStyle := defaultStyle
		zeroWidthStyle.MaxWidth = 0
		zeroWidth, err := renderer.Layout(root, ir.Display, zeroWidthStyle)
		if err != nil {
			t.Fatal(err)
		}
		first, _ := box.MarshalCanonical()
		second, _ := zeroWidth.MarshalCanonical()
		if string(first) != string(second) {
			t.Fatal("zero MaxWidth changed unconstrained display geometry")
		}
	})
	t.Run("inline ignores max width", func(t *testing.T) {
		base, err := renderer.Layout(root, ir.Inline, testStyle(16))
		if err != nil {
			t.Fatal(err)
		}
		style := testStyle(16)
		style.MaxWidth = 1
		box, err := renderer.Layout(root, ir.Inline, style)
		if err != nil {
			t.Fatal(err)
		}
		if len(box.Lines) != 0 {
			t.Fatalf("inline Lines = %+v, want none", box.Lines)
		}
		first, _ := base.MarshalCanonical()
		second, _ := box.MarshalCanonical()
		if string(first) != string(second) {
			t.Fatal("inline MaxWidth changed inline geometry")
		}
	})
	t.Run("one fitting line", func(t *testing.T) {
		style := testStyle(16)
		style.MaxWidth = 100
		box, err := renderer.Layout(root, ir.Display, style)
		if err != nil {
			t.Fatal(err)
		}
		if len(box.Lines) != 1 || box.Lines[0].Baseline != 0 {
			t.Fatalf("Lines = %+v, want one line at baseline 0", box.Lines)
		}
		if box.Lines[0].Box.Width != box.Width || box.Lines[0].Box.Overflow != box.Overflow {
			t.Fatalf("fitting line = %+v, parent width %v", box.Lines[0].Box, box.Width)
		}
	})
	t.Run("no legal break", func(t *testing.T) {
		noBreak := ir.NewRow(source.NewRange(0, 2),
			glyph("x", ir.ClassNormal),
			glyph("y", ir.ClassNormal),
		)
		style := testStyle(16)
		style.MaxWidth = 1
		box, err := renderer.Layout(noBreak, ir.Display, style)
		if err != nil {
			t.Fatal(err)
		}
		if len(box.Lines) != 1 {
			t.Fatalf("Lines = %+v, want one overfull line", box.Lines)
		}
		if box.Overflow <= 0 || box.Lines[0].Box.Overflow != box.Overflow {
			t.Fatalf("overflow = %v line overflow = %v", box.Overflow, box.Lines[0].Box.Overflow)
		}
	})
	t.Run("trailing explicit break", func(t *testing.T) {
		trailing := ir.NewRow(source.NewRange(0, 3),
			glyph("x", ir.ClassNormal),
			glyph("y", ir.ClassNormal),
			ir.NewBreak(source.NewRange(2, 3), ir.BreakExplicit, 0),
		)
		style := testStyle(16)
		style.MaxWidth = 1
		box, err := renderer.Layout(trailing, ir.Display, style)
		if err != nil {
			t.Fatal(err)
		}
		if len(box.Lines) != 1 {
			t.Fatalf("trailing break produced %d lines, want 1", len(box.Lines))
		}
	})
}

func TestDisplayLinesHandleTablesAndMultilineAsOverfull(t *testing.T) {
	t.Parallel()

	renderer, _, _ := newTestRenderer(t)
	tests := []struct {
		name string
		root ir.Node
	}{
		{
			name: "table",
			root: ir.NewTable(source.NewRange(0, 4),
				ir.TableRow{Cells: []ir.Node{glyph("a", ir.ClassNormal), glyph("b", ir.ClassNormal)}},
				ir.TableRow{Cells: []ir.Node{glyph("c", ir.ClassNormal), glyph("d", ir.ClassNormal)}},
			),
		},
		{
			name: "multiline",
			root: ir.NewMultiline(source.NewRange(0, 4),
				[]ir.Node{glyph("a", ir.ClassNormal), glyph("b", ir.ClassNormal)},
				[]ir.Node{glyph("c", ir.ClassNormal), glyph("d", ir.ClassNormal)},
			),
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			style := testStyle(16)
			style.MaxWidth = 1
			box, err := renderer.Layout(test.root, ir.Display, style)
			if err != nil {
				t.Fatal(err)
			}
			if len(box.Lines) != 1 {
				t.Fatalf("Lines = %d, want one unbreakable line", len(box.Lines))
			}
			if box.Overflow <= 0 || box.Lines[0].Box.Overflow != box.Overflow {
				t.Fatalf("overflow = %v line overflow = %v", box.Overflow, box.Lines[0].Box.Overflow)
			}
		})
	}
}

func TestDisplayLineLimitsAndCancellation(t *testing.T) {
	t.Parallel()

	root := ir.NewRow(source.NewRange(0, 3),
		glyph("x", ir.ClassNormal),
		glyph("+", ir.ClassBinary),
		glyph("y", ir.ClassNormal),
	)
	glyphs := &fakeGlyphSource{glyphs: map[string]Glyph{}}
	limited, err := New(Fonts{Glyphs: glyphs, Metrics: fakeStyleFont{}}, Options{
		Limits: source.Limits{MaxOutputItems: 4},
	})
	if err != nil {
		t.Fatal(err)
	}
	style := testStyle(16)
	style.MaxWidth = 100
	if _, err := limited.Layout(root, ir.Display, style); !errors.Is(err, ErrLimit) {
		t.Fatalf("line output limit error = %v, want ErrLimit", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	state := &layoutState{
		ctx:      ctx,
		request:  display.Style{MaxWidth: 1},
		hostSize: hostSubunits,
		limits:   source.Limits{}.WithDefaults(),
	}
	_, err = state.greedyLinePlans([]lineBoundary{
		{itemIndex: 0, x: 0},
		{itemIndex: 1, x: geometry.Em(geometry.EmScale)},
		{itemIndex: 2, x: geometry.Em(2 * geometry.EmScale)},
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("greedyLinePlans() error = %v, want context cancellation", err)
	}
}

func TestDisplayLineFragmentCycleIsRejected(t *testing.T) {
	t.Parallel()

	root := emptyFragment(Display, geometry.Em(geometry.EmScale))
	root.Width = geometry.Em(geometry.EmScale)
	root.Ascent = geometry.Em(geometry.EmScale)
	root.breaks = []fragmentBreak{{x: geometry.Em(geometry.EmScale / 2), kind: display.BreakExplicit}}
	if err := root.pushGroup(0, 0, root); err != nil {
		t.Fatal(err)
	}
	state := &layoutState{
		ctx:       context.Background(),
		mode:      ir.Display,
		request:   display.Style{MaxWidth: 1},
		rootStyle: Style{Size: Display},
		metrics:   testMetrics(),
		hostSize:  hostSubunits,
		limits:    source.Limits{}.WithDefaults(),
	}
	if _, err := state.flatten(root); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("flatten() error = %v, want invalid fragment cycle", err)
	}
}
