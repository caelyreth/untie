package fixture

import (
	"encoding/json"
	"errors"
	"fmt"
	"unicode/utf8"

	"github.com/caelyreth/untie/source"
)

// ErrInvalidUTF8 reports a fixture source that cannot be represented as a
// JavaScript/TypeScript string. UTF-16 offsets have no meaning for such input.
var ErrInvalidUTF8 = errors.New("fixture: source is not valid UTF-8")

// UTF16Range is a half-open range in UTF-16 code units.
type UTF16Range struct {
	Start int
	End   int
}

// UnmarshalJSON decodes a compact [start, end] UTF-16 range.
func (r *UTF16Range) UnmarshalJSON(data []byte) error {
	var values []json.RawMessage
	if err := json.Unmarshal(data, &values); err != nil {
		return fmt.Errorf("UTF-16 range: %w", err)
	}
	if len(values) != 2 {
		return fmt.Errorf("UTF-16 range has %d values; want 2", len(values))
	}
	start, err := parseInt(values[0], "UTF-16 start")
	if err != nil {
		return err
	}
	end, err := parseInt(values[1], "UTF-16 end")
	if err != nil {
		return err
	}
	decoded := UTF16Range{Start: start, End: end}
	if err := decoded.Validate(); err != nil {
		return err
	}
	*r = decoded
	return nil
}

// MarshalJSON restores the compact [start, end] representation.
func (r UTF16Range) MarshalJSON() ([]byte, error) {
	if err := r.Validate(); err != nil {
		return nil, err
	}
	return json.Marshal([]int{r.Start, r.End})
}

// Validate checks that the range is non-negative and ordered.
func (r UTF16Range) Validate() error {
	if r.Start < 0 || r.End < r.Start {
		return fmt.Errorf("invalid UTF-16 range [%d,%d)", r.Start, r.End)
	}
	return nil
}

// UTF16BoundaryError reports a UTF-16 offset that does not lie on a code-point
// boundary, typically the middle of a surrogate pair.
type UTF16BoundaryError struct {
	Start  int
	End    int
	Offset int
}

func (e *UTF16BoundaryError) Error() string {
	return fmt.Sprintf("fixture: UTF-16 offset %d is not a code-point boundary in [%d,%d)", e.Offset, e.Start, e.End)
}

// UTF16OffsetsToByteRange converts a half-open UTF-16 range to UTF-8 byte
// offsets. The complete source must be valid UTF-8. Both offsets must be on
// code-point boundaries and within the source.
func UTF16OffsetsToByteRange(src string, start, end int) (source.Range, error) {
	index, err := newUTF16Index(src)
	if err != nil {
		return source.Range{}, err
	}
	return index.byteRange(start, end)
}

type utf16Index struct {
	boundaries map[int]int
	units      int
}

func newUTF16Index(src string) (*utf16Index, error) {
	boundaries, units, err := utf16Boundaries(src)
	if err != nil {
		return nil, err
	}
	return &utf16Index{boundaries: boundaries, units: units}, nil
}

func (i *utf16Index) byteRange(start, end int) (source.Range, error) {
	if start < 0 || end < start || end > i.units {
		return source.Range{}, fmt.Errorf("fixture: invalid UTF-16 range [%d,%d) for %d code units", start, end, i.units)
	}
	startByte, ok := i.boundaries[start]
	if !ok {
		return source.Range{}, &UTF16BoundaryError{Start: 0, End: i.units, Offset: start}
	}
	endByte, ok := i.boundaries[end]
	if !ok {
		return source.Range{}, &UTF16BoundaryError{Start: 0, End: i.units, Offset: end}
	}
	return source.NewRange(startByte, endByte), nil
}

// ConvertUTF16Range converts r to UTF-8 byte offsets.
func ConvertUTF16Range(src string, r UTF16Range) (source.Range, error) {
	return UTF16OffsetsToByteRange(src, r.Start, r.End)
}

func utf16Boundaries(src string) (map[int]int, int, error) {
	boundaries := map[int]int{0: 0}
	units := 0
	for byteOffset := 0; byteOffset < len(src); {
		r, width := utf8.DecodeRuneInString(src[byteOffset:])
		if r == utf8.RuneError && width == 1 {
			return nil, 0, fmt.Errorf("%w at byte %d", ErrInvalidUTF8, byteOffset)
		}
		units++
		if r > 0xffff {
			units++
		}
		byteOffset += width
		boundaries[units] = byteOffset
	}
	return boundaries, units, nil
}
