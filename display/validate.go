package display

import (
	"errors"
	"fmt"
	"math"

	"github.com/caelyreth/untie/geometry"
)

var ErrInvalidBox = errors.New("display: invalid box")

// Validate checks structural and numeric output invariants.
func (b *Box) Validate() error {
	if b == nil {
		return fmt.Errorf("%w: nil box", ErrInvalidBox)
	}
	for _, field := range []struct {
		name  string
		value float32
	}{
		{"width", b.Width}, {"ascent", b.Ascent}, {"descent", b.Descent},
		{"italic", b.Italic}, {"axis-height", b.AxisHeight}, {"overflow", b.Overflow},
	} {
		if !finite(field.value) || field.value < 0 {
			return fmt.Errorf("%w: %s is %v", ErrInvalidBox, field.name, field.value)
		}
	}
	for i, item := range b.Items {
		if err := validateItem(item); err != nil {
			return fmt.Errorf("%w: item %d: %v", ErrInvalidBox, i, err)
		}
	}
	for i, hit := range b.Hits {
		if err := validateHit(hit); err != nil {
			return fmt.Errorf("%w: hit %d: %v", ErrInvalidBox, i, err)
		}
	}
	for i, breakPoint := range b.Breaks {
		if err := validateBreak(breakPoint); err != nil {
			return fmt.Errorf("%w: break %d: %v", ErrInvalidBox, i, err)
		}
	}
	return nil
}

func validateHit(hit Hit) error {
	if !hit.Source.IsValid() {
		return errors.New("invalid source range")
	}
	if err := validateRect(hit.Area); err != nil {
		return fmt.Errorf("area: %v", err)
	}
	if hit.Depth < 0 {
		return errors.New("negative depth")
	}
	if hit.Kind > HitOther {
		return fmt.Errorf("unknown kind %d", hit.Kind)
	}
	return nil
}

func validateBreak(breakPoint Break) error {
	if !finite(breakPoint.X) || breakPoint.X < 0 {
		return errors.New("invalid x position")
	}
	if breakPoint.Kind > BreakExplicit {
		return fmt.Errorf("unknown kind %d", breakPoint.Kind)
	}
	return nil
}

func validateItem(item Item) error {
	if item.Kind > ItemGlyphRef {
		return fmt.Errorf("unknown kind %d", item.Kind)
	}
	if err := validateRect(item.Bound); err != nil {
		return fmt.Errorf("bound: %v", err)
	}
	switch item.Kind {
	case ItemGlyphPath:
		if len(item.Path) == 0 {
			return errors.New("empty glyph path")
		}
		for i, seg := range item.Path {
			if seg.Op > geometry.SegClose {
				return fmt.Errorf("segment %d has unknown op %d", i, seg.Op)
			}
			points := 1
			switch seg.Op {
			case geometry.SegQuadTo:
				points = 2
			case geometry.SegCubeTo:
				points = 3
			case geometry.SegClose:
				points = 0
			}
			for j := 0; j < points; j++ {
				if !finite(seg.Points[j].X) || !finite(seg.Points[j].Y) {
					return fmt.Errorf("segment %d has non-finite point", i)
				}
			}
		}
	case ItemRule:
		return validateRect(item.Rect)
	case ItemGlyphRef:
		if item.FontID == "" {
			return errors.New("glyph ref has empty font ID")
		}
		if !finite(item.At.X) || !finite(item.At.Y) || !finite(item.Em) || item.Em <= 0 {
			return errors.New("glyph ref has invalid placement")
		}
	}
	return nil
}

func validateRect(r geometry.Rect) error {
	for _, value := range []float32{r.X, r.Y, r.Width, r.Height} {
		if !finite(value) {
			return errors.New("non-finite value")
		}
	}
	if r.Width < 0 || r.Height < 0 {
		return errors.New("negative size")
	}
	return nil
}

func finite(v float32) bool {
	value := float64(v)
	return !math.IsNaN(value) && !math.IsInf(value, 0)
}
