package layout

import (
	"context"
	"errors"
	"reflect"
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
		view := lineBoxForTest(box, line)
		if err := view.Validate(); err != nil {
			t.Fatalf("line %d Validate() error = %v", i, err)
		}
		if len(view.Lines) != 0 {
			t.Fatalf("line %d recursively owns lines", i)
		}
		if view.Alt != (display.Alt{}) {
			t.Fatalf("line %d Alt = %+v, want empty", i, view.Alt)
		}
		if !closeHost(line.Width, wantWidths[i]) {
			t.Fatalf("line %d Width = %v, want %v", i, line.Width, wantWidths[i])
		}
		if len(view.Items) == 0 {
			t.Fatalf("line %d has no items", i)
		}
		if line.Items.Count == 0 {
			t.Fatalf("line %d item range is empty", i)
		}
		if i == 0 && !closeHost(line.Width, style.MaxWidth) {
			t.Fatalf("boundary-fitting line width = %v, want exact max %v", line.Width, style.MaxWidth)
		}
		if i == len(box.Lines)-1 && line.Overflow <= 0 {
			t.Fatal("overfull final line did not report overflow")
		}
		itemCount += line.Items.Count
		hitCount += line.Hits.Count
	}
	if itemCount != len(unwrapped.Items) {
		t.Fatalf("line items = %d, want %d", itemCount, len(unwrapped.Items))
	}
	if len(box.Items) != itemCount || len(box.Hits) != hitCount {
		t.Fatalf("parent aggregate = %d items/%d hits, want %d/%d", len(box.Items), len(box.Hits), itemCount, hitCount)
	}
	for i, line := range box.Lines {
		foundRoot := false
		for _, hit := range box.Hits[line.Hits.Start:line.Hits.End()] {
			if hit.Kind == display.HitRow && hit.Source == root.Origin {
				foundRoot = true
				break
			}
		}
		if !foundRoot {
			t.Fatalf("line %d has no row hit for root source %v", i, root.Origin)
		}
	}
	resolvedRow := false
	for _, hit := range box.Hits {
		if hit.Kind != display.HitRow || hit.Source != root.Origin {
			continue
		}
		for xi := 1; xi < 10 && !resolvedRow; xi++ {
			for yi := 1; yi < 10 && !resolvedRow; yi++ {
				point := geometry.Point{
					X: hit.Area.X + hit.Area.Width*float32(xi)/10,
					Y: hit.Area.Y + hit.Area.Height*float32(yi)/10,
				}
				got, ok := box.At(point)
				resolvedRow = ok && got.Kind == display.HitRow && got.Source == root.Origin
			}
		}
	}
	if !resolvedRow {
		t.Fatal("Box.At did not resolve an inter-atom point to the wrapped row hit")
	}
	if box.Lines[0].Baseline != 0 {
		t.Fatalf("first baseline = %v, want 0", box.Lines[0].Baseline)
	}
	wantBaseline := box.Lines[0].Descent + box.Lines[1].Ascent
	if !closeHost(box.Lines[1].Baseline, wantBaseline) {
		t.Fatalf("second baseline = %v, want %v", box.Lines[1].Baseline, wantBaseline)
	}
	secondLineItem := lineBoxForTest(box, box.Lines[1]).Items[0].Bound.Y
	parentItem := box.Items[box.Lines[1].Items.Start].Bound.Y
	if got := parentItem - secondLineItem; !closeHost(got, box.Lines[1].Baseline) {
		t.Fatalf("second-line parent item offset = %v, want baseline %v", got, box.Lines[1].Baseline)
	}
	if !closeHost(box.Width, style.MaxWidth) {
		t.Fatalf("parent Width = %v, want constrained frame %v", box.Width, style.MaxWidth)
	}
	if box.Overflow != box.Lines[3].Overflow {
		t.Fatalf("parent Overflow = %v, want %v", box.Overflow, box.Lines[3].Overflow)
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
	clone.Items[clone.Lines[0].Items.Start].Bound.X++
	if box.Items[box.Lines[0].Items.Start].Bound.X == clone.Items[clone.Lines[0].Items.Start].Bound.X {
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
		for _, align := range []display.Align{display.AlignCenter, display.AlignEnd} {
			alignedStyle := zeroWidthStyle
			alignedStyle.Align = align
			aligned, err := renderer.Layout(root, ir.Display, alignedStyle)
			if err != nil {
				t.Fatal(err)
			}
			alignedCanonical, _ := aligned.MarshalCanonical()
			if string(first) != string(alignedCanonical) {
				t.Fatalf("Align %d changed unconstrained display geometry", align)
			}
		}
	})
	t.Run("inline ignores max width and align", func(t *testing.T) {
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
		for _, align := range []display.Align{display.AlignCenter, display.AlignEnd} {
			alignedStyle := style
			alignedStyle.Align = align
			aligned, err := renderer.Layout(root, ir.Inline, alignedStyle)
			if err != nil {
				t.Fatal(err)
			}
			alignedCanonical, _ := aligned.MarshalCanonical()
			if string(first) != string(alignedCanonical) {
				t.Fatalf("inline Align %d changed geometry", align)
			}
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
		if box.Width != style.MaxWidth {
			t.Fatalf("parent Width = %v, want constrained frame %v", box.Width, style.MaxWidth)
		}
		if box.Lines[0].OffsetX != 0 {
			t.Fatalf("start-aligned OffsetX = %v, want 0", box.Lines[0].OffsetX)
		}
		if box.Lines[0].Overflow != box.Overflow {
			t.Fatalf("fitting line overflow = %v, parent overflow %v", box.Lines[0].Overflow, box.Overflow)
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
		if box.Overflow <= 0 || box.Lines[0].Overflow != box.Overflow {
			t.Fatalf("overflow = %v line overflow = %v", box.Overflow, box.Lines[0].Overflow)
		}
	})
	t.Run("trailing explicit break", func(t *testing.T) {
		trailing := ir.NewRow(source.NewRange(0, 3),
			glyph("x", ir.ClassNormal),
			glyph("y", ir.ClassNormal),
			ir.NewBreak(source.NewRange(2, 3), ir.BreakExplicit),
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
			name: "table without breaks",
			root: ir.NewTable(source.NewRange(0, 8),
				ir.TableRow{
					Cells: []ir.Node{glyph("a", ir.ClassNormal), glyph("b", ir.ClassNormal)},
				},
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
			if box.Overflow <= 0 || box.Lines[0].Overflow != box.Overflow {
				t.Fatalf("overflow = %v line overflow = %v", box.Overflow, box.Lines[0].Overflow)
			}
		})
	}
}

func TestDisplayLinesFallbackWhenSourceTopologyIsUnproven(t *testing.T) {
	t.Parallel()

	renderer, _, _ := newTestRenderer(t)
	fraction := ir.NewFraction(source.NewRange(0, 3),
		glyph("1", ir.ClassNormal),
		glyph("2", ir.ClassNormal),
	)
	negative := ir.NewRow(source.NewRange(0, 7),
		glyph("a", ir.ClassNormal),
		glyph("+", ir.ClassBinary),
		glyph("b", ir.ClassNormal),
		ir.NewSpace(source.NewRange(3, 4), ir.SpaceExplicit, irEm(-23.0/16)),
		glyph("+", ir.ClassBinary),
		glyph("c", ir.ClassNormal),
	)
	table := ir.NewTable(source.NewRange(0, 8),
		ir.TableRow{
			Cells: []ir.Node{glyph("a", ir.ClassNormal), glyph("b", ir.ClassNormal)},
		},
	)
	tests := []struct {
		name      string
		root      ir.Node
		hasBreaks bool
	}{
		{name: "fraction", root: fraction},
		{name: "negative overlap", root: negative, hasBreaks: true},
		{name: "table", root: table},
		{name: "unbreakable overflow", root: ir.NewRow(source.NewRange(0, 2),
			glyph("x", ir.ClassNormal), glyph("y", ir.ClassNormal))},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			unwrapped, err := renderer.Layout(test.root, ir.Display, testStyle(16))
			if err != nil {
				t.Fatal(err)
			}
			if test.hasBreaks {
				inline, err := renderer.Layout(test.root, ir.Inline, testStyle(16))
				if err != nil {
					t.Fatal(err)
				}
				if len(inline.Breaks) == 0 {
					t.Fatal("negative-overlap probe unexpectedly had no break opportunities")
				}
			}
			style := testStyle(16)
			style.MaxWidth = 1
			box, err := renderer.Layout(test.root, ir.Display, style)
			if err != nil {
				t.Fatalf("constrained layout failed: %v", err)
			}
			if len(box.Lines) != 1 {
				t.Fatalf("Lines = %d, want one safe fallback", len(box.Lines))
			}
			line := lineBoxForTest(box, box.Lines[0])
			if !reflect.DeepEqual(line.Items, unwrapped.Items) ||
				!reflect.DeepEqual(line.Hits, unwrapped.Hits) ||
				line.Width != unwrapped.Width ||
				line.Ascent != unwrapped.Ascent ||
				line.Descent != unwrapped.Descent {
				t.Fatal("fallback line did not preserve the complete base geometry")
			}
			if box.Overflow <= 0 {
				t.Fatalf("fallback overflow = %v, want constrained overflow", box.Overflow)
			}
			if err := box.Validate(); err != nil {
				t.Fatalf("fallback Validate() error = %v", err)
			}
		})
	}
}

func TestDisplayLinesZeroAdvanceCutOwnership(t *testing.T) {
	t.Parallel()

	renderer, glyphs, _ := newTestRenderer(t)
	glyphs.glyphs["tall"] = Glyph{
		ID:      tallGlyphID,
		FontID:  "test-math",
		Advance: 0,
		Ascent:  em(3, 1),
		Descent: em(1, 1),
	}

	tests := []struct {
		name       string
		zeroClass  ir.MathClass
		breakNode  ir.Node
		beforeZero ir.Node
		wantBreaks int
	}{
		{name: "explicit break after zero", zeroClass: ir.ClassNormal,
			breakNode: ir.NewBreak(source.NewRange(2, 3), ir.BreakExplicit), wantBreaks: 1},
		{name: "automatic break after zero binary", zeroClass: ir.ClassBinary, wantBreaks: 1},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			a := glyphAt("a", ir.ClassNormal, 0, 1)
			zero := glyphAt("tall", test.zeroClass, 1, 2)
			b := glyphAt("b", ir.ClassNormal, 2, 3)
			nodes := []ir.Node{a}
			if test.beforeZero != nil {
				nodes = append(nodes, test.beforeZero)
			}
			nodes = append(nodes, zero)
			if test.breakNode != nil {
				nodes = append(nodes, test.breakNode)
			}
			nodes = append(nodes, b)
			root := ir.NewRow(source.NewRange(0, 3), nodes...)
			inline, err := renderer.Layout(root, ir.Inline, testStyle(16))
			if err != nil {
				t.Fatal(err)
			}
			if len(inline.Breaks) != test.wantBreaks {
				t.Fatalf("inline Breaks = %+v", inline.Breaks)
			}
			style := testStyle(16)
			style.MaxWidth = inline.Breaks[len(inline.Breaks)-1].X
			box, err := renderer.Layout(root, ir.Display, style)
			if err != nil {
				t.Fatal(err)
			}
			if len(box.Lines) != 2 {
				t.Fatalf("Lines = %d, want 2", len(box.Lines))
			}
			first, second := box.Lines[0], box.Lines[1]
			firstView, secondView := lineBoxForTest(box, first), lineBoxForTest(box, second)
			if len(firstView.Items) < 2 {
				t.Fatalf("first line items = %d, zero-advance atom was lost or moved", len(firstView.Items))
			}
			if !closeHost(first.Ascent, 48) || !closeHost(first.Descent, 16) {
				t.Fatalf("first line metrics = %v/%v, want tall zero extents", first.Ascent, first.Descent)
			}
			if !hasAtomHit(firstView.Hits, source.NewRange(1, 2)) {
				t.Fatal("zero-advance atom hit is not owned by the first line")
			}
			if hasAtomHit(firstView.Hits, source.NewRange(2, 3)) {
				t.Fatal("next atom hit leaked onto the first line")
			}
			if !hasAtomHit(secondView.Hits, source.NewRange(2, 3)) {
				t.Fatal("next atom hit is not owned by the second line")
			}
			if got := secondView.Items[0].Bound.X; !closeHost(got, 0) {
				t.Fatalf("second line starts at x=%v, want 0", got)
			}
			wantBaseline := first.Descent + second.Ascent
			if !closeHost(second.Baseline, wantBaseline) {
				t.Fatalf("second baseline = %v, want %v", second.Baseline, wantBaseline)
			}
		})
	}
}

func TestDisplayLinesEqualWidthUsesLowestLogicalCut(t *testing.T) {
	t.Parallel()

	renderer, glyphs, _ := newTestRenderer(t)
	glyphs.glyphs["tall"] = Glyph{
		ID:      tallGlyphID,
		FontID:  "test-math",
		Advance: 0,
		Ascent:  em(3, 1),
		Descent: em(1, 1),
	}
	a := glyphAt("a", ir.ClassNormal, 0, 1)
	zero := glyphAt("tall", ir.ClassNormal, 1, 2)
	b := glyphAt("b", ir.ClassNormal, 2, 3)
	root := ir.NewRow(source.NewRange(0, 3),
		a,
		ir.NewBreak(source.NewRange(1, 2), ir.BreakExplicit),
		zero,
		ir.NewBreak(source.NewRange(2, 3), ir.BreakExplicit),
		b,
	)
	inline, err := renderer.Layout(root, ir.Inline, testStyle(16))
	if err != nil {
		t.Fatal(err)
	}
	if len(inline.Breaks) != 2 || inline.Breaks[0].X != inline.Breaks[1].X {
		t.Fatalf("inline Breaks = %+v, want two distinct equal-width cuts", inline.Breaks)
	}

	style := testStyle(16)
	style.MaxWidth = inline.Breaks[0].X
	box, err := renderer.Layout(root, ir.Display, style)
	if err != nil {
		t.Fatal(err)
	}
	if len(box.Lines) != 2 {
		t.Fatalf("Lines = %d, want 2", len(box.Lines))
	}
	first, second := box.Lines[0], box.Lines[1]
	if !hasAtomHit(lineBoxForTest(box, first).Hits, source.NewRange(0, 1)) {
		t.Fatal("first line does not own the atom before the first cut")
	}
	if hasAtomHit(lineBoxForTest(box, first).Hits, source.NewRange(1, 2)) {
		t.Fatal("zero-advance atom crossed the lower logical cut")
	}
	if !hasAtomHit(lineBoxForTest(box, second).Hits, source.NewRange(1, 2)) {
		t.Fatal("second line does not own the zero-advance atom")
	}
}

const tallGlyphID uint16 = 0x7a11

func glyphAt(text string, class ir.MathClass, start, end int) *ir.Glyph {
	node := ir.NewGlyph(source.NewRange(start, end), text)
	node.Class = class
	return node
}

func hasAtomHit(hits []display.Hit, span source.Range) bool {
	for _, hit := range hits {
		if hit.Source == span && hit.Kind == display.HitAtom {
			return true
		}
	}
	return false
}

func TestDisplayParentOverflowIsGlobal(t *testing.T) {
	t.Parallel()

	state := &layoutState{
		ctx:      context.Background(),
		request:  display.Style{MaxWidth: 20},
		hostSize: hostSubunits,
		limits:   source.Limits{}.WithDefaults(),
	}
	lines := []displayLineInput{
		{Box: &display.Box{
			Width: 10, Ascent: 10, Overflow: 5,
			Items: []display.Item{{
				Kind:      display.ItemRule,
				Bound:     geometry.Rect{X: 8, Y: -10, Width: 7, Height: 10},
				PathIndex: display.NoPath,
			}},
		}},
		{Box: &display.Box{
			Width: 20, Ascent: 10,
			Items: []display.Item{{
				Kind:      display.ItemRule,
				Bound:     geometry.Rect{X: 0, Y: -10, Width: 20, Height: 10},
				PathIndex: display.NoPath,
			}},
		}},
	}
	parent, err := assembleDisplayLinesForTest(state, lines)
	if err != nil {
		t.Fatal(err)
	}
	if parent.Overflow != 0 {
		t.Fatalf("parent Overflow = %v, want global overflow 0 (first line local overflow was 5)", parent.Overflow)
	}
}

type countdownContext struct {
	context.Context
	remaining int
}

func (c *countdownContext) Err() error {
	if c.remaining == 0 {
		return context.Canceled
	}
	c.remaining--
	return nil
}

func TestDisplayLineLargePathExtentCancellation(t *testing.T) {
	t.Parallel()

	ctx := &countdownContext{Context: context.Background(), remaining: 2}
	state := &layoutState{
		ctx:      ctx,
		request:  display.Style{MaxWidth: 1},
		hostSize: hostSubunits,
		limits:   source.Limits{}.WithDefaults(),
	}
	root := emptyFragment(Display, geometry.Em(geometry.EmScale))
	root.Width = geometry.Em(geometry.EmScale)
	path := &pathDraw{path: make(Path, 300)}
	if err := root.pushPath(0, 0, path); err != nil {
		t.Fatal(err)
	}
	_, err := state.lineFragment(root,
		lineBoundary{itemIndex: 0, x: 0},
		lineBoundary{itemIndex: 1, x: root.Width},
	)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("lineFragment() error = %v, want path-scan cancellation", err)
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
		Limits: source.Limits{MaxOutputItems: 2},
	})
	if err != nil {
		t.Fatal(err)
	}
	style := testStyle(16)
	style.MaxWidth = 100
	if _, err := limited.Layout(root, ir.Display, style); !errors.Is(err, ErrLimit) {
		t.Fatalf("line output limit error = %v, want ErrLimit", err)
	}

	hitLimited, err := New(Fonts{Glyphs: glyphs, Metrics: fakeStyleFont{}}, Options{
		Limits: source.Limits{MaxNodes: 3},
	})
	if err != nil {
		t.Fatal(err)
	}
	wrapRoot := ir.NewRow(source.NewRange(0, 3),
		glyph("a", ir.ClassNormal),
		glyph("+", ir.ClassBinary),
		glyph("b", ir.ClassNormal),
	)
	style.MaxWidth = 1
	if _, err := hitLimited.Layout(wrapRoot, ir.Display, style); !errors.Is(err, ErrLimit) {
		t.Fatalf("parent hit aggregation error = %v, want ErrLimit", err)
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

func TestDisplayRangeLimitsCountCanonicalStorageOnce(t *testing.T) {
	t.Parallel()

	root := ir.NewRow(source.NewRange(0, 3),
		glyph("a", ir.ClassNormal),
		glyph("+", ir.ClassBinary),
		glyph("b", ir.ClassNormal),
	)
	fonts := Fonts{Glyphs: &fakeGlyphSource{glyphs: map[string]Glyph{}}, Metrics: fakeStyleFont{}}
	style := testStyle(16)
	style.MaxWidth = 1
	baselineRenderer, err := New(fonts, Options{})
	if err != nil {
		t.Fatal(err)
	}
	baseline, err := baselineRenderer.Layout(root, ir.Display, style)
	if err != nil {
		t.Fatal(err)
	}
	pathSegments := 0
	for _, item := range baseline.Items {
		if item.Kind == display.ItemGlyphPath {
			pathSegments += len(pathSegmentsForTest(baseline, item))
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
			box, err := renderer.Layout(root, ir.Display, style)
			if test.wantErr {
				if !errors.Is(err, ErrLimit) || box != nil {
					t.Fatalf("Layout() = %v, %v, want ErrLimit", box, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("Layout() error = %v", err)
			}
			if len(box.Items) != len(baseline.Items) {
				t.Fatalf("items = %d, want %d", len(box.Items), len(baseline.Items))
			}
		})
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
