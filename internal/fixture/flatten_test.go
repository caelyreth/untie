package fixture

import "testing"

func TestFlattenFrameUsesFirstUseFontOrderAndPreservesPaintOrder(t *testing.T) {
	t.Parallel()
	fill := "#ff0000"
	transform := [4]float64{1, 0, 0, 1}
	layout := FrameLayout{
		Box:      FrameBox{Anchor: "baseline", Width: ptr(10), Ascent: ptr(7), Descent: ptr(1)},
		FontSize: 11,
		Page:     [2]float64{10, 8},
		Glyphs: []GlyphPlacement{
			{Font: "second", Glyph: 2, Size: 11, Text: "b"},
			{Font: "first", Glyph: 1, Size: 11, Text: "a", Fill: &fill, Transform: &transform},
			{Font: "second", Glyph: 3, Size: 7, Text: "c"},
		},
		Shapes: []FrameShape{{
			Rect:   []float64{0, 0, 10, 1},
			Stroke: &ShapeStroke{Color: "#000000", Thickness: 0.5},
		}},
	}
	frame, err := FlattenFrame(layout)
	if err != nil {
		t.Fatal(err)
	}
	if len(frame.Fonts) != 2 || frame.Fonts[0] != "second" || frame.Fonts[1] != "first" {
		t.Fatalf("fonts = %v", frame.Fonts)
	}
	if frame.Glyphs[0].Font != 0 || frame.Glyphs[1].Font != 1 || frame.Glyphs[2].Font != 0 {
		t.Fatalf("font indices = %v", frame.Glyphs)
	}
	if frame.Glyphs[1].Fill != fill || !frame.Glyphs[1].HasTransform {
		t.Fatalf("fill/transform lost: %#v", frame.Glyphs[1])
	}
	if len(frame.Shapes) != 1 || frame.Shapes[0].ShapeGeometry() != "rect" {
		t.Fatalf("shapes = %#v", frame.Shapes)
	}
}

func TestFlattenFrameAddsDefaultFillForTransforms(t *testing.T) {
	t.Parallel()
	transform := [4]float64{1, 0, 0, -1}
	frame, err := FlattenFrame(FrameLayout{
		Box:      FrameBox{Anchor: "top", Width: ptr(1), Height: ptr(1)},
		FontSize: 10,
		Page:     [2]float64{1, 1},
		Glyphs: []GlyphPlacement{{
			Font: "face", Glyph: 1, Size: 10, Text: "x", Transform: &transform,
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !frame.Glyphs[0].HasFill || frame.Glyphs[0].Fill != "#000000" {
		t.Fatalf("default transform fill = %#v", frame.Glyphs[0])
	}
}

func TestFlattenFrameRejectsInvalidInput(t *testing.T) {
	t.Parallel()
	_, err := FlattenFrame(FrameLayout{
		Box:      FrameBox{Anchor: "top", Width: ptr(1), Height: ptr(1)},
		FontSize: 10,
		Page:     [2]float64{1, 1},
		Glyphs:   []GlyphPlacement{{Font: "", Glyph: 1, Size: 10}},
	})
	if err == nil {
		t.Fatal("expected an error")
	}
}
