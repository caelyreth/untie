package display

import (
	"fmt"
	"math"

	"github.com/caelyreth/untie/geometry"
)

// PixelGrid describes an optional device-pixel grid for thin rules and stems.
//
// Scale is the host-unit-to-output-pixel scale and PixelRatio is the additional
// device-pixel ratio. The zero value disables snapping. Both fields must be
// positive and finite when either is set.
type PixelGrid struct {
	Scale      float32
	PixelRatio float32
}

// Enabled reports whether the grid requests any snapping.
func (p PixelGrid) Enabled() bool {
	return p.Scale != 0 || p.PixelRatio != 0
}

// Validate checks pixel-grid configuration before layout.
func (p PixelGrid) Validate() error {
	if !p.Enabled() {
		return nil
	}
	for _, field := range []struct {
		name  string
		value float32
	}{
		{"scale", p.Scale},
		{"pixel ratio", p.PixelRatio},
	} {
		value := float64(field.value)
		if math.IsNaN(value) || math.IsInf(value, 0) || field.value <= 0 {
			return fmt.Errorf("%s must be finite and positive", field.name)
		}
	}
	return nil
}

// SnapAxis identifies the minor axis of a thin rule that can be aligned.
type SnapAxis uint8

const (
	// SnapNone means the item is not a conservative snapping candidate.
	SnapNone SnapAxis = iota
	// SnapX aligns a vertical rule's x position and width.
	SnapX
	// SnapY aligns a horizontal rule's y position and thickness.
	SnapY
)

// SnapRect aligns the minor axis of a thin rule to the configured grid.
//
// Snapping is deliberately conservative: only rules whose minor thickness is
// between 0.25 and 3 physical pixels are changed. The start edge moves to the
// nearest grid boundary, and thickness rounds to the nearest physical-pixel
// count with a one-pixel minimum. The major axis and all non-rule geometry are
// unchanged. An unset or invalid axis/grid is a no-op.
func (p PixelGrid) SnapRect(rect geometry.Rect, axis SnapAxis) geometry.Rect {
	if !p.Enabled() || axis != SnapX && axis != SnapY {
		return rect
	}
	pixelsPerUnit := float64(p.Scale) * float64(p.PixelRatio)
	if math.IsNaN(pixelsPerUnit) || math.IsInf(pixelsPerUnit, 0) || pixelsPerUnit <= 0 {
		return rect
	}
	step := 1 / pixelsPerUnit
	switch axis {
	case SnapX:
		rect.X, rect.Width = snapSpan(rect.X, rect.Width, step)
	case SnapY:
		rect.Y, rect.Height = snapSpan(rect.Y, rect.Height, step)
	}
	return rect
}

func snapSpan(start, size float32, step float64) (float32, float32) {
	pixels := float64(size) / step
	if math.IsNaN(pixels) || math.IsInf(pixels, 0) || pixels < 0.25 || pixels > 3 {
		return start, size
	}
	snappedPixels := math.Floor(pixels + 0.5)
	if snappedPixels < 1 {
		snappedPixels = 1
	}
	snappedSize := float32(snappedPixels * step)
	snappedStart := float32(math.Floor(float64(start)/step+0.5) * step)
	return snappedStart, snappedSize
}
