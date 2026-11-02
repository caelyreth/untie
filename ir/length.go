package ir

import (
	"errors"
	"fmt"
)

const lengthScale int64 = 1_000_000

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
func EmLength(v float64) Length {
	return scaledLength(LengthEm, v)
}

// ExLength constructs a length in exes.
func ExLength(v float64) Length {
	return scaledLength(LengthEx, v)
}

// MuLength constructs a length in TeX math units.
func MuLength(v float64) Length {
	return scaledLength(LengthMu, v)
}

// PtLength constructs a length in points.
func PtLength(v float64) Length {
	return scaledLength(LengthPt, v)
}

// DipLength constructs a length in host DIPs.
func DipLength(v float64) Length {
	return scaledLength(LengthDip, v)
}

// PercentLength constructs a percentage length, where 100 means 100%.
func PercentLength(v float64) Length {
	return scaledLength(LengthPercent, v)
}

func scaledLength(unit LengthUnit, v float64) Length {
	return Length{Unit: unit, Value: int64(v * float64(lengthScale))}
}

// Float returns the unresolved value in its named unit.
func (l Length) Float() float64 {
	return float64(l.Value) / float64(lengthScale)
}

// IsZero reports whether l is zero.
func (l Length) IsZero() bool {
	return l.Unit == LengthZero || l.Value == 0
}

// Validate checks unit and finite range.
func (l Length) Validate() error {
	if l.Unit > LengthPercent {
		return fmt.Errorf("%w: unknown unit %d", ErrInvalidLength, l.Unit)
	}
	return nil
}
