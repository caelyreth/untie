package layout

import (
	"fmt"

	"github.com/caelyreth/untie/geometry"
	"github.com/caelyreth/untie/ir"
)

func (s *layoutState) length(value ir.Length, style Style, metrics MathMetrics) (geometry.Em, error) {
	if err := value.Validate(); err != nil {
		return 0, fmt.Errorf("%w: %v", ErrInvalidInput, err)
	}
	if value.Unit == ir.LengthZero || value.Value == 0 {
		return 0, nil
	}
	if value.Value < 0 {
		return 0, fmt.Errorf("%w: negative length", ErrInvalidInput)
	}

	base, err := geometry.EmFromParts(value.Value, 1_000_000)
	if err != nil {
		return 0, err
	}
	switch value.Unit {
	case ir.LengthEm:
		return scale(base, s.scale(style))
	case ir.LengthMu:
		base, err = mulDiv(base, 1, 18)
		if err != nil {
			return 0, err
		}
		return scale(base, s.scale(style))
	case ir.LengthEx:
		base, err = scale(base, metrics.XHeight)
		if err != nil {
			return 0, err
		}
		return scale(base, s.scale(style))
	case ir.LengthDip:
		return mulDiv(base, int64(geometry.EmScale), s.hostSize)
	case ir.LengthPt:
		base, err = mulDiv(base, 4, 3)
		if err != nil {
			return 0, err
		}
		return mulDiv(base, int64(geometry.EmScale), s.hostSize)
	case ir.LengthPercent:
		return 0, fmt.Errorf("%w: percentage length needs a construction context", ErrInvalidInput)
	default:
		return 0, fmt.Errorf("%w: unknown length unit %d", ErrInvalidInput, value.Unit)
	}
}

func (s *layoutState) orMetric(value ir.Length, fallback geometry.Em, style Style, metrics MathMetrics) (geometry.Em, error) {
	if value.Unit == ir.LengthZero || value.Value == 0 {
		return s.metricAt(fallback, style)
	}
	return s.length(value, style, metrics)
}
