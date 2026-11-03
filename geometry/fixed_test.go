package geometry

import (
	"math"
	"testing"
)

func TestEmFromParts(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		numerator   int64
		denominator int64
		want        Em
	}{
		{name: "one", numerator: 1, denominator: 1, want: Em(EmScale)},
		{name: "half", numerator: 1, denominator: 2, want: Em(EmScale / 2)},
		{name: "negative", numerator: -1, denominator: 4, want: Em(-EmScale / 4)},
		{name: "rounded", numerator: 1, denominator: 3, want: Em(21845)},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := EmFromParts(tc.numerator, tc.denominator)
			if err != nil {
				t.Fatalf("EmFromParts() error = %v", err)
			}
			if got != tc.want {
				t.Fatalf("EmFromParts() = %d, want %d", got, tc.want)
			}
		})
	}
}

func TestEmErrors(t *testing.T) {
	t.Parallel()

	if _, err := EmFromParts(1, 0); err != ErrInvalidScale {
		t.Fatalf("zero denominator error = %v, want %v", err, ErrInvalidScale)
	}
	if _, err := Em(math.MaxInt64).MulDiv(2, 1); err != ErrOverflow {
		t.Fatalf("overflow error = %v, want %v", err, ErrOverflow)
	}
	if _, err := (Em(1)).Add(Em(math.MaxInt64)); err != ErrOverflow {
		t.Fatalf("add overflow error = %v, want %v", err, ErrOverflow)
	}
	if _, err := Em(math.MinInt64).Abs(); err != ErrOverflow {
		t.Fatalf("Abs(MinInt64) error = %v, want %v", err, ErrOverflow)
	}
}

func TestEmMulDivPowerOfTwoRoundingAndOverflow(t *testing.T) {
	t.Parallel()

	tests := []struct {
		value Em
		num   int64
		den   int64
		want  Em
	}{
		{value: 3, num: 1, den: 2, want: 2},
		{value: -3, num: 1, den: 2, want: -2},
		{value: 5, num: 1, den: 4, want: 1},
		{value: 6, num: 1, den: 4, want: 2},
		{value: Em(math.MaxInt64), num: 2, den: 2, want: Em(math.MaxInt64)},
	}
	for _, test := range tests {
		got, err := test.value.MulDiv(test.num, test.den)
		if err != nil {
			t.Fatalf("%d*%d/%d error = %v", test.value, test.num, test.den, err)
		}
		if got != test.want {
			t.Fatalf("%d*%d/%d = %d, want %d", test.value, test.num, test.den, got, test.want)
		}
	}
	if _, err := Em(math.MaxInt64).MulDiv(3, 2); err != ErrOverflow {
		t.Fatalf("power-of-two overflow error = %v, want %v", err, ErrOverflow)
	}
	if got, err := Em(math.MinInt64).MulDiv(1, 1); err != nil || got != Em(math.MinInt64) {
		t.Fatalf("MinInt64 MulDiv = %d, %v; want %d, nil", got, err, Em(math.MinInt64))
	}
}

func TestEmFloat(t *testing.T) {
	t.Parallel()

	if got := Em(EmScale * 3 / 2).Float(); got != 1.5 {
		t.Fatalf("Float() = %v, want 1.5", got)
	}
}

func TestDipConversions(t *testing.T) {
	t.Parallel()

	size, err := DipFromFloat(16)
	if err != nil {
		t.Fatalf("DipFromFloat() error = %v", err)
	}
	if got := size.Float(); got != 16 {
		t.Fatalf("Dip.Float() = %v, want 16", got)
	}
	if got := size.Float32(); got != 16 {
		t.Fatalf("Dip.Float32() = %v, want 16", got)
	}
	got, err := EmToDip(Em(EmScale/2), size)
	if err != nil {
		t.Fatalf("EmToDip() error = %v", err)
	}
	if got != Dip(DipScale*8) {
		t.Fatalf("EmToDip() = %v, want %v", got, Dip(DipScale*8))
	}
	if _, err := EmToDip(1, 0); err != ErrInvalidScale {
		t.Fatalf("EmToDip(zero size) error = %v, want %v", err, ErrInvalidScale)
	}
}

func TestDipFromFloatRejectsInvalidValues(t *testing.T) {
	t.Parallel()

	for _, value := range []float64{
		math.NaN(),
		math.Inf(1),
		math.Inf(-1),
		float64(MaxDimension) + 1,
		math.MaxFloat64,
		-math.MaxFloat64,
	} {
		if _, err := DipFromFloat(value); err == nil {
			t.Fatalf("DipFromFloat(%v) error = nil", value)
		}
	}
	if _, err := EmFromParts(MaxDimension+1, 1); err != ErrDimension {
		t.Fatalf("EmFromParts(dimension overflow) error = %v, want %v", err, ErrDimension)
	}
}

func TestFontUnitsToEm(t *testing.T) {
	t.Parallel()

	got, err := FontUnitsToEm(500, 1000)
	if err != nil {
		t.Fatalf("FontUnitsToEm() error = %v", err)
	}
	if got != Em(EmScale/2) {
		t.Fatalf("FontUnitsToEm() = %d, want %d", got, Em(EmScale/2))
	}
	if _, err := FontUnitsToEm(1, 0); err != ErrInvalidScale {
		t.Fatalf("zero UPEM error = %v, want %v", err, ErrInvalidScale)
	}
}
