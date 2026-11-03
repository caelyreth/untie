package display

import (
	"math"
	"testing"

	"github.com/caelyreth/untie/geometry"
)

func TestPixelGridSnapRectIsConservativeAndDeterministic(t *testing.T) {
	t.Parallel()

	grid := PixelGrid{Scale: 1, PixelRatio: 2}
	tests := []struct {
		name string
		rect geometry.Rect
		axis SnapAxis
		want geometry.Rect
	}{
		{
			name: "horizontal rule",
			rect: geometry.Rect{X: 1.3, Y: 2.2, Width: 8.4, Height: 0.4},
			axis: SnapY,
			want: geometry.Rect{X: 1.3, Y: 2, Width: 8.4, Height: 0.5},
		},
		{
			name: "vertical stem",
			rect: geometry.Rect{X: -1.2, Y: 3.7, Width: 0.7, Height: 8.2},
			axis: SnapX,
			want: geometry.Rect{X: -1, Y: 3.7, Width: 0.5, Height: 8.2},
		},
		{
			name: "hairline remains untouched",
			rect: geometry.Rect{X: 1, Y: 1, Width: 10, Height: 0.1},
			axis: SnapY,
			want: geometry.Rect{X: 1, Y: 1, Width: 10, Height: 0.1},
		},
		{
			name: "thick rule remains untouched",
			rect: geometry.Rect{X: 1, Y: 1, Width: 10, Height: 2},
			axis: SnapY,
			want: geometry.Rect{X: 1, Y: 1, Width: 10, Height: 2},
		},
		{
			name: "no axis",
			rect: geometry.Rect{X: 1.2, Y: 2.2, Width: 8.4, Height: 0.4},
			axis: SnapNone,
			want: geometry.Rect{X: 1.2, Y: 2.2, Width: 8.4, Height: 0.4},
		},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			if got := grid.SnapRect(test.rect, test.axis); got != test.want {
				t.Fatalf("SnapRect() = %#v, want %#v", got, test.want)
			}
			if got := grid.SnapRect(test.rect, test.axis); got != test.want {
				t.Fatalf("SnapRect() is nondeterministic: %#v", got)
			}
		})
	}
}

func TestPixelGridZeroValueAndValidation(t *testing.T) {
	t.Parallel()

	rect := geometry.Rect{X: 1.2, Y: 2.2, Width: 8.4, Height: 0.4}
	var disabled PixelGrid
	if disabled.Enabled() {
		t.Fatal("zero PixelGrid is enabled")
	}
	if err := disabled.Validate(); err != nil {
		t.Fatalf("zero PixelGrid validation: %v", err)
	}
	if got := disabled.SnapRect(rect, SnapY); got != rect {
		t.Fatalf("disabled SnapRect() = %#v, want %#v", got, rect)
	}

	for _, grid := range []PixelGrid{
		{Scale: 1},
		{Scale: -1, PixelRatio: 1},
		{Scale: 1, PixelRatio: float32(math.NaN())},
		{Scale: float32(math.Inf(1)), PixelRatio: 1},
	} {
		if err := grid.Validate(); err == nil {
			t.Fatalf("Validate() accepted %+v", grid)
		}
	}
}
