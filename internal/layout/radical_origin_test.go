package layout

import (
	"context"
	"testing"

	"github.com/caelyreth/untie/display"
	"github.com/caelyreth/untie/geometry"
	"github.com/caelyreth/untie/ir"
	"github.com/caelyreth/untie/source"
)

func TestFlattenUsesCenteredGlyphOrigin(t *testing.T) {
	t.Parallel()

	renderer, err := New(Fonts{
		Glyphs:  &fakeGlyphSource{glyphs: map[string]Glyph{}},
		Metrics: fakeStyleFont{},
	}, Options{})
	if err != nil {
		t.Fatal(err)
	}
	state := &layoutState{
		ctx:      context.Background(),
		renderer: renderer,
		request:  display.Style{Size: 16, GlyphRefs: true},
		hostSize: 16 * hostSubunits,
		limits:   source.DefaultLimits(),
	}
	fragment := emptyFragment(Text, geometry.Em(geometry.EmScale))
	glyph := &GlyphFragment{
		Glyph: Glyph{
			ID:            7,
			FontID:        "test-math",
			Ascent:        em(1, 2),
			Descent:       em(1, 2),
			OriginY:       -em(1, 10),
			ExactGlyphRef: true,
			Outline: Path{
				{Op: geometry.SegMoveTo, Points: [3]Point{{X: 0, Y: -em(1, 2)}}},
				{Op: geometry.SegLineTo, Points: [3]Point{{X: em(1, 1), Y: -em(1, 2)}}},
				{Op: geometry.SegLineTo, Points: [3]Point{{X: em(1, 1), Y: em(1, 2)}}},
				{Op: geometry.SegLineTo, Points: [3]Point{{X: 0, Y: em(1, 2)}}},
				{Op: geometry.SegClose},
			},
		},
		Scale: geometry.Em(geometry.EmScale),
		Paint: ir.Paint{},
	}
	if err := fragment.pushGlyph(0, 0, glyph); err != nil {
		t.Fatal(err)
	}
	box, err := state.flatten(fragment)
	if err != nil {
		t.Fatal(err)
	}
	var ref *display.Item
	for i := range box.Items {
		if box.Items[i].Kind == display.ItemGlyphRef {
			ref = &box.Items[i]
			break
		}
	}
	if ref == nil {
		t.Fatal("flatten emitted no glyph reference")
	}
	if !closeHost(ref.At.Y, -1.6) {
		t.Fatalf("glyph origin y = %v, want -1.6", ref.At.Y)
	}
}
