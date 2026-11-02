package fixture

import (
	"math"
	"testing"

	"github.com/caelyreth/untie/geometry"
)

func TestInspectPath(t *testing.T) {
	t.Parallel()
	path := geometry.Path{
		{Op: geometry.SegMoveTo, Points: [3]geometry.Point{{X: 1, Y: 2}}},
		{Op: geometry.SegLineTo, Points: [3]geometry.Point{{X: 5, Y: -1}}},
		{Op: geometry.SegQuadTo, Points: [3]geometry.Point{{X: 7, Y: 3}, {X: 9, Y: 4}}},
		{Op: geometry.SegClose},
	}
	info, err := InspectPath(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Segments != 4 || info.Subpaths != 1 || info.Closed != 1 || !info.HasBounds {
		t.Fatalf("info = %#v", info)
	}
	if info.Bounds.X != 1 || info.Bounds.Y != -1 || info.Bounds.Width != 8 || info.Bounds.Height != 5 {
		t.Fatalf("bounds = %#v", info.Bounds)
	}
}

func TestInspectPathRejectsInvalidInput(t *testing.T) {
	t.Parallel()
	tests := []geometry.Path{
		nil,
		{{Op: geometry.SegLineTo, Points: [3]geometry.Point{{X: 1, Y: 1}}}},
		{{Op: geometry.SegClose}},
		{{Op: geometry.SegMoveTo, Points: [3]geometry.Point{{X: float32(math.NaN())}}}},
		{{Op: geometry.SegOp(99)}},
	}
	for _, path := range tests {
		if _, err := InspectPath(path); err == nil {
			t.Fatalf("path %#v unexpectedly accepted", path)
		}
	}
}
