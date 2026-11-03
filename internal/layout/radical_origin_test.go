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
	var path *display.Item
	for i := range box.Items {
		if box.Items[i].Kind == display.ItemGlyphPath && box.Items[i].Glyph == ref.Glyph {
			path = &box.Items[i]
			break
		}
	}
	if path == nil {
		t.Fatal("flatten emitted no glyph path")
	}
	if path.At != ref.At {
		t.Fatalf("path origin %+v differs from ref origin %+v", path.At, ref.At)
	}
	if path.Bound != ref.Bound {
		t.Fatalf("path bound %+v differs from ref bound %+v", path.Bound, ref.Bound)
	}
}

func TestFlattenPlacesGlyphPathAndRefAtSourceOrigin(t *testing.T) {
	t.Parallel()

	renderer, glyphs, _ := newTestRenderer(t)
	glyphs.glyphs["axis"] = Glyph{
		ID:            9,
		FontID:        "origin-face",
		Advance:       em(1, 1),
		Ascent:        em(1, 2),
		Descent:       em(1, 2),
		OriginY:       -em(1, 10),
		TextLike:      true,
		ExactGlyphRef: true,
		Outline: Path{
			{Op: geometry.SegMoveTo, Points: [3]Point{{X: 0, Y: -em(1, 2)}}},
			{Op: geometry.SegLineTo, Points: [3]Point{{X: em(1, 1), Y: -em(1, 2)}}},
			{Op: geometry.SegLineTo, Points: [3]Point{{X: em(1, 1), Y: em(1, 2)}}},
			{Op: geometry.SegLineTo, Points: [3]Point{{X: 0, Y: em(1, 2)}}},
			{Op: geometry.SegClose},
		},
	}
	style := testStyle(16)
	style.GlyphRefs = true
	box, err := renderer.Layout(glyph("axis", ir.ClassNormal), ir.Inline, style)
	if err != nil {
		t.Fatalf("Layout() error = %v", err)
	}
	var pathItem, refItem *display.Item
	for i := range box.Items {
		switch box.Items[i].Kind {
		case display.ItemGlyphPath:
			pathItem = &box.Items[i]
		case display.ItemGlyphRef:
			refItem = &box.Items[i]
		}
	}
	if pathItem == nil || refItem == nil {
		t.Fatalf("items = %+v, want path and reference", box.Items)
	}
	for name, item := range map[string]*display.Item{"path": pathItem, "ref": refItem} {
		if !closeHost(item.At.Y, -1.6) {
			t.Fatalf("%s origin y = %v, want -1.6", name, item.At.Y)
		}
	}
	if !closeHost(pathItem.Bound.Y, -9.6) || !closeHost(pathItem.Bound.Height, 16) {
		t.Fatalf("path bound = %+v, want y=-9.6 height=16", pathItem.Bound)
	}
	if got := box.Paths[pathItem.PathIndex].Bound.Y; !closeHost(got, -0.5) {
		t.Fatalf("template bound y = %v, want -0.5", got)
	}
}
