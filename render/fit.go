package render

import (
	"errors"
	"fmt"

	"github.com/caelyreth/untie/display"
	"github.com/caelyreth/untie/geometry"
)

var (
	// ErrInvalidViewport reports a nil, non-finite, or non-positive viewport.
	ErrInvalidViewport = errors.New("render: invalid viewport")
	// ErrInvalidFitOptions reports invalid padding or alignment.
	ErrInvalidFitOptions = errors.New("render: invalid fit options")
)

// Align is horizontal placement inside the padded viewport.
type Align uint8

const (
	AlignStart Align = iota
	AlignCenter
	AlignEnd
)

// FitOptions controls placement inside a viewport. Padding is applied equally
// on every side. The box is always centered vertically.
type FitOptions struct {
	Padding float32
	Align   Align
}

// Transform maps box coordinates into viewport coordinates as
// offset + point*scale.
type Transform struct {
	Scale  float32
	Offset geometry.Point
}

// Apply maps one point from box coordinates into viewport coordinates.
func (t Transform) Apply(point geometry.Point) geometry.Point {
	return geometry.Point{
		X: t.Offset.X + point.X*t.Scale,
		Y: t.Offset.Y + point.Y*t.Scale,
	}
}

// ApplyRect maps an axis-aligned rectangle from box coordinates into viewport
// coordinates.
func (t Transform) ApplyRect(rect geometry.Rect) geometry.Rect {
	return geometry.Rect{
		X:      t.Offset.X + rect.X*t.Scale,
		Y:      t.Offset.Y + rect.Y*t.Scale,
		Width:  rect.Width * t.Scale,
		Height: rect.Height * t.Scale,
	}
}

// Fit returns a uniform transform that tightly fits Box.Bounds(), including
// certified overflow, inside viewport after padding.
//
// Fit may enlarge small boxes. A box with zero width and height uses scale 1.
// The viewport must have finite positive dimensions, and padding must leave a
// finite positive content area.
func Fit(box *display.Box, viewport geometry.Rect, options FitOptions) (Transform, error) {
	if box == nil {
		return Transform{}, fmt.Errorf("%w: nil box", ErrInvalidBox)
	}
	if !validViewport(viewport) {
		return Transform{}, ErrInvalidViewport
	}
	padding := options.Padding
	if !finite(padding) || padding < 0 || padding >= viewport.Width/2 || padding >= viewport.Height/2 {
		return Transform{}, fmt.Errorf("%w: padding %v does not leave a positive content area",
			ErrInvalidFitOptions, padding)
	}
	if options.Align > AlignEnd {
		return Transform{}, fmt.Errorf("%w: unknown alignment %d", ErrInvalidFitOptions, options.Align)
	}

	bounds := box.Bounds()
	if !validBounds(bounds) {
		return Transform{}, fmt.Errorf("%w: bounds are invalid", ErrInvalidBox)
	}
	availableWidth := viewport.Width - 2*padding
	availableHeight := viewport.Height - 2*padding
	scale, err := fitScale(bounds, availableWidth, availableHeight)
	if err != nil {
		return Transform{}, err
	}

	centerX := viewport.X + viewport.Width/2
	centerY := viewport.Y + viewport.Height/2
	var offsetX float32
	switch options.Align {
	case AlignStart:
		offsetX = viewport.X + padding - bounds.X*scale
	case AlignCenter:
		offsetX = centerX - (bounds.X+bounds.Width/2)*scale
	case AlignEnd:
		offsetX = viewport.X + viewport.Width - padding - (bounds.X+bounds.Width)*scale
	}
	offsetY := centerY - (bounds.Y+bounds.Height/2)*scale
	if !finite(scale) || scale <= 0 || !finite(offsetX) || !finite(offsetY) {
		return Transform{}, fmt.Errorf("%w: transform is not finite", ErrInvalidBox)
	}
	return Transform{
		Scale:  scale,
		Offset: geometry.Point{X: offsetX, Y: offsetY},
	}, nil
}

func validViewport(viewport geometry.Rect) bool {
	return finite(viewport.X) &&
		finite(viewport.Y) &&
		finite(viewport.Width) &&
		finite(viewport.Height) &&
		viewport.Width > 0 &&
		viewport.Height > 0
}

func validBounds(bounds geometry.Rect) bool {
	return finite(bounds.X) &&
		finite(bounds.Y) &&
		finite(bounds.Width) &&
		finite(bounds.Height) &&
		bounds.Width >= 0 &&
		bounds.Height >= 0
}

func fitScale(bounds geometry.Rect, availableWidth, availableHeight float32) (float32, error) {
	scale := float32(1)
	fitted := false
	if bounds.Width > 0 {
		scale = availableWidth / bounds.Width
		fitted = true
	}
	if bounds.Height > 0 {
		heightScale := availableHeight / bounds.Height
		if !fitted || heightScale < scale {
			scale = heightScale
		}
	}
	if !finite(scale) || scale <= 0 {
		return 0, fmt.Errorf("%w: box cannot be fitted", ErrInvalidBox)
	}
	return scale, nil
}
