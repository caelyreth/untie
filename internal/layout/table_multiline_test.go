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

func TestBuiltInTableExactBaselines(t *testing.T) {
	t.Parallel()

	renderer, _, _ := newTestRenderer(t)
	style := testStyle(16)
	style.GlyphRefs = true

	t.Run("matrix", func(t *testing.T) {
		table := ir.NewTable(source.NewRange(0, 7),
			ir.TableRow{Cells: []ir.Node{glyph("x", ir.ClassNormal), glyph("y", ir.ClassNormal)}},
			ir.TableRow{Cells: []ir.Node{glyph("x", ir.ClassNormal), glyph("y", ir.ClassNormal)}},
		)
		box, err := renderer.Layout(table, ir.Display, style)
		if err != nil {
			t.Fatal(err)
		}
		if err := box.Validate(); err != nil {
			t.Fatal(err)
		}
		if !closeHost(box.Width, 16) || !closeHost(box.Ascent, 18.4) || !closeHost(box.Descent, 10.4) {
			t.Fatalf("box metrics = %+v", box)
		}
		refs := glyphItems(box)
		if len(refs) != 4 {
			t.Fatalf("refs = %d, want 4", len(refs))
		}
		wantX := []float32{0, 0, 8, 8}
		wantY := []float32{-7.2, 7.2, -7.2, 7.2}
		for i := range refs {
			if !closeHost(refs[i].At.X, wantX[i]) || !closeHost(refs[i].At.Y, wantY[i]) {
				t.Fatalf("matrix glyph %d at %+v, want (%v,%v)", i, refs[i].At, wantX[i], wantY[i])
			}
		}
	})

	t.Run("cases", func(t *testing.T) {
		table := ir.NewTable(source.NewRange(0, 5),
			ir.TableRow{Cells: []ir.Node{glyph("x", ir.ClassNormal), glyph("y", ir.ClassNormal)}},
			ir.TableRow{Cells: []ir.Node{glyph("1", ir.ClassNormal)}},
		)
		table.Columns = []ir.ColumnAlign{ir.AlignLeft, ir.AlignLeft}
		box, err := renderer.Layout(table, ir.Display, style)
		if err != nil {
			t.Fatal(err)
		}
		refs := glyphItems(box)
		if len(refs) != 3 {
			t.Fatalf("refs = %d, want 3", len(refs))
		}
		if !closeHost(refs[1].At.X, 0) || !closeHost(refs[1].At.Y, 7.2) ||
			!closeHost(refs[2].At.X, 8) || !closeHost(refs[2].At.Y, -7.2) {
			t.Fatalf("cases positions = %+v", refs)
		}
	})

	t.Run("ragged empty row", func(t *testing.T) {
		table := ir.NewTable(source.NewRange(0, 5),
			ir.TableRow{Cells: []ir.Node{glyph("x", ir.ClassNormal), glyph("y", ir.ClassNormal)}},
			ir.TableRow{},
		)
		box, err := renderer.Layout(table, ir.Display, style)
		if err != nil {
			t.Fatal(err)
		}
		if !closeHost(box.Width, 16) || !closeHost(box.Ascent, 18.4) || !closeHost(box.Descent, 10.4) {
			t.Fatalf("ragged box = %+v", box)
		}
		if refs := glyphItems(box); len(refs) != 2 {
			t.Fatalf("ragged refs = %d, want 2", len(refs))
		}
	})

	t.Run("right and center alignment", func(t *testing.T) {
		table := ir.NewTable(source.NewRange(0, 5),
			ir.TableRow{Cells: []ir.Node{glyph("a", ir.ClassNormal)}},
			ir.TableRow{Cells: []ir.Node{glyph("bb", ir.ClassNormal)}},
		)
		table.Columns = []ir.ColumnAlign{ir.AlignRight}
		box, err := renderer.Layout(table, ir.Display, style)
		if err != nil {
			t.Fatal(err)
		}
		refs := glyphItems(box)
		if !closeHost(refs[0].At.X, 1.6) || !closeHost(refs[1].At.X, 0) {
			t.Fatalf("right-aligned refs = %+v", refs)
		}
		table.Columns[0] = ir.AlignCenter
		box, err = renderer.Layout(table, ir.Display, style)
		if err != nil {
			t.Fatal(err)
		}
		refs = glyphItems(box)
		if !closeHost(refs[0].At.X, 0.8) || !closeHost(refs[1].At.X, 0) {
			t.Fatalf("centered refs = %+v", refs)
		}
	})

	t.Run("inherits script cell style", func(t *testing.T) {
		table := ir.NewTable(source.NewRange(0, 1), ir.TableRow{Cells: []ir.Node{glyph("x", ir.ClassNormal)}})
		table.Size = ir.SizeScript
		box, err := renderer.Layout(table, ir.Display, style)
		if err != nil {
			t.Fatal(err)
		}
		refs := glyphItems(box)
		if len(refs) != 1 || !closeHost(refs[0].Em, 9.5998535) || !closeHost(box.Ascent, 7.5197754) {
			t.Fatalf("script table = %+v refs=%+v", box, refs)
		}
	})
}

func TestTableAlignmentSubcolumns(t *testing.T) {
	t.Parallel()

	renderer, _, _ := newTestRenderer(t)
	style := testStyle(16)
	style.GlyphRefs = true
	table := ir.NewTable(source.NewRange(0, 8),
		ir.TableRow{Cells: []ir.Node{alignedCell("a", "b")}},
		ir.TableRow{Cells: []ir.Node{alignedCell("bb", "c")}},
	)
	box, err := renderer.Layout(table, ir.Display, style)
	if err != nil {
		t.Fatal(err)
	}
	refs := glyphItems(box)
	if len(refs) != 4 {
		t.Fatalf("refs = %d, want 4", len(refs))
	}
	wantX := []float32{1.6, 9.6, 0, 9.6}
	for i := range refs {
		if !closeHost(refs[i].At.X, wantX[i]) {
			t.Fatalf("aligned glyph %d x = %v, want %v", i, refs[i].At.X, wantX[i])
		}
	}
	if !closeHost(box.Width, 17.6) {
		t.Fatalf("width = %v, want 17.6", box.Width)
	}
}

func TestMultilineAlignmentAndCentering(t *testing.T) {
	t.Parallel()

	renderer, _, _ := newTestRenderer(t)
	style := testStyle(16)
	style.GlyphRefs = true

	t.Run("aligned alternating columns", func(t *testing.T) {
		multiline := ir.NewMultiline(source.NewRange(0, 12),
			[]ir.Node{glyph("a", ir.ClassNormal), alignMarker(), glyph("=", ir.ClassNormal), glyph("1", ir.ClassNormal)},
			[]ir.Node{glyph("bb", ir.ClassNormal), alignMarker(), glyph("=", ir.ClassNormal), glyph("2", ir.ClassNormal)},
		)
		box, err := renderer.Layout(multiline, ir.Display, style)
		if err != nil {
			t.Fatal(err)
		}
		refs := glyphItems(box)
		if len(refs) != 6 {
			t.Fatalf("refs = %d, want 6", len(refs))
		}
		wantX := []float32{1.6, 9.6, 17.6, 0, 9.6, 17.6}
		wantY := []float32{0, 0, 0, 24.8, 24.8, 24.8}
		for i := range refs {
			if !closeHost(refs[i].At.X, wantX[i]) || !closeHost(refs[i].At.Y, wantY[i]) {
				t.Fatalf("multiline glyph %d at %+v, want (%v,%v)", i, refs[i].At, wantX[i], wantY[i])
			}
		}
		if !closeHost(box.Ascent, 11.2) || !closeHost(box.Descent, 28) || !closeHost(box.Width, 25.6) {
			t.Fatalf("aligned box = %+v", box)
		}
	})

	t.Run("gathered centers each row on first baseline", func(t *testing.T) {
		multiline := ir.NewMultiline(source.NewRange(0, 5),
			[]ir.Node{glyph("a", ir.ClassNormal)},
			[]ir.Node{glyph("bb", ir.ClassNormal)},
		)
		multiline.Centered = true
		box, err := renderer.Layout(multiline, ir.Display, style)
		if err != nil {
			t.Fatal(err)
		}
		refs := glyphItems(box)
		if len(refs) != 2 {
			t.Fatalf("refs = %d, want 2", len(refs))
		}
		if !closeHost(refs[0].At.X, 0.8) || !closeHost(refs[0].At.Y, 0) ||
			!closeHost(refs[1].At.X, 0) || !closeHost(refs[1].At.Y, 24.8) {
			t.Fatalf("gathered positions = %+v", refs)
		}
		if !closeHost(box.Ascent, 11.2) || !closeHost(box.Descent, 28) {
			t.Fatalf("gathered box = %+v", box)
		}
	})
}

func TestAlignmentMovesInfixLeftSpacingToPreviousColumn(t *testing.T) {
	t.Parallel()

	renderer, _, _ := newTestRenderer(t)
	style := testStyle(16)
	style.GlyphRefs = true
	relation := glyph("=", ir.ClassRelation)
	relation.LeftSpacing = irEm(0.5)
	multiline := ir.NewMultiline(source.NewRange(0, 3),
		[]ir.Node{glyph("a", ir.ClassNormal), alignMarker(), relation, glyph("1", ir.ClassNormal)},
	)
	box, err := renderer.Layout(multiline, ir.Display, style)
	if err != nil {
		t.Fatal(err)
	}
	refs := glyphItems(box)
	if len(refs) != 3 {
		t.Fatalf("refs = %d, want 3", len(refs))
	}
	if !closeHost(refs[0].At.X, 0) || !closeHost(refs[1].At.X, 16) || !closeHost(refs[2].At.X, 28.444336) {
		t.Fatalf("aligned refs = %+v, want a at 0, = at 16, 1 at 28.444336", refs)
	}
}

func TestBuiltInTableRulesTagsAndDashes(t *testing.T) {
	t.Parallel()

	renderer, _, _ := newTestRenderer(t)
	style := testStyle(16)
	style.GlyphRefs = true
	table := ir.NewTable(source.NewRange(0, 7),
		ir.TableRow{Cells: []ir.Node{glyph("x", ir.ClassNormal), glyph("y", ir.ClassNormal)}},
		ir.TableRow{Cells: []ir.Node{glyph("x", ir.ClassNormal), glyph("y", ir.ClassNormal)}},
	)
	table.RowGap = irEm(0.2)
	table.ColumnGap = irEm(0.4)
	thickness := irEm(0.05)
	table.Horizontal = []ir.TableLine{
		{Position: 0, Count: 1, Thickness: thickness},
		{Position: 1, Count: 1, Thickness: thickness},
		{Position: 2, Count: 1, Thickness: thickness},
	}
	table.Vertical = []ir.TableLine{
		{Position: 0, Count: 1, Thickness: thickness},
		{Position: 1, Count: 1, Thickness: thickness},
		{Position: 2, Count: 1, Thickness: thickness},
	}
	box, err := renderer.Layout(table, ir.Display, style)
	if err != nil {
		t.Fatal(err)
	}
	rules := ruleItems(box)
	if len(rules) != 6 {
		t.Fatalf("rules = %d, want 6", len(rules))
	}
	wantVerticalX := []float32{2.8, 17.2, 31.6}
	verticalIndex := 0
	for _, item := range rules {
		if item.Rect.Width < item.Rect.Height {
			if !closeHost(item.Rect.X, wantVerticalX[verticalIndex]) || !closeHost(item.Rect.Y, -23.2) {
				t.Fatalf("vertical rule %d = %+v", verticalIndex, item.Rect)
			}
			verticalIndex++
		}
	}
	if verticalIndex != 3 {
		t.Fatalf("vertical rules = %d, want 3", verticalIndex)
	}
	wantHorizontalY := []float32{-22, -4.4, 13.2}
	horizontalIndex := 0
	for _, item := range rules {
		if item.Rect.Width > item.Rect.Height {
			if !closeHost(item.Rect.Y, wantHorizontalY[horizontalIndex]) || !closeHost(item.Rect.Width, 35.2) {
				t.Fatalf("horizontal rule %d = %+v", horizontalIndex, item.Rect)
			}
			horizontalIndex++
		}
	}
	if horizontalIndex != 3 {
		t.Fatalf("horizontal rules = %d, want 3", horizontalIndex)
	}

	tagged := ir.NewTable(source.NewRange(0, 3), ir.TableRow{
		Cells: []ir.Node{glyph("x", ir.ClassNormal)},
		Tag:   glyph("tag", ir.ClassNormal),
	})
	taggedBox, err := renderer.Layout(tagged, ir.Display, style)
	if err != nil {
		t.Fatal(err)
	}
	taggedRefs := glyphItems(taggedBox)
	if len(taggedRefs) != 2 || !closeHost(taggedRefs[1].At.X, 24) || !closeHost(taggedRefs[1].At.Y, 0) {
		t.Fatalf("tag positions = %+v", taggedRefs)
	}

	dashed := ir.NewTable(source.NewRange(0, 3), ir.TableRow{Cells: []ir.Node{glyph("x", ir.ClassNormal)}})
	dashed.Horizontal = []ir.TableLine{{Position: 0, Count: 1, Dashed: true, Thickness: thickness}}
	dashedBox, err := renderer.Layout(dashed, ir.Display, style)
	if err != nil {
		t.Fatal(err)
	}
	dashRules := ruleItems(dashedBox)
	if len(dashRules) < 2 {
		t.Fatalf("dashed rules = %d, want multiple", len(dashRules))
	}
	for i := 1; i < len(dashRules); i++ {
		if dashRules[i].Rect.X <= dashRules[i-1].Rect.X {
			t.Fatalf("dash order = %+v", dashRules)
		}
	}
}

func TestTableLayoutIsDeterministic(t *testing.T) {
	t.Parallel()

	renderer, _, _ := newTestRenderer(t)
	table := ir.NewTable(source.NewRange(0, 7),
		ir.TableRow{Cells: []ir.Node{glyph("x", ir.ClassNormal), glyph("y", ir.ClassNormal)}},
		ir.TableRow{Cells: []ir.Node{glyph("x", ir.ClassNormal), glyph("y", ir.ClassNormal)}},
	)
	table.RowGap = irEm(0.2)
	table.ColumnGap = irEm(0.4)
	table.Horizontal = []ir.TableLine{{Position: 1, Count: 1, Dashed: true, Thickness: irEm(0.05)}}
	first, err := renderer.Layout(table, ir.Display, testStyle(16))
	if err != nil {
		t.Fatal(err)
	}
	second, err := renderer.Layout(table, ir.Display, testStyle(16))
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(first, second) {
		t.Fatal("repeated table layout produced different output")
	}
}

func TestTableAndMultilineLimitsAndCancellation(t *testing.T) {
	t.Parallel()

	glyphs := &fakeGlyphSource{glyphs: map[string]Glyph{}}
	renderer, err := New(Fonts{Glyphs: glyphs, Metrics: fakeStyleFont{}}, Options{
		Limits: source.Limits{MaxTableCells: 1},
	})
	if err != nil {
		t.Fatal(err)
	}
	table := ir.NewTable(source.NewRange(0, 3),
		ir.TableRow{Cells: []ir.Node{glyph("x", ir.ClassNormal), glyph("y", ir.ClassNormal)}},
	)
	if _, err := renderer.Layout(table, ir.Inline, testStyle(16)); !errors.Is(err, ErrLimit) {
		t.Fatalf("table cell limit error = %v", err)
	}

	outputLimited, err := New(Fonts{Glyphs: glyphs, Metrics: fakeStyleFont{}}, Options{
		Limits: source.Limits{MaxOutputItems: 1},
	})
	if err != nil {
		t.Fatal(err)
	}
	lineTable := ir.NewTable(source.NewRange(0, 1), ir.TableRow{Cells: []ir.Node{glyph("x", ir.ClassNormal)}})
	lineTable.Horizontal = []ir.TableLine{{Position: 0, Count: 1}}
	if _, err := outputLimited.Layout(lineTable, ir.Inline, testStyle(16)); !errors.Is(err, ErrLimit) {
		t.Fatalf("table output limit error = %v", err)
	}

	dimensionLimited, err := New(Fonts{Glyphs: glyphs, Metrics: fakeStyleFont{}}, Options{
		Limits: source.Limits{MaxDimension: 1},
	})
	if err != nil {
		t.Fatal(err)
	}
	gapped := ir.NewTable(source.NewRange(0, 1), ir.TableRow{Cells: []ir.Node{glyph("x", ir.ClassNormal)}})
	gapped.RowGap = irEm(2)
	if _, err := dimensionLimited.Layout(gapped, ir.Inline, testStyle(16)); !errors.Is(err, ErrLimit) {
		t.Fatalf("table dimension limit error = %v", err)
	}

	pathLimited, err := New(Fonts{Glyphs: glyphs, Metrics: fakeStyleFont{}}, Options{
		Limits: source.Limits{MaxPathSegments: 2},
	})
	if err != nil {
		t.Fatal(err)
	}
	pathTable := ir.NewTable(source.NewRange(0, 1), ir.TableRow{Cells: []ir.Node{glyph("x", ir.ClassNormal)}})
	if _, err := pathLimited.Layout(pathTable, ir.Inline, testStyle(16)); !errors.Is(err, ErrLimit) {
		t.Fatalf("table path limit error = %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancelling := &cancelGlyphSource{base: glyphs, cancel: cancel}
	cancellable, err := New(Fonts{Glyphs: cancelling, Metrics: fakeStyleFont{}}, Options{})
	if err != nil {
		t.Fatal(err)
	}
	cancelTable := ir.NewTable(source.NewRange(0, 3),
		ir.TableRow{Cells: []ir.Node{glyph("x", ir.ClassNormal), glyph("y", ir.ClassNormal)}},
	)
	if _, err := cancellable.LayoutContext(ctx, cancelTable, ir.Inline, testStyle(16)); !errors.Is(err, context.Canceled) {
		t.Fatalf("table cancellation error = %v", err)
	}
}

func TestLargeOperatorStretchesToWideLimit(t *testing.T) {
	t.Parallel()

	glyphs := &fakeGlyphSource{glyphs: map[string]Glyph{
		"sum":  testGlyph("sum", em(1, 2)),
		"wide": testGlyph("wide", geometry.Em(geometry.EmScale)),
	}}
	stretcher := &fakeStretcher{base: glyphs}
	renderer, err := New(Fonts{Glyphs: glyphs, Metrics: fakeStyleFont{}, Stretch: stretcher}, Options{})
	if err != nil {
		t.Fatal(err)
	}
	base := glyph("sum", ir.ClassLarge)
	scripts := ir.NewScripts(source.NewRange(0, 8), base)
	scripts.Top = glyph("wide", ir.ClassNormal)
	scripts.Limits = ir.LimitsAlways
	style := testStyle(16)
	style.GlyphRefs = true
	box, err := renderer.Layout(scripts, ir.Display, style)
	if err != nil {
		t.Fatal(err)
	}
	refs := glyphItems(box)
	if len(refs) != 2 || !closeHost(refs[0].Bound.Width, 12.8) {
		t.Fatalf("stretched limit refs = %+v", refs)
	}

	withoutStretch, err := New(Fonts{Glyphs: glyphs, Metrics: fakeStyleFont{}}, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := withoutStretch.Layout(scripts, ir.Display, style); !errors.Is(err, ErrInsufficientStretch) {
		t.Fatalf("unstretched operator error = %v", err)
	}
}

func TestBuiltInTableRowsHaveParenthesisHeightFloor(t *testing.T) {
	t.Parallel()

	glyphs := &fakeGlyphSource{glyphs: map[string]Glyph{
		"tiny": {
			ID:      7,
			FontID:  "test-math",
			Advance: em(1, 5),
			Ascent:  em(1, 10),
			Descent: em(1, 10),
			Outline: Path{
				{Op: geometry.SegMoveTo, Points: [3]Point{{X: 0, Y: -em(1, 10)}}},
				{Op: geometry.SegLineTo, Points: [3]Point{{X: em(1, 5), Y: em(1, 10)}}},
				{Op: geometry.SegClose},
			},
		},
	}}
	renderer, err := New(Fonts{Glyphs: glyphs, Metrics: fakeStyleFont{}}, Options{})
	if err != nil {
		t.Fatal(err)
	}
	tiny := ir.NewGlyph(source.NewRange(0, 1), "tiny")
	table := ir.NewTable(source.NewRange(0, 1), ir.TableRow{Cells: []ir.Node{tiny}})
	box, err := renderer.Layout(table, ir.Display, testStyle(16))
	if err != nil {
		t.Fatal(err)
	}
	if got, want := box.Ascent+box.Descent, float32(14.399902); !closeHost(got, want) {
		t.Fatalf("table height = %v, want parenthesis-floor height %v", got, want)
	}
}

func BenchmarkBuiltInTable(b *testing.B) {
	renderer, _, _ := newTestRenderer(b)
	table := ir.NewTable(source.NewRange(0, 15),
		ir.TableRow{Cells: []ir.Node{glyph("a", ir.ClassNormal), glyph("b", ir.ClassNormal), glyph("c", ir.ClassNormal)}},
		ir.TableRow{Cells: []ir.Node{glyph("d", ir.ClassNormal), glyph("e", ir.ClassNormal), glyph("f", ir.ClassNormal)}},
		ir.TableRow{Cells: []ir.Node{glyph("g", ir.ClassNormal), glyph("h", ir.ClassNormal), glyph("i", ir.ClassNormal)}},
	)
	table.RowGap = irEm(0.2)
	table.ColumnGap = irEm(0.5)
	b.ResetTimer()
	for range b.N {
		if _, err := renderer.Layout(table, ir.Display, testStyle(16)); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkMultiline(b *testing.B) {
	renderer, _, _ := newTestRenderer(b)
	multiline := ir.NewMultiline(source.NewRange(0, 12),
		[]ir.Node{glyph("a", ir.ClassNormal), alignMarker(), glyph("=", ir.ClassNormal), glyph("1", ir.ClassNormal)},
		[]ir.Node{glyph("b", ir.ClassNormal), alignMarker(), glyph("=", ir.ClassNormal), glyph("2", ir.ClassNormal)},
		[]ir.Node{glyph("c", ir.ClassNormal), alignMarker(), glyph("=", ir.ClassNormal), glyph("3", ir.ClassNormal)},
	)
	b.ResetTimer()
	for range b.N {
		if _, err := renderer.Layout(multiline, ir.Display, testStyle(16)); err != nil {
			b.Fatal(err)
		}
	}
}

type cancelGlyphSource struct {
	base   *fakeGlyphSource
	cancel context.CancelFunc
}

func (s *cancelGlyphSource) Glyph(req GlyphRequest) (Glyph, error) {
	glyph, err := s.base.Glyph(req)
	s.cancel()
	return glyph, err
}

func alignedCell(left, right string) ir.Node {
	row := ir.NewRow(source.NewRange(0, 3))
	row.Items = []ir.Node{glyph(left, ir.ClassNormal), alignMarker(), glyph(right, ir.ClassNormal)}
	return row
}

func alignMarker() *ir.AlignMarker {
	return ir.NewAlignMarker(source.NewRange(0, 1))
}

func ruleItems(box *display.Box) []display.Item {
	var items []display.Item
	for _, item := range box.Items {
		if item.Kind == display.ItemRule {
			items = append(items, item)
		}
	}
	return items
}

func testGlyph(text string, advance geometry.Em) Glyph {
	return Glyph{
		ID:       1,
		FontID:   "test-math",
		Text:     text,
		Advance:  advance,
		Ascent:   em(7, 10),
		Descent:  em(2, 10),
		TextLike: true,
		Outline: Path{
			{Op: geometry.SegMoveTo, Points: [3]Point{{X: 0, Y: -em(2, 10)}}},
			{Op: geometry.SegLineTo, Points: [3]Point{{X: advance, Y: -em(2, 10)}}},
			{Op: geometry.SegLineTo, Points: [3]Point{{X: advance, Y: em(7, 10)}}},
			{Op: geometry.SegLineTo, Points: [3]Point{{X: 0, Y: em(7, 10)}}},
			{Op: geometry.SegClose},
		},
	}
}
