package layout

import (
	"errors"
	"reflect"
	"testing"

	"github.com/caelyreth/untie/display"
	"github.com/caelyreth/untie/geometry"
	"github.com/caelyreth/untie/ir"
	"github.com/caelyreth/untie/source"
)

func TestAccentPlacementStretchAndFeatures(t *testing.T) {
	t.Parallel()

	renderer, glyphs, stretcher := newTestRenderer(t)
	glyphs.glyphs["^"] = Glyph{
		ID:           31,
		FontID:       "test-math",
		Advance:      em(1, 2),
		Ascent:       em(1, 2),
		Descent:      0,
		InkBottom:    -em(1, 10),
		HasInkBottom: true,
		TopAccent:    em(1, 4),
		HasTopAccent: true,
		Outline: Path{
			{Op: geometry.SegMoveTo, Points: [3]Point{{X: 0, Y: -em(1, 10)}}},
			{Op: geometry.SegLineTo, Points: [3]Point{{X: em(1, 2), Y: -em(1, 10)}}},
			{Op: geometry.SegLineTo, Points: [3]Point{{X: em(1, 4), Y: -em(1, 2)}}},
			{Op: geometry.SegClose},
		},
	}
	body := ir.NewRow(source.NewRange(0, 3),
		glyph("x", ir.ClassNormal),
		glyph("x", ir.ClassNormal),
		glyph("x", ir.ClassNormal),
	)
	accent := ir.NewAccent(source.NewRange(0, 4), body, glyph("^", ir.ClassDiacritic), ir.PositionAbove)
	accent.Dotless = true
	box, err := renderer.Layout(accent, ir.Inline, testStyle(16))
	if err != nil {
		t.Fatalf("Layout(accent) error = %v", err)
	}
	if err := box.Validate(); err != nil {
		t.Fatalf("display validation error = %v", err)
	}
	if len(box.Items) != 4 {
		t.Fatalf("items = %d, want three body glyphs and accent", len(box.Items))
	}
	if stretcher.calls == 0 {
		t.Fatal("accent did not consult the stretch port")
	}
	if box.Items[3].At.Y >= box.Items[0].At.Y {
		t.Fatalf("accent y = %v, body y = %v; want accent above", box.Items[3].At.Y, box.Items[0].At.Y)
	}
	if !containsFeature(glyphs.seen["x"], "dtls") {
		t.Fatalf("body features = %v, want dtls", glyphs.seen["x"])
	}
	if !containsFeature(glyphs.seen["^"], "flac") {
		t.Fatalf("accent features = %v, want flac", glyphs.seen["^"])
	}

	below := ir.NewAccent(source.NewRange(0, 4), glyph("x", ir.ClassNormal), glyph("^", ir.ClassDiacritic), ir.PositionBelow)
	belowBox, err := renderer.Layout(below, ir.Inline, testStyle(16))
	if err != nil {
		t.Fatalf("Layout(below accent) error = %v", err)
	}
	if belowBox.Items[1].At.Y <= belowBox.Items[0].At.Y {
		t.Fatalf("below accent y = %v, body y = %v", belowBox.Items[1].At.Y, belowBox.Items[0].At.Y)
	}
}

func TestAccentReportsInsufficientStretch(t *testing.T) {
	t.Parallel()

	glyphs := &fakeGlyphSource{glyphs: map[string]Glyph{}}
	renderer, err := New(Fonts{Glyphs: glyphs, Metrics: fakeStyleFont{}}, Options{})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	body := ir.NewRow(source.NewRange(0, 1))
	for i := 0; i < 10; i++ {
		body.Items = append(body.Items, glyph("x", ir.ClassNormal))
	}
	accent := ir.NewAccent(source.NewRange(0, 1), body, glyph("^", ir.ClassDiacritic), ir.PositionAbove)
	_, err = renderer.Layout(accent, ir.Inline, testStyle(16))
	if !errors.Is(err, ErrUnsupported) || !errors.Is(err, ErrInsufficientStretch) {
		t.Fatalf("Layout() error = %v, want unsupported insufficient stretch", err)
	}
}

func TestDelimiterAndRadicalReportInsufficientStretch(t *testing.T) {
	t.Parallel()

	glyphs := &fakeGlyphSource{glyphs: map[string]Glyph{
		"T": {
			ID:      1,
			FontID:  "test-math",
			Advance: em(1, 2),
			Ascent:  em(2, 1),
			Descent: em(1, 2),
		},
		"(": {
			ID:      2,
			FontID:  "test-math",
			Advance: em(1, 4),
			Ascent:  em(1, 2),
			Descent: em(1, 10),
		},
		")": {
			ID:      3,
			FontID:  "test-math",
			Advance: em(1, 4),
			Ascent:  em(1, 2),
			Descent: em(1, 10),
		},
		"√": {
			ID:      4,
			FontID:  "test-math",
			Advance: em(1, 2),
			Ascent:  em(1, 2),
			Descent: em(1, 10),
		},
	}, strict: true}
	renderer, err := New(Fonts{Glyphs: glyphs, Metrics: fakeStyleFont{}}, Options{})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	fenced := ir.NewFenced(source.NewRange(0, 1), glyph("T", ir.ClassNormal))
	fenced.Open = glyph("(", ir.ClassOpening)
	fenced.Close = glyph(")", ir.ClassClosing)
	fenced.Balanced = true
	if _, err := renderer.Layout(fenced, ir.Inline, testStyle(16)); !errors.Is(err, ErrInsufficientStretch) {
		t.Fatalf("fenced stretch error = %v", err)
	}

	radical := ir.NewRadical(source.NewRange(0, 1), glyph("T", ir.ClassNormal), nil)
	if _, err := renderer.Layout(radical, ir.Inline, testStyle(16)); !errors.Is(err, ErrInsufficientStretch) {
		t.Fatalf("radical stretch error = %v", err)
	}
}

func TestCancelStrokesAndMetrics(t *testing.T) {
	t.Parallel()

	renderer, _, _ := newTestRenderer(t)
	body := glyph("x", ir.ClassNormal)
	cancel := ir.NewCancel(source.NewRange(0, 2), body)
	box, err := renderer.Layout(cancel, ir.Inline, testStyle(16))
	if err != nil {
		t.Fatalf("Layout(cancel) error = %v", err)
	}
	if err := box.Validate(); err != nil {
		t.Fatalf("display validation error = %v", err)
	}
	if len(box.Items) != 2 {
		t.Fatalf("items = %d, want body and cancel stroke", len(box.Items))
	}
	if box.Items[1].Kind != display.ItemGlyphPath || box.Items[1].FontID != "" ||
		len(box.Paths[box.Items[1].PathIndex].Segments) != 5 {
		t.Fatalf("cancel stroke = %+v", box.Items[1])
	}
	if !closeHost(box.Width, 8) || !closeHost(box.Ascent, 10.9279785) || !closeHost(box.Descent, 0) {
		t.Fatalf("cancel metrics = %v/%v/%v", box.Width, box.Ascent, box.Descent)
	}

	inverted := ir.NewCancel(source.NewRange(0, 2), glyph("x", ir.ClassNormal))
	inverted.Inverted = true
	invertedBox, err := renderer.Layout(inverted, ir.Inline, testStyle(16))
	if err != nil {
		t.Fatalf("Layout(inverted cancel) error = %v", err)
	}
	if reflect.DeepEqual(
		box.Paths[box.Items[1].PathIndex].Segments,
		invertedBox.Paths[invertedBox.Items[1].PathIndex].Segments,
	) {
		t.Fatal("inverted cancel did not reverse the stroke")
	}

	crossed := ir.NewCancel(source.NewRange(0, 2), glyph("x", ir.ClassNormal))
	crossed.Cross = true
	crossedBox, err := renderer.Layout(crossed, ir.Inline, testStyle(16))
	if err != nil {
		t.Fatalf("Layout(crossed cancel) error = %v", err)
	}
	if len(crossedBox.Items) != 3 {
		t.Fatalf("crossed items = %d, want two strokes", len(crossedBox.Items))
	}

	bad := ir.NewCancel(source.NewRange(0, 2), glyph("x", ir.ClassNormal))
	bad.Thickness = irEm(-0.05)
	if _, err := renderer.Layout(bad, ir.Inline, testStyle(16)); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("negative cancel thickness error = %v", err)
	}
}

func TestBoxPaddingFillBorderRadiusAndLimits(t *testing.T) {
	t.Parallel()

	renderer, _, _ := newTestRenderer(t)
	boxNode := ir.NewBox(source.NewRange(0, 3), glyph("x", ir.ClassNormal))
	boxNode.Padding = [4]ir.Length{irEm(0.1), irEm(0.1), irEm(0.1), irEm(0.1)}
	boxNode.HasFill = true
	boxNode.Fill = ir.Paint{Explicit: ir.RGBA{R: 1, G: 2, B: 3, A: 255}, HasExplicit: true}
	boxNode.HasBorder = true
	boxNode.BorderWidth = irEm(0.05)
	boxNode.Border = ir.Paint{Role: ir.PaintError}
	boxNode.Radius = irEm(0.1)
	box, err := renderer.Layout(boxNode, ir.Inline, testStyle(16))
	if err != nil {
		t.Fatalf("Layout(box) error = %v", err)
	}
	if err := box.Validate(); err != nil {
		t.Fatalf("display validation error = %v", err)
	}
	if len(box.Items) != 3 {
		t.Fatalf("items = %d, want fill, border, and body", len(box.Items))
	}
	if box.Items[0].Kind != display.ItemGlyphPath || !box.Items[0].Paint.HasExplicit || box.Items[0].Paint.Explicit.R != 1 {
		t.Fatalf("fill item = %+v", box.Items[0])
	}
	if box.Items[1].Kind != display.ItemGlyphPath || box.Items[1].Paint.Role != display.RoleError {
		t.Fatalf("border item = %+v", box.Items[1])
	}
	closes := 0
	for _, segment := range box.Paths[box.Items[1].PathIndex].Segments {
		if segment.Op == geometry.SegClose {
			closes++
		}
	}
	if closes != 2 {
		t.Fatalf("border contours = %d, want outer and inner ring", closes)
	}
	if !closeHost(box.Width, 11.200195) || !closeHost(box.Ascent, 10.9279785) || !closeHost(box.Descent, 0) {
		t.Fatalf("box metrics = %v/%v/%v", box.Width, box.Ascent, box.Descent)
	}

	square := ir.NewBox(source.NewRange(0, 1), glyph("x", ir.ClassNormal))
	square.HasFill = true
	square.HasBorder = true
	squareBox, err := renderer.Layout(square, ir.Inline, testStyle(16))
	if err != nil {
		t.Fatalf("Layout(square box) error = %v", err)
	}
	if len(squareBox.Items) < 3 || squareBox.Items[0].Kind != display.ItemRule {
		t.Fatalf("square box items = %+v", squareBox.Items)
	}

	bad := ir.NewBox(source.NewRange(0, 1), glyph("x", ir.ClassNormal))
	bad.Radius = irEm(-0.1)
	if _, err := renderer.Layout(bad, ir.Inline, testStyle(16)); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("negative radius error = %v", err)
	}

	limited, err := New(Fonts{Glyphs: &fakeGlyphSource{glyphs: map[string]Glyph{}}, Metrics: fakeStyleFont{}}, Options{
		Limits: source.Limits{MaxDimension: 1},
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	if _, err := limited.Layout(boxNode, ir.Inline, testStyle(16)); !errors.Is(err, ErrLimit) {
		t.Fatalf("oversized box error = %v, want limit", err)
	}

	pathLimited, err := New(Fonts{Glyphs: &fakeGlyphSource{glyphs: map[string]Glyph{}}, Metrics: fakeStyleFont{}}, Options{
		Limits: source.Limits{MaxPathSegments: 10},
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	if _, err := pathLimited.Layout(boxNode, ir.Inline, testStyle(16)); !errors.Is(err, ErrLimit) {
		t.Fatalf("rounded box path limit error = %v, want limit", err)
	}
}

func TestPhantomRetainsSelectedExtents(t *testing.T) {
	t.Parallel()

	renderer, _, _ := newTestRenderer(t)
	tests := []struct {
		name                 string
		width, height, depth bool
		wantWidth            float32
		wantAscent           float32
		wantDescent          float32
	}{
		{"width", true, false, false, 8, 10.9279785, 0},
		{"height", false, true, false, 0, 10.9279785, 0},
		{"depth", false, false, true, 0, 10.9279785, 0},
		{"all", true, true, true, 8, 10.9279785, 0},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			node := ir.NewPhantom(source.NewRange(0, 1), glyph("x", ir.ClassNormal))
			node.Width, node.Height, node.Depth = test.width, test.height, test.depth
			box, err := renderer.Layout(node, ir.Inline, testStyle(16))
			if err != nil {
				t.Fatalf("Layout() error = %v", err)
			}
			if len(box.Items) != 0 {
				t.Fatalf("items = %d, want no hidden ink", len(box.Items))
			}
			if !closeHost(box.Width, test.wantWidth) || !closeHost(box.Ascent, test.wantAscent) || !closeHost(box.Descent, test.wantDescent) {
				t.Fatalf("metrics = %v/%v/%v, want %v/%v/%v", box.Width, box.Ascent, box.Descent, test.wantWidth, test.wantAscent, test.wantDescent)
			}
		})
	}

	oversized := ir.NewPhantom(source.NewRange(0, 1), glyph("x", ir.ClassNormal))
	limited, err := New(Fonts{Glyphs: &fakeGlyphSource{glyphs: map[string]Glyph{}}, Metrics: fakeStyleFont{}}, Options{
		Limits: source.Limits{MaxDimension: 1},
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	if _, err := limited.Layout(oversized, ir.Inline, testStyle(16)); !errors.Is(err, ErrLimit) {
		t.Fatalf("oversized phantom error = %v, want limit", err)
	}
}

func TestEndToEndPathOperationsAndLimit(t *testing.T) {
	t.Parallel()

	glyphs := &fakeGlyphSource{glyphs: map[string]Glyph{
		"x": {
			ID:      9,
			FontID:  "test-math",
			Advance: em(1, 2),
			Ascent:  em(1, 5),
			Descent: em(1, 5),
			Outline: Path{
				{Op: geometry.SegMoveTo, Points: [3]Point{{X: 0, Y: -em(1, 5)}}},
				{Op: geometry.SegLineTo, Points: [3]Point{{X: em(1, 2), Y: -em(1, 5)}}},
				{Op: geometry.SegQuadTo, Points: [3]Point{{X: em(3, 5), Y: 0}, {X: em(1, 2), Y: em(1, 5)}}},
				{Op: geometry.SegCubeTo, Points: [3]Point{{X: em(2, 5), Y: em(2, 5)}, {X: em(1, 5), Y: em(2, 5)}, {X: 0, Y: em(1, 5)}}},
				{Op: geometry.SegClose},
			},
		},
	}, strict: true}
	renderer, err := New(Fonts{Glyphs: glyphs, Metrics: fakeStyleFont{}}, Options{})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	box, err := renderer.Layout(glyph("x", ir.ClassNormal), ir.Inline, testStyle(16))
	if err != nil {
		t.Fatalf("Layout() error = %v", err)
	}
	if err := box.Validate(); err != nil {
		t.Fatalf("display validation error = %v", err)
	}
	if len(box.Items) != 1 {
		t.Fatalf("path items = %+v", box.Items)
	}
	segments := box.Paths[box.Items[0].PathIndex].Segments
	if len(segments) != 5 || segments[4].Op != geometry.SegClose {
		t.Fatalf("path = %+v", box.Items)
	}

	exact, err := New(Fonts{Glyphs: glyphs, Metrics: fakeStyleFont{}}, Options{
		Limits: source.Limits{MaxPathSegments: 5},
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	if _, err := exact.Layout(glyph("x", ir.ClassNormal), ir.Inline, testStyle(16)); err != nil {
		t.Fatalf("exact path limit error = %v", err)
	}

	limited, err := New(Fonts{Glyphs: glyphs, Metrics: fakeStyleFont{}}, Options{
		Limits: source.Limits{MaxPathSegments: 4},
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	if _, err := limited.Layout(glyph("x", ir.ClassNormal), ir.Inline, testStyle(16)); !errors.Is(err, ErrLimit) {
		t.Fatalf("path limit error = %v", err)
	}
}

func containsFeature(features []string, want string) bool {
	for _, feature := range features {
		if feature == want {
			return true
		}
	}
	return false
}

func BenchmarkLayoutAccent(b *testing.B) {
	renderer, _, _ := newTestRenderer(b)
	root := ir.NewAccent(source.NewRange(0, 3), glyph("x", ir.ClassNormal), glyph("^", ir.ClassDiacritic), ir.PositionAbove)
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if _, err := renderer.Layout(root, ir.Inline, testStyle(16)); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkLayoutCancel(b *testing.B) {
	renderer, _, _ := newTestRenderer(b)
	root := ir.NewCancel(source.NewRange(0, 3), glyph("x", ir.ClassNormal))
	root.Cross = true
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if _, err := renderer.Layout(root, ir.Inline, testStyle(16)); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkLayoutBox(b *testing.B) {
	renderer, _, _ := newTestRenderer(b)
	root := ir.NewBox(source.NewRange(0, 3), glyph("x", ir.ClassNormal))
	root.Padding = [4]ir.Length{irEm(0.1), irEm(0.1), irEm(0.1), irEm(0.1)}
	root.HasFill = true
	root.HasBorder = true
	root.Radius = irEm(0.1)
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if _, err := renderer.Layout(root, ir.Inline, testStyle(16)); err != nil {
			b.Fatal(err)
		}
	}
}
