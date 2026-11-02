package layout

import (
	"errors"
	"math"

	"github.com/caelyreth/untie/geometry"
)

var (
	// ErrInvalidInput reports malformed layout input.
	ErrInvalidInput = errors.New("layout: invalid input")
	// ErrMissingGlyph reports a glyph source miss.
	ErrMissingGlyph = errors.New("layout: missing glyph")
	// ErrUnsupported reports a construct owned by a later work package.
	ErrUnsupported = errors.New("layout: unsupported construct")
	// ErrLimit reports a configured resource limit.
	ErrLimit = errors.New("layout: limit exceeded")
)

var (
	// ThinSpace is the neutral TeX/OpenType thin math space (3/18 em).
	ThinSpace geometry.Em = 10_923
	// MediumSpace is the neutral medium math space (4/18 em).
	MediumSpace geometry.Em = 14_564
	// ThickSpace is the neutral thick math space (5/18 em).
	ThickSpace geometry.Em = 18_204
)

func add(a, b geometry.Em) (geometry.Em, error) {
	return a.Add(b)
}

func sub(a geometry.Em, rest ...geometry.Em) (geometry.Em, error) {
	out := a
	for _, value := range rest {
		if value == math.MinInt64 {
			return 0, geometry.ErrOverflow
		}
		var err error
		out, err = out.Add(-value)
		if err != nil {
			return 0, err
		}
	}
	return out, nil
}

func mulDiv(a geometry.Em, num, den int64) (geometry.Em, error) {
	return a.MulDiv(num, den)
}

func scale(a, factor geometry.Em) (geometry.Em, error) {
	return mulDiv(a, int64(factor), int64(geometry.EmScale))
}

func unscale(a, factor geometry.Em) (geometry.Em, error) {
	if factor == 0 {
		return 0, geometry.ErrInvalidScale
	}
	return mulDiv(a, int64(geometry.EmScale), int64(factor))
}

func midpoint(extent geometry.Em) geometry.Em {
	return extent / 2
}

func max(a, b geometry.Em) geometry.Em {
	if a > b {
		return a
	}
	return b
}

func clamp(v, lo, hi geometry.Em) geometry.Em {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func sum(values ...geometry.Em) (geometry.Em, error) {
	var out geometry.Em
	for _, value := range values {
		var err error
		out, err = add(out, value)
		if err != nil {
			return 0, err
		}
	}
	return out, nil
}
