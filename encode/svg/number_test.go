package svg

import (
	"math"
	"math/rand"
	"strconv"
	"testing"
)

func TestAppendFixedFloat32MatchesStrconv(t *testing.T) {
	t.Parallel()

	random := rand.New(rand.NewSource(1))
	values := []float32{
		0,
		float32(math.Copysign(0, -1)),
		0.0005,
		-0.0005,
		0.125,
		-0.125,
		1.2345,
		-1.2345,
		999.999,
		-999.999,
		1_000_000,
		-1_000_000,
	}
	for range 20_000 {
		values = append(values, float32((random.Float64()*2-1)*1_000_000))
	}

	for _, value := range values {
		for precision := 1; precision <= 6; precision++ {
			got := string(appendFixedFloat32(nil, value, precision))
			want := strconvNumber(value, precision)
			if precision == 3 {
				want = trimLeadingZero(want)
			}
			if got != want {
				t.Fatalf("value=%v bits=%08x precision=%d: got %q, want %q",
					value, math.Float32bits(value), precision, got, want)
			}
		}
	}
}

func trimLeadingZero(number string) string {
	if len(number) > 1 && number[0] == '0' && number[1] == '.' {
		return number[1:]
	}
	if len(number) > 2 && number[0] == '-' && number[1] == '0' && number[2] == '.' {
		return "-" + number[2:]
	}
	return number
}

func TestAppendUintMatchesStrconv(t *testing.T) {
	t.Parallel()

	values := []uint64{0, 1, 9, 10, 99, 100, math.MaxUint32, math.MaxUint64}
	random := rand.New(rand.NewSource(2))
	for range 10_000 {
		values = append(values, random.Uint64())
	}
	for _, value := range values {
		got := string(appendUint(nil, value))
		want := strconv.FormatUint(value, 10)
		if got != want {
			t.Fatalf("value=%d: got %q, want %q", value, got, want)
		}
	}
}

func strconvNumber(value float32, precision int) string {
	number := strconv.AppendFloat(nil, float64(value), 'f', precision, 32)
	for len(number) > 0 && number[len(number)-1] == '0' {
		number = number[:len(number)-1]
	}
	if len(number) > 0 && number[len(number)-1] == '.' {
		number = number[:len(number)-1]
	}
	if len(number) == 0 {
		return "0"
	}
	return string(number)
}
