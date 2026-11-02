package fixture

import (
	"errors"
	"testing"

	"github.com/caelyreth/untie/source"
)

func TestUTF16OffsetsToByteRange(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		src   string
		start int
		end   int
		want  source.Range
	}{
		{name: "empty", src: "", start: 0, end: 0, want: source.NewRange(0, 0)},
		{name: "ascii", src: "abc", start: 1, end: 2, want: source.NewRange(1, 2)},
		{name: "bmp multibyte", src: "πx", start: 0, end: 1, want: source.NewRange(0, 2)},
		{name: "supplementary", src: "𝑥", start: 0, end: 2, want: source.NewRange(0, 4)},
		{name: "around supplementary", src: "A𝑥B", start: 1, end: 3, want: source.NewRange(1, 5)},
		{name: "combining", src: "e\u0301x", start: 0, end: 2, want: source.NewRange(0, 3)},
		{name: "zwj emoji", src: "👩‍👩‍👧x", start: 0, end: 8, want: source.NewRange(0, 18)},
		{name: "end", src: "𝑥", start: 2, end: 2, want: source.NewRange(4, 4)},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			got, err := UTF16OffsetsToByteRange(test.src, test.start, test.end)
			if err != nil {
				t.Fatalf("UTF16OffsetsToByteRange: %v", err)
			}
			if got != test.want {
				t.Fatalf("range = %#v, want %#v", got, test.want)
			}
		})
	}
}

func TestUTF16OffsetsRejectInvalidBoundaries(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		src   string
		start int
		end   int
	}{
		{name: "negative start", src: "x", start: -1, end: 1},
		{name: "end before start", src: "xy", start: 2, end: 1},
		{name: "past end", src: "xy", start: 0, end: 3},
		{name: "inside surrogate start", src: "𝑥", start: 1, end: 2},
		{name: "inside surrogate end", src: "𝑥", start: 0, end: 1},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			_, err := UTF16OffsetsToByteRange(test.src, test.start, test.end)
			if err == nil {
				t.Fatal("expected an error")
			}
			var boundary *UTF16BoundaryError
			if test.name == "inside surrogate start" || test.name == "inside surrogate end" {
				if !errors.As(err, &boundary) {
					t.Fatalf("error = %T %v, want UTF16BoundaryError", err, err)
				}
			}
		})
	}
}

func TestUTF16OffsetsRejectInvalidUTF8(t *testing.T) {
	t.Parallel()
	_, err := UTF16OffsetsToByteRange(string([]byte{'a', 0xff, 'b'}), 0, 1)
	if !errors.Is(err, ErrInvalidUTF8) {
		t.Fatalf("error = %v, want ErrInvalidUTF8", err)
	}
}
