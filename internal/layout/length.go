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
	if value.Value < 0 {
		return 0, fmt.Errorf("%w: negative structural length", ErrInvalidInput)
	}
	return s.signedLength(value, style, metrics)
}

func (s *layoutState) signedLength(value ir.Length, style Style, metrics MathMetrics) (geometry.Em, error) {
	return s.convertLength(value, style, metrics, true)
}

// baseLength converts a source length to unscaled base ems. Absolute units
// already produce base ems; relative units are not multiplied by the current
// math-style scale. Use this for ports whose result is scaled once later.
func (s *layoutState) baseLength(value ir.Length, style Style, metrics MathMetrics) (geometry.Em, error) {
	return s.convertLength(value, style, metrics, false)
}

func (s *layoutState) convertLength(
	value ir.Length,
	style Style,
	metrics MathMetrics,
	scaleRelative bool,
) (geometry.Em, error) {
	if err := value.Validate(); err != nil {
		return 0, fmt.Errorf("%w: %v", ErrInvalidInput, err)
	}
	if value.Unit == ir.LengthZero || value.Value == 0 {
		return 0, nil
	}
	base, err := geometry.EmFromParts(value.Value, 1_000_000)
	if err != nil {
		return 0, err
	}
	switch value.Unit {
	case ir.LengthEm:
		if scaleRelative {
			return scale(base, s.scale(style))
		}
		return base, nil
	case ir.LengthMu:
		base, err = mulDiv(base, 1, 18)
		if err != nil {
			return 0, err
		}
		if scaleRelative {
			return scale(base, s.scale(style))
		}
		return base, nil
	case ir.LengthEx:
		base, err = scale(base, metrics.XHeight)
		if err != nil {
			return 0, err
		}
		if scaleRelative {
			return scale(base, s.scale(style))
		}
		return base, nil
	case ir.LengthDip:
		return mulDiv(base, hostSubunits, s.hostSize)
	case ir.LengthPt:
		base, err = mulDiv(base, 4, 3)
		if err != nil {
			return 0, err
		}
		return mulDiv(base, hostSubunits, s.hostSize)
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
