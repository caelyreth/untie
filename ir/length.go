package ir

import (
	"errors"
	"fmt"
	"math"

	"github.com/caelyreth/untie/source"
)

const lengthScale int64 = 1_000_000

const maxLengthValue int64 = (1 << 40) * lengthScale

// LengthUnit names the unit of an unresolved semantic length.
type LengthUnit uint8

const (
	LengthZero LengthUnit = iota
	LengthEm
	LengthEx
	LengthMu
	LengthPt
	LengthDip
	LengthPercent
)

var ErrInvalidLength = errors.New("ir: invalid length")

// Length stores a signed value in millionths of its unit. Frontends may perform
// deterministic arithmetic before lowering, but layout converts units.
type Length struct {
	Unit  LengthUnit
	Value int64
}

// RawLength constructs a length from millionths of unit.
func RawLength(unit LengthUnit, value int64) Length {
	if unit == LengthZero {
		return Length{}
	}
	return Length{Unit: unit, Value: value}
}

// EmLength constructs a length in ems.
func EmLength(v float64) (Length, error) {
	return scaledLength(LengthEm, v)
}

// ExLength constructs a length in exes.
func ExLength(v float64) (Length, error) {
	return scaledLength(LengthEx, v)
}

// MuLength constructs a length in TeX math units.
func MuLength(v float64) (Length, error) {
	return scaledLength(LengthMu, v)
}

// PtLength constructs a length in points.
func PtLength(v float64) (Length, error) {
	return scaledLength(LengthPt, v)
}

// DipLength constructs a length in host DIPs.
func DipLength(v float64) (Length, error) {
	return scaledLength(LengthDip, v)
}

// PercentLength constructs a percentage length, where 100 means 100%.
func PercentLength(v float64) (Length, error) {
	return scaledLength(LengthPercent, v)
}

func scaledLength(unit LengthUnit, v float64) (Length, error) {
	maxDimension := source.DefaultLimits().MaxDimension
	if math.IsNaN(v) || math.IsInf(v, 0) || math.Abs(v) > float64(maxDimension) {
		return Length{}, fmt.Errorf("%w: value %v outside allowed dimension", ErrInvalidLength, v)
	}
	scaled := math.Round(v * float64(lengthScale))
	const int64Limit = float64(uint64(1) << 63)
	if math.IsNaN(scaled) || math.IsInf(scaled, 0) || scaled < -int64Limit || scaled >= int64Limit {
		return Length{}, fmt.Errorf("%w: value %v overflows fixed-point range", ErrInvalidLength, v)
	}
	value := int64(scaled)
	if value > maxLengthValue || value < -maxLengthValue {
		return Length{}, fmt.Errorf("%w: value %v exceeds maximum dimension", ErrInvalidLength, v)
	}
	return Length{Unit: unit, Value: value}, nil
}

// Float returns the unresolved value in its named unit.
func (l Length) Float() float64 {
	return float64(l.Value) / float64(lengthScale)
}

// IsZero reports whether l is zero.
func (l Length) IsZero() bool {
	return l.Unit == LengthZero || l.Value == 0
}

// Validate checks the unit and the default maximum dimension.
func (l Length) Validate() error {
	return l.ValidateWithLimit(source.DefaultLimits().MaxDimension)
}

// ValidateWithLimit checks the unit and a caller-provided maximum absolute
// value in the length's named unit.
func (l Length) ValidateWithLimit(maxDimension int64) error {
	if l.Unit > LengthPercent {
		return fmt.Errorf("%w: unknown unit %d", ErrInvalidLength, l.Unit)
	}
	if l.Unit == LengthZero {
		if l.Value != 0 {
			return fmt.Errorf("%w: zero unit has non-zero value", ErrInvalidLength)
		}
		return nil
	}
	if maxDimension <= 0 {
		return fmt.Errorf("%w: non-positive maximum dimension", ErrInvalidLength)
	}
	maxValue := maxDimension
	if maxValue > math.MaxInt64/lengthScale {
		maxValue = math.MaxInt64
	} else {
		maxValue *= lengthScale
	}
	if l.Value > maxValue || l.Value < -maxValue {
		return fmt.Errorf("%w: value %d exceeds maximum dimension", ErrInvalidLength, l.Value)
	}
	return nil
}
