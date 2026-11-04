package main

import (
	"context"
	"testing"
	"time"

	"github.com/caelyreth/untie"
	"github.com/caelyreth/untie/display"
	"github.com/caelyreth/untie/geometry"
	"github.com/egoist/mygo/ui"
)

func TestStreamRendersMarkdownAndMath(t *testing.T) {
	model, err := newApp(mathFont)
	if err != nil {
		t.Fatal(err)
	}
	tester := ui.NewTester(model.view, 980, 760)
	tester.Frame()
	if !tester.HasText("Streaming Markdown") || !tester.HasText("Render stream") {
		t.Fatalf("initial texts = %q", tester.Texts())
	}
	if err := tester.Click("Render stream"); err != nil {
		t.Fatal(err)
	}
	if model.page != renderPage {
		t.Fatal("render button did not open the render page")
	}
	tester.Advance(2 * time.Second)
	if model.shown == 0 || model.morph.current == nil {
		t.Fatalf("stream did not reach math: shown=%d current=%v", model.shown, model.morph.current != nil)
	}
	tester.Advance(6 * time.Second)
	if !model.complete || model.streaming {
		t.Fatalf("stream state = complete:%v streaming:%v", model.complete, model.streaming)
	}
	if !tester.HasText("complete") {
		t.Fatalf("completed texts = %q", tester.Texts())
	}
	if err := tester.Click("Edit source"); err != nil {
		t.Fatal(err)
	}
	if model.page != editorPage {
		t.Fatal("edit button did not return to the editor page")
	}
}

func TestFormulaMorphKeepsPreviousBox(t *testing.T) {
	model, err := newApp(mathFont)
	if err != nil {
		t.Fatal(err)
	}
	first, err := model.renderer.Render(context.Background(), `x`, untie.Display, untie.Style{})
	if err != nil {
		t.Fatal(err)
	}
	second, err := model.renderer.Render(context.Background(), `x^2`, untie.Display, untie.Style{})
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	var morph formulaMorph
	morph.update(first.Box, `x`, now, true)
	morph.update(second.Box, `x^2`, now.Add(16*time.Millisecond), true)
	if morph.previousBox != first.Box || morph.currentBox != second.Box {
		t.Fatalf("morph boxes = previous:%p current:%p", morph.previousBox, morph.currentBox)
	}
	if len(morph.previous) == 0 || len(morph.current) == 0 {
		t.Fatalf("morph commands = previous:%d current:%d", len(morph.previous), len(morph.current))
	}
	previousKeys := make(map[string]struct{}, len(morph.previous))
	for _, shape := range morph.previous {
		previousKeys[shape.key] = struct{}{}
	}
	shared := false
	for _, shape := range morph.current {
		if _, ok := previousKeys[shape.key]; ok {
			shared = true
			break
		}
	}
	if !shared {
		t.Fatal("successive layouts shared no stable morph command")
	}
	if morph.source != `x^2` || morph.started.IsZero() {
		t.Fatalf("morph state = %+v", morph)
	}
}

func TestMorphHidesStreamingErrorPaint(t *testing.T) {
	t.Parallel()

	box := &untie.Box{Items: []display.Item{
		{Kind: display.ItemRule, Paint: display.Paint{Role: display.RoleError}},
		{Kind: display.ItemRule, Paint: display.Paint{Role: display.RoleInk}},
	}}
	if shapes := extractMorphShapes(box, true); len(shapes) != 1 {
		t.Fatalf("streaming shapes = %d, want only the non-error rule", len(shapes))
	}
	if shapes := extractMorphShapes(box, false); len(shapes) != 2 {
		t.Fatalf("completed shapes = %d, want both rules", len(shapes))
	}
}

func TestMorphUsesNaturalScaleForTallMath(t *testing.T) {
	t.Parallel()

	box := &untie.Box{Width: 700, Ascent: 1200, Descent: 544}
	var morph formulaMorph
	morph.currentBox = box
	if height := morph.height(); height != 1788 {
		t.Fatalf("morph height = %v, want natural content height", height)
	}
	if width := morph.contentWidth(); width != 744 {
		t.Fatalf("morph width = %v, want natural scrollable width", width)
	}
	x, y, scale := fitTransform(box, ui.Rect{X: 10, Y: 20, W: 900, H: 240})
	if scale != 1 {
		t.Fatalf("scale = %v, want natural scale", scale)
	}
	if x != 110 || y != 1242 {
		t.Fatalf("origin = (%v,%v), want centered top origin (110,1242)", x, y)
	}
}

func TestMorphInterpolatesGlyphPointsAndRuleBounds(t *testing.T) {
	t.Parallel()

	a := geometry.Path{
		{Op: geometry.SegMoveTo, Points: [3]geometry.Point{{X: 0, Y: 0}}},
		{Op: geometry.SegLineTo, Points: [3]geometry.Point{{X: 10, Y: 0}}},
	}
	b := geometry.Path{
		{Op: geometry.SegMoveTo, Points: [3]geometry.Point{{X: 4, Y: 2}}},
		{Op: geometry.SegLineTo, Points: [3]geometry.Point{{X: 18, Y: 6}}},
	}
	path := lerpPath(a, b, 0.5)
	if path[0].Points[0] != (geometry.Point{X: 2, Y: 1}) ||
		path[1].Points[0] != (geometry.Point{X: 14, Y: 3}) {
		t.Fatalf("morphed path = %#v", path)
	}
	bar := lerpRect(
		geometry.Rect{X: 0, Y: 10, Width: 20, Height: 1},
		geometry.Rect{X: 4, Y: 14, Width: 32, Height: 3},
		0.5,
	)
	if bar != (geometry.Rect{X: 2, Y: 12, Width: 26, Height: 2}) {
		t.Fatalf("morphed fraction bar = %#v", bar)
	}

	matches := matchMorphShapes(
		[]morphShape{{key: "old", kind: morphRule, bound: geometry.Rect{X: 2, Y: 4}}},
		[]morphShape{{key: "new", kind: morphRule, bound: geometry.Rect{X: 3, Y: 4}}},
	)
	if matches[0] != 0 {
		t.Fatalf("changed fraction bar was not matched: %v", matches)
	}
}
