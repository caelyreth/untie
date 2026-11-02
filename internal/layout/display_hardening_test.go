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

func TestLayoutPreservesNegativeItalicCorrection(t *testing.T) {
	t.Parallel()

	renderer, glyphs, _ := newTestRenderer(t)
	glyphs.glyphs["x"] = Glyph{
		ID:            7,
		FontID:        "test-math",
		Advance:       em(1, 2),
		Ascent:        em(7, 10),
		Descent:       em(2, 10),
		Italic:        -em(1, 20),
		ExactGlyphRef: true,
		TextLike:      true,
	}
	root := ir.NewRow(source.NewRange(0, 1), glyph("x", ir.ClassNormal))
	style := testStyle(16)
	style.MaxWidth = 100
	box, err := renderer.Layout(root, ir.Display, style)
	if err != nil {
		t.Fatalf("Layout() error = %v", err)
	}
	if err := box.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
	if !closeHost(box.Italic, -0.8) {
		t.Fatalf("Box.Italic = %v, want -0.8", box.Italic)
	}
	if len(box.Lines) != 1 {
		t.Fatalf("Lines = %d, want 1", len(box.Lines))
	}
	if !closeHost(box.Lines[0].Box.Italic, -0.8) {
		t.Fatalf("line Italic = %v, want -0.8", box.Lines[0].Box.Italic)
	}
}

func TestDisplayFallbackReusesLineStorage(t *testing.T) {
	t.Parallel()

	path := make(geometry.Path, 300)
	for i := range path {
		path[i] = geometry.Segment{
			Op:     geometry.SegMoveTo,
			Points: [3]geometry.Point{{X: 1, Y: 2}},
		}
	}
	base := &display.Box{
		Items: []display.Item{{
			Kind: display.ItemGlyphPath,
			Path: path,
		}},
		Hits: []display.Hit{{Source: source.NewRange(0, 1)}},
	}
	state := &layoutState{
		ctx:      context.Background(),
		request:  display.Style{MaxWidth: 1},
		hostSize: hostSubunits,
		limits:   source.DefaultLimits(),
	}
	line, err := state.singleDisplayLine(base, 1)
	if err != nil {
		t.Fatal(err)
	}
	if &line.Items[0] != &base.Items[0] || &line.Items[0].Path[0] != &base.Items[0].Path[0] {
		t.Fatal("singleDisplayLine copied item or path storage")
	}
	if &line.Hits[0] != &base.Hits[0] {
		t.Fatal("singleDisplayLine copied hit storage")
	}

	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	state.ctx = canceled
	if _, err := state.singleDisplayLine(base, 1); !errors.Is(err, context.Canceled) {
		t.Fatalf("singleDisplayLine(canceled) error = %v", err)
	}
}

func TestDisplayOverflowScanCancellation(t *testing.T) {
	t.Parallel()

	state := &layoutState{
		ctx:      &countdownContext{Context: context.Background(), remaining: 1},
		hostSize: hostSubunits,
		limits:   source.DefaultLimits(),
	}
	items := make([]display.Item, 300)
	for i := range items {
		items[i] = display.Item{
			Kind:  display.ItemRule,
			Bound: geometry.Rect{Width: 1, Height: 1},
			Rect:  geometry.Rect{Width: 1, Height: 1},
		}
	}
	_, err := state.displayItemsOverflow(items, 1, 1, 1)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("displayItemsOverflow() error = %v, want context.Canceled", err)
	}
}

func TestDisplayParentItalicComesFromFinalLine(t *testing.T) {
	t.Parallel()

	state := &layoutState{
		ctx:      context.Background(),
		hostSize: hostSubunits,
		limits:   source.DefaultLimits(),
		request:  display.Style{MaxWidth: 10},
	}
	parent, err := state.assembleDisplayLines([]display.LineBox{
		{Box: &display.Box{Ascent: 1, Descent: 1, Italic: 0.25}},
		{Box: &display.Box{Ascent: 1, Descent: 1, Italic: -0.5}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if parent.Italic != -0.5 {
		t.Fatalf("parent Italic = %v, want final line correction -0.5", parent.Italic)
	}
}
