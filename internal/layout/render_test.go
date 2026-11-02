package layout

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"sync"
	"testing"

	"github.com/caelyreth/untie/display"
	"github.com/caelyreth/untie/geometry"
	"github.com/caelyreth/untie/ir"
	"github.com/caelyreth/untie/source"
)

func emLength(t *testing.T, value float64) ir.Length {
	t.Helper()
	length, err := ir.EmLength(value)
	if err != nil {
		t.Fatalf("EmLength(%v) error = %v", value, err)
	}
	return length
}

func TestRowAssemblyAndSpacing(t *testing.T) {
	t.Parallel()

	renderer, _, _ := newTestRenderer(t)
	x := glyph("x", ir.ClassNormal)
	plus := glyph("+", ir.ClassBinary)
	y := glyph("y", ir.ClassNormal)
	root := ir.NewRow(source.NewRange(0, 3), x, plus, y)
	box, err := renderer.Layout(root, ir.Inline, testStyle(16))
	if err != nil {
		t.Fatalf("Layout() error = %v", err)
	}

	wantWidth := float32(float64(16) * (1.5 + 2*float64(MediumSpace)/float64(geometry.EmScale)))
	if diff := wantWidth - box.Width; diff > 1.0/1024 || diff < -1.0/1024 {
		t.Fatalf("Width = %v, want %v", box.Width, wantWidth)
	}
	if got, want := box.Ascent, float32(11.2); !closeHost(got, want) {
		t.Fatalf("Ascent = %v, want %v", got, want)
	}
	if got, want := box.Descent, float32(3.2); !closeHost(got, want) {
		t.Fatalf("Descent = %v, want %v", got, want)
	}
	if len(box.Items) != 3 {
		t.Fatalf("items = %d, want 3", len(box.Items))
	}
	for i, item := range box.Items {
		if item.Kind != display.ItemGlyphPath {
			t.Fatalf("item %d kind = %d, want glyph path", i, item.Kind)
		}
		if item.Em != 16 {
			t.Fatalf("item %d Em = %v, want 16", i, item.Em)
		}
	}

	// A leading binary is unary, so it incurs no automatic operator space.
	unaryRoot := ir.NewRow(source.NewRange(0, 2), glyph("+", ir.ClassBinary), glyph("x", ir.ClassNormal))
	unaryBox, err := renderer.Layout(unaryRoot, ir.Inline, testStyle(16))
	if err != nil {
		t.Fatalf("Layout(unary) error = %v", err)
	}
	if unaryBox.Width != 16 {
		t.Fatalf("unary width = %v, want 16", unaryBox.Width)
	}
}

func TestExplicitSpaceDisablesAutomaticSpacing(t *testing.T) {
	t.Parallel()

	renderer, _, _ := newTestRenderer(t)
	space := ir.NewSpace(source.NewRange(1, 2), ir.SpaceExplicit, emLength(t, 1))
	root := ir.NewRow(source.NewRange(0, 3), glyph("x", ir.ClassNormal), space, glyph("y", ir.ClassNormal))
	box, err := renderer.Layout(root, ir.Inline, testStyle(10))
	if err != nil {
		t.Fatalf("Layout() error = %v", err)
	}
	if box.Width != 20 {
		t.Fatalf("Width = %v, want 20", box.Width)
	}

	standalone, err := renderer.Layout(space, ir.Inline, testStyle(10))
	if err != nil {
		t.Fatalf("Layout(space) error = %v", err)
	}
	if standalone.Width != 10 {
		t.Fatalf("standalone space width = %v, want 10", standalone.Width)
	}
}

func TestScriptsUseDerivedStyles(t *testing.T) {
	t.Parallel()

	renderer, _, _ := newTestRenderer(t)
	scripts := ir.NewScripts(source.NewRange(0, 3), glyph("x", ir.ClassNormal))
	scripts.TopRight = glyph("2", ir.ClassNormal)
	box, err := renderer.Layout(scripts, ir.Inline, testStyle(16))
	if err != nil {
		t.Fatalf("Layout() error = %v", err)
	}
	if len(box.Items) < 2 {
		t.Fatalf("items = %d, want base and superscript", len(box.Items))
	}
	if got, want := box.Items[0].Em, float32(16); got != want {
		t.Fatalf("base Em = %v, want %v", got, want)
	}
	if got, want := box.Items[1].Em, float32(12.8); !closeHost(got, want) {
		t.Fatalf("superscript Em = %v, want %v", got, want)
	}
}

func TestFractionProducesRuleAndNestedStyles(t *testing.T) {
	t.Parallel()

	renderer, _, _ := newTestRenderer(t)
	fraction := ir.NewFraction(source.NewRange(0, 5), glyph("1", ir.ClassNormal), glyph("2", ir.ClassNormal))
	box, err := renderer.Layout(fraction, ir.Inline, testStyle(20))
	if err != nil {
		t.Fatalf("Layout() error = %v", err)
	}
	if box.Width <= 0 || box.Ascent <= 0 || box.Descent <= 0 {
		t.Fatalf("invalid box metrics: %+v", box)
	}
	if len(box.Items) != 3 {
		t.Fatalf("items = %d, want numerator, denominator, and rule", len(box.Items))
	}
	if box.Items[0].Em != 16 || box.Items[1].Em != 16 {
		t.Fatalf("fraction child sizes = %v, %v, want 16, 16", box.Items[0].Em, box.Items[1].Em)
	}
	if box.Items[2].Kind != display.ItemRule {
		t.Fatalf("last item kind = %d, want rule", box.Items[2].Kind)
	}

	noBar := ir.NewFraction(source.NewRange(0, 5), glyph("1", ir.ClassNormal), glyph("2", ir.ClassNormal))
	noBar.Style = ir.FractionNoBar
	noBar.Bar = false
	noBarBox, err := renderer.Layout(noBar, ir.Inline, testStyle(20))
	if err != nil {
		t.Fatalf("Layout(no-bar fraction) error = %v", err)
	}
	for _, item := range noBarBox.Items {
		if item.Kind == display.ItemRule {
			t.Fatal("no-bar fraction emitted a fraction rule")
		}
	}
}

func TestRadicalAndFences(t *testing.T) {
	t.Parallel()

	renderer, _, stretcher := newTestRenderer(t)
	radical := ir.NewRadical(source.NewRange(0, 3), glyph("x", ir.ClassNormal), glyph("3", ir.ClassNormal))
	box, err := renderer.Layout(radical, ir.Inline, testStyle(16))
	if err != nil {
		t.Fatalf("Layout(radical) error = %v", err)
	}
	if len(box.Items) < 4 {
		t.Fatalf("radical items = %d, want sign, rule, radicand, and index", len(box.Items))
	}
	if stretcher.calls == 0 {
		t.Fatal("radical did not use the stretch extension port")
	}

	fenced := ir.NewFenced(source.NewRange(0, 3), glyph("x", ir.ClassNormal))
	fenced.Open = glyph("(", ir.ClassOpening)
	fenced.Close = glyph(")", ir.ClassClosing)
	fenced.Balanced = true
	fenceBox, err := renderer.Layout(fenced, ir.Inline, testStyle(16))
	if err != nil {
		t.Fatalf("Layout(fenced) error = %v", err)
	}
	if fenceBox.Width <= 16 {
		t.Fatalf("fenced width = %v, want delimiters included", fenceBox.Width)
	}
}

func TestRuleLayoutAndExplicitStyle(t *testing.T) {
	t.Parallel()

	renderer, _, _ := newTestRenderer(t)
	line := ir.NewLine(source.NewRange(0, 3), glyph("x", ir.ClassNormal), ir.PositionAbove)
	line.Thickness = emLength(t, 0.05)
	line.Gap = emLength(t, 0.1)
	box, err := renderer.Layout(line, ir.Inline, testStyle(16))
	if err != nil {
		t.Fatalf("Layout(line) error = %v", err)
	}
	if len(box.Items) != 2 || box.Items[1].Kind != display.ItemRule {
		t.Fatalf("line items = %+v, want glyph and rule", box.Items)
	}
	if !closeHost(box.Items[1].Rect.Height, float32(0.05*16)) {
		t.Fatalf("rule thickness = %v, want 0.8", box.Items[1].Rect.Height)
	}

	style := ir.NewStyle(source.NewRange(0, 3), glyph("x", ir.ClassNormal))
	style.SetSize = true
	style.NewSize = ir.SizeScriptScript
	style.SetCramped = true
	style.Cramped = true
	styledBox, err := renderer.Layout(style, ir.Inline, testStyle(16))
	if err != nil {
		t.Fatalf("Layout(style) error = %v", err)
	}
	if got, want := styledBox.Items[0].Em, float32(9.6); !closeHost(got, want) {
		t.Fatalf("styled Em = %v, want %v", got, want)
	}

	color := ir.NewColor(source.NewRange(0, 1), glyph("x", ir.ClassNormal), ir.Paint{
		Explicit:    ir.RGBA{R: 12, G: 34, B: 56, A: 255},
		HasExplicit: true,
	})
	colorBox, err := renderer.Layout(color, ir.Inline, testStyle(16))
	if err != nil {
		t.Fatalf("Layout(color) error = %v", err)
	}
	if paint := colorBox.Items[0].Paint; !paint.HasExplicit || paint.Explicit.R != 12 {
		t.Fatalf("color paint = %+v", paint)
	}
}

func TestTextTableAndPlaceholderExtensions(t *testing.T) {
	t.Parallel()

	glyphs := &fakeGlyphSource{glyphs: map[string]Glyph{}}
	table := &fakeTableLayouter{}
	renderer, err := New(Fonts{
		Glyphs:  glyphs,
		Metrics: fakeStyleFont{},
		Text:    fakeTextSource{},
	}, Options{Tables: table})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	text := ir.NewText(source.NewRange(0, 2), "ab")
	textBox, err := renderer.Layout(text, ir.Inline, testStyle(16))
	if err != nil {
		t.Fatalf("Layout(text) error = %v", err)
	}
	if len(textBox.Items) != 1 || textBox.Items[0].FontID != "test-text" {
		t.Fatalf("text items = %+v", textBox.Items)
	}

	row := ir.NewRow(source.NewRange(0, 1), glyph("x", ir.ClassNormal))
	matrix := ir.NewTable(source.NewRange(0, 1), ir.TableRow{Cells: []ir.Node{row}})
	if _, err := renderer.Layout(matrix, ir.Inline, testStyle(16)); err != nil {
		t.Fatalf("Layout(table) error = %v", err)
	}
	if !table.called {
		t.Fatal("table extension was not called")
	}

	placeholder := ir.NewPlaceholder(source.NewRange(0, 1), "missing")
	placeholderBox, err := renderer.Layout(placeholder, ir.Inline, testStyle(16))
	if err != nil {
		t.Fatalf("Layout(placeholder) error = %v", err)
	}
	if len(placeholderBox.Items) != 4 {
		t.Fatalf("placeholder items = %d, want 4 rules", len(placeholderBox.Items))
	}
	for i, item := range placeholderBox.Items {
		if item.Kind != display.ItemRule || item.Paint.Role != display.RolePlaceholder {
			t.Fatalf("placeholder item %d = %+v", i, item)
		}
	}
}

func TestGlyphRefsAndMissingGlyph(t *testing.T) {
	t.Parallel()

	glyphs := &fakeGlyphSource{glyphs: map[string]Glyph{
		"x": {
			ID:      7,
			FontID:  "test-math",
			Advance: em(1, 2),
			Ascent:  em(7, 10),
			Descent: em(2, 10),
			Outline: Path{
				{Op: geometry.SegMoveTo, Points: [3]Point{{X: 0, Y: -em(2, 10)}}},
				{Op: geometry.SegLineTo, Points: [3]Point{{X: em(1, 2), Y: em(7, 10)}}},
				{Op: geometry.SegClose},
			},
		},
	}, strict: true}
	renderer, err := New(Fonts{Glyphs: glyphs, Metrics: fakeStyleFont{}}, Options{})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	style := testStyle(16)
	style.GlyphRefs = true
	box, err := renderer.Layout(glyph("x", ir.ClassNormal), ir.Inline, style)
	if err != nil {
		t.Fatalf("Layout() error = %v", err)
	}
	if len(box.Items) != 2 || box.Items[0].Kind != display.ItemGlyphPath || box.Items[1].Kind != display.ItemGlyphRef {
		t.Fatalf("glyph ref items = %+v", box.Items)
	}
	if box.Items[1].Em != 16 || box.Items[1].Glyph != 7 {
		t.Fatalf("glyph ref = %+v, want Em=16 Glyph=7", box.Items[1])
	}
	if _, err := renderer.Layout(glyph("missing", ir.ClassNormal), ir.Inline, testStyle(16)); !errors.Is(err, ErrMissingGlyph) {
		t.Fatalf("missing glyph error = %v", err)
	}
}

func TestDeterministicFlattening(t *testing.T) {
	t.Parallel()

	renderer, _, _ := newTestRenderer(t)
	root := ir.NewRow(source.NewRange(0, 3),
		glyph("x", ir.ClassNormal),
		glyph("+", ir.ClassBinary),
		glyph("y", ir.ClassNormal),
	)
	first, err := renderer.Layout(root, ir.Inline, testStyle(17))
	if err != nil {
		t.Fatalf("Layout() error = %v", err)
	}
	firstJSON, err := json.Marshal(first)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}

	const workers = 8
	outputs := make([][]byte, workers)
	errs := make([]error, workers)
	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			box, err := renderer.Layout(root, ir.Inline, testStyle(17))
			if err != nil {
				errs[index] = err
				return
			}
			outputs[index], errs[index] = json.Marshal(box)
		}(i)
	}
	wg.Wait()
	for i := range outputs {
		if errs[i] != nil {
			t.Fatalf("worker %d error = %v", i, errs[i])
		}
		if !reflect.DeepEqual(firstJSON, outputs[i]) {
			t.Fatalf("worker %d produced non-deterministic output", i)
		}
	}
}

func TestLayoutContextCancellationAndLimits(t *testing.T) {
	t.Parallel()

	renderer, _, _ := newTestRenderer(t)
	root := ir.NewRow(source.NewRange(0, 3), glyph("x", ir.ClassNormal), glyph("y", ir.ClassNormal), glyph("z", ir.ClassNormal))
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := renderer.LayoutContext(ctx, root, ir.Inline, testStyle(16)); !errors.Is(err, context.Canceled) {
		t.Fatalf("LayoutContext() error = %v, want canceled", err)
	}

	limited, err := New(Fonts{Glyphs: &fakeGlyphSource{glyphs: map[string]Glyph{}}, Metrics: fakeStyleFont{}}, Options{
		Limits: source.Limits{MaxNodes: 2},
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	if _, err := limited.Layout(root, ir.Inline, testStyle(16)); !errors.Is(err, ErrLimit) {
		t.Fatalf("Layout() error = %v, want limit", err)
	}

	pathLimited, err := New(Fonts{Glyphs: &fakeGlyphSource{glyphs: map[string]Glyph{}}, Metrics: fakeStyleFont{}}, Options{
		Limits: source.Limits{MaxPathSegments: 2},
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	if _, err := pathLimited.Layout(glyph("x", ir.ClassNormal), ir.Inline, testStyle(16)); !errors.Is(err, ErrLimit) {
		t.Fatalf("Layout() error = %v, want path limit", err)
	}
}

func TestDisplayMaximumWidthOverflow(t *testing.T) {
	t.Parallel()

	renderer, _, _ := newTestRenderer(t)
	root := ir.NewRow(source.NewRange(0, 2), glyph("x", ir.ClassNormal), glyph("y", ir.ClassNormal))
	style := testStyle(16)
	style.MaxWidth = 8
	box, err := renderer.Layout(root, ir.Display, style)
	if err != nil {
		t.Fatalf("Layout() error = %v", err)
	}
	if box.Overflow != 8 {
		t.Fatalf("Overflow = %v, want 8", box.Overflow)
	}
}

func BenchmarkLayoutRow(b *testing.B) {
	renderer, _, _ := newTestRenderer(b)
	root := ir.NewRow(source.NewRange(0, 3),
		glyph("x", ir.ClassNormal),
		glyph("+", ir.ClassBinary),
		glyph("y", ir.ClassNormal),
	)
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if _, err := renderer.Layout(root, ir.Inline, testStyle(16)); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkLayoutScripts(b *testing.B) {
	renderer, _, _ := newTestRenderer(b)
	root := ir.NewScripts(source.NewRange(0, 3), glyph("x", ir.ClassNormal))
	root.TopRight = glyph("2", ir.ClassNormal)
	root.BottomRight = glyph("1", ir.ClassNormal)
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if _, err := renderer.Layout(root, ir.Inline, testStyle(16)); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkLayoutFraction(b *testing.B) {
	renderer, _, _ := newTestRenderer(b)
	root := ir.NewFraction(source.NewRange(0, 5), glyph("1", ir.ClassNormal), glyph("2", ir.ClassNormal))
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if _, err := renderer.Layout(root, ir.Inline, testStyle(16)); err != nil {
			b.Fatal(err)
		}
	}
}
