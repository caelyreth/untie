package fixture

import (
	"math"
	"testing"
)

func TestPointsToDIP(t *testing.T) {
	t.Parallel()
	tests := []struct {
		points float64
		want   float64
	}{
		{points: 0, want: 0},
		{points: 1, want: 4.0 / 3.0},
		{points: 3, want: 4},
		{points: -1.5, want: -2},
	}
	for _, test := range tests {
		got, err := PointsToDIP(test.points)
		if err != nil {
			t.Fatalf("PointsToDIP(%v): %v", test.points, err)
		}
		if math.Abs(got-test.want) > 1e-12 {
			t.Fatalf("PointsToDIP(%v) = %v, want %v", test.points, got, test.want)
		}
		roundTrip, err := DIPToPoints(got)
		if err != nil {
			t.Fatalf("DIPToPoints(%v): %v", got, err)
		}
		if math.Abs(roundTrip-test.points) > 1e-12 {
			t.Fatalf("round trip = %v, want %v", roundTrip, test.points)
		}
	}
}

func TestPointsToDIPRejectsNonFinite(t *testing.T) {
	t.Parallel()
	for _, value := range []float64{math.NaN(), math.Inf(1), math.Inf(-1), math.MaxFloat64} {
		if _, err := PointsToDIP(value); err == nil {
			t.Fatalf("PointsToDIP(%v) unexpectedly succeeded", value)
		}
	}
}

func TestFrameToDIPConvertsAllPhysicalLengths(t *testing.T) {
	t.Parallel()
	f := Frame{
		Box: &FrameBox{
			Anchor: "baseline", Width: ptr(3), Ascent: ptr(1), Descent: ptr(0.5),
		},
		FontSize: 11,
		Fonts:    []string{"NewCMMath-Book"},
		Glyphs: []FrameGlyph{{
			Font: 0, Glyph: 1, X: 3, Y: -1.5, Size: 6, Text: "x",
			Fill: "#000000", HasFill: true,
			Transform: [4]float64{1, 0, 0, 1}, HasTransform: true,
		}},
		Shapes: []FrameShape{{
			Line:   []float64{1, 2, 3, 4},
			Stroke: &ShapeStroke{Color: "#000000", Thickness: 0.75},
		}},
	}
	got, err := FrameToDIP(f)
	if err != nil {
		t.Fatal(err)
	}
	if *got.Box.Width != 4 || got.FontSize != 4.0/3.0*11 {
		t.Fatalf("box/font size not converted: %#v", got)
	}
	if got.Glyphs[0].X != 4 || got.Glyphs[0].Y != -2 || got.Glyphs[0].Size != 8 {
		t.Fatalf("glyph not converted: %#v", got.Glyphs[0])
	}
	if got.Glyphs[0].Transform != f.Glyphs[0].Transform {
		t.Fatal("unitless transform changed")
	}
	if got.Shapes[0].Line[1] != 8.0/3.0 || got.Shapes[0].Stroke.Thickness != 1 {
		t.Fatalf("shape not converted: %#v", got.Shapes[0])
	}
}

func ptr(value float64) *float64 {
	return &value
}
