package display

import "github.com/caelyreth/untie/geometry"

// Bounds returns the baseline-relative rectangle that contains the box's
// nominal geometry and its certified overflow. The result is suitable for
// fitting the complete display list into a host viewport.
func (b *Box) Bounds() geometry.Rect {
	if b == nil {
		return geometry.Rect{}
	}
	overflow := maxFloat(0, b.Overflow)
	return geometry.Rect{
		X:      -overflow,
		Y:      -b.Ascent - overflow,
		Width:  b.Width + 2*overflow,
		Height: b.Ascent + b.Descent + 2*overflow,
	}
}
