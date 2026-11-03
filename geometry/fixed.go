// Package geometry defines deterministic geometric primitives.
package geometry

import (
	"errors"
	"math"
	"math/bits"
)

const (
	// EmScale is the number of fixed-point subunits in one em.
	EmScale int64 = 1 << 16
	// DipScale is the number of fixed-point subunits in one host DIP.
	DipScale int64 = 1 << 10
	// MaxDimension is the largest absolute coordinate accepted by public
	// constructors. Ir.Limits carries the same default and may apply a smaller
	// policy limit at a higher layer.
	MaxDimension int64 = 1 << 40
	MaxEm              = Em(MaxDimension * EmScale)
	MaxDip             = Dip(MaxDimension * DipScale)
)

var (
	ErrOverflow     = errors.New("geometry: overflow")
	ErrInvalidScale = errors.New("geometry: invalid scale")
	ErrInvalidFloat = errors.New("geometry: invalid floating-point value")
	ErrDimension    = errors.New("geometry: dimension out of range")
)

// Em is a signed fixed-point length in 1/EmScale em.
type Em int64

// Dip is a signed fixed-point length in 1/DipScale host DIP.
type Dip int64

// EmFromParts returns a value equal to numerator/denominator em.
func EmFromParts(numerator, denominator int64) (Em, error) {
	if denominator == 0 {
		return 0, ErrInvalidScale
	}
	value, err := mulDiv(numerator, EmScale, denominator)
	if err != nil {
		return 0, err
	}
	if value > int64(MaxEm) || value < -int64(MaxEm) {
		return 0, ErrDimension
	}
	return Em(value), nil
}

// Add returns a+b, reporting overflow.
func (a Em) Add(b Em) (Em, error) {
	if (b > 0 && a > Em(math.MaxInt64)-b) || (b < 0 && a < Em(math.MinInt64)-b) {
		return 0, ErrOverflow
	}
	return a + b, nil
}

// MulDiv returns a*num/den rounded to nearest, ties away from zero.
func (a Em) MulDiv(num, den int64) (Em, error) {
	if den == 0 {
		return 0, ErrInvalidScale
	}
	value, err := mulDiv(int64(a), num, den)
	if err != nil {
		return 0, err
	}
	return Em(value), nil
}

// Abs returns the absolute value of a, reporting overflow for MinInt64.
func (a Em) Abs() (Em, error) {
	if a == Em(math.MinInt64) {
		return 0, ErrOverflow
	}
	if a > MaxEm || a < -MaxEm {
		return 0, ErrDimension
	}
	if a < 0 {
		return -a, nil
	}
	return a, nil
}

// Float returns the value in ems.
func (a Em) Float() float64 {
	return float64(a) / float64(EmScale)
}

// DipFromFloat converts a finite, representable host-unit value to fixed-point
// DIP. Rounding is to nearest, with ties away from zero.
func DipFromFloat(v float64) (Dip, error) {
	if math.IsNaN(v) || math.IsInf(v, 0) {
		return 0, ErrInvalidFloat
	}
	if math.Abs(v) > float64(MaxDimension) {
		return 0, ErrDimension
	}
	scaled := math.Round(v * float64(DipScale))
	const int64Limit = float64(uint64(1) << 63)
	if scaled < -int64Limit || scaled >= int64Limit {
		return 0, ErrOverflow
	}
	return Dip(int64(scaled)), nil
}

// Float returns the value in DIPs.
func (d Dip) Float() float64 {
	return float64(d) / float64(DipScale)
}

// Float32 returns the value in DIPs as a host-friendly float32.
func (d Dip) Float32() float32 {
	return float32(d.Float())
}

// EmToDip scales a fixed-point em value by the root em size.
func EmToDip(em Em, emSize Dip) (Dip, error) {
	if emSize <= 0 {
		return 0, ErrInvalidScale
	}
	if emSize > MaxDip {
		return 0, ErrDimension
	}
	if em > MaxEm || em < -MaxEm {
		return 0, ErrDimension
	}
	value, err := mulDiv(int64(em), int64(emSize), EmScale)
	if err != nil {
		return 0, err
	}
	if value > int64(MaxDip) || value < -int64(MaxDip) {
		return 0, ErrDimension
	}
	return Dip(value), nil
}

// FontUnitsToEm converts integer font design units to fixed-point em.
func FontUnitsToEm(value int64, unitsPerEm int64) (Em, error) {
	if unitsPerEm <= 0 {
		return 0, ErrInvalidScale
	}
	scaled, err := mulDiv(value, EmScale, unitsPerEm)
	if err != nil {
		return 0, err
	}
	if scaled > int64(MaxEm) || scaled < -int64(MaxEm) {
		return 0, ErrDimension
	}
	return Em(scaled), nil
}

func mulDiv(a, b, c int64) (int64, error) {
	if a == 0 {
		return 0, nil
	}
	if c == 0 {
		return 0, ErrInvalidScale
	}
	if a == math.MinInt64 && b == math.MinInt64 {
		return 0, ErrOverflow
	}

	negative := (a < 0) != (b < 0)
	negative = negative != (c < 0)
	ua := abs64(a)
	ub := abs64(b)
	uc := abs64(c)
	hi, lo := bitsMul(ua, ub)
	var q, r uint64
	if uc&(uc-1) == 0 {
		shift := bits.TrailingZeros64(uc)
		if shift == 0 {
			if hi != 0 {
				return 0, ErrOverflow
			}
			q = lo
		} else {
			if hi >= uint64(1)<<shift {
				return 0, ErrOverflow
			}
			q = lo>>shift | hi<<(64-shift)
		}
		r = lo & (uc - 1)
	} else {
		if hi >= uc {
			return 0, ErrOverflow
		}
		q, r = bitsDiv(hi, lo, uc)
	}
	if r >= uc/2+uc%2 {
		if q == math.MaxUint64 {
			return 0, ErrOverflow
		}
		q++
	}
	if negative {
		if q > 1<<63 {
			return 0, ErrOverflow
		}
		if q == 1<<63 {
			return math.MinInt64, nil
		}
		return -int64(q), nil
	}
	if q > math.MaxInt64 {
		return 0, ErrOverflow
	}
	return int64(q), nil
}

func abs64(v int64) uint64 {
	if v < 0 {
		return uint64(-(v + 1)) + 1
	}
	return uint64(v)
}
