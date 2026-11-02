package layout

import (
	"errors"
	"math"
	"testing"

	"github.com/caelyreth/untie/geometry"
)

func TestFixedPointArithmetic(t *testing.T) {
	t.Parallel()

	half := geometry.Em(geometry.EmScale / 2)
	scaled, err := scale(half, em(4, 5))
	if err != nil {
		t.Fatalf("scale() error = %v", err)
	}
	if want := em(2, 5); scaled < want-1 || scaled > want+1 {
		t.Fatalf("scale() = %d, want %d (+/-1)", scaled, want)
	}
	unscaled, err := unscale(scaled, em(4, 5))
	if err != nil {
		t.Fatalf("unscale() error = %v", err)
	}
	if unscaled < half-1 || unscaled > half+1 {
		t.Fatalf("unscale() = %d, want %d (+/-1)", unscaled, half)
	}
	total, err := sum(em(1, 4), em(2, 4), em(1, 4))
	if err != nil {
		t.Fatalf("sum() error = %v", err)
	}
	if total != geometry.Em(geometry.EmScale) {
		t.Fatalf("sum() = %d, want %d", total, geometry.EmScale)
	}
	difference, err := sub(em(3, 4), em(1, 4), em(1, 4))
	if err != nil {
		t.Fatalf("sub() error = %v", err)
	}
	if difference != em(1, 4) {
		t.Fatalf("sub() = %d, want %d", difference, em(1, 4))
	}
}

func TestFixedPointOverflowIsReported(t *testing.T) {
	t.Parallel()

	_, err := add(geometry.Em(math.MaxInt64), 1)
	if !errors.Is(err, geometry.ErrOverflow) {
		t.Fatalf("add() error = %v, want overflow", err)
	}
	_, err = sub(geometry.Em(math.MinInt64), 1)
	if !errors.Is(err, geometry.ErrOverflow) {
		t.Fatalf("sub() error = %v, want overflow", err)
	}
	_, err = unscale(1, 0)
	if !errors.Is(err, geometry.ErrInvalidScale) {
		t.Fatalf("unscale() error = %v, want invalid scale", err)
	}
}
