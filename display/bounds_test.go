package display

import "testing"

func TestBoxBoundsIncludeCertifiedOverflow(t *testing.T) {
	box := &Box{
		Width:    20,
		Ascent:   10,
		Descent:  4,
		Overflow: 3,
	}
	bounds := box.Bounds()
	if bounds.X != -3 || bounds.Y != -13 || bounds.Width != 26 || bounds.Height != 20 {
		t.Fatalf("Bounds() = %+v", bounds)
	}
	if got := (*Box)(nil).Bounds(); got.X != 0 || got.Y != 0 || got.Width != 0 || got.Height != 0 {
		t.Fatalf("nil Bounds() = %+v", got)
	}
}
