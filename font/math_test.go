package font

import (
	"encoding/binary"
	"slices"
	"testing"
)

func TestParseMathKernInfoCornersAndRecords(t *testing.T) {
	t.Parallel()

	records, err := parseMathKernInfo(mathKernInfoFixture())
	if err != nil {
		t.Fatal(err)
	}
	if len(records) != 2 || records[3] == nil || records[7] == nil {
		t.Fatalf("kern records = %v, want glyphs 3 and 7", records)
	}
	for _, tc := range []struct {
		glyph   GlyphID
		corner  MathKernCorner
		heights []int32
		kerns   []int32
	}{
		{3, MathKernTopRight, nil, []int32{-11}},
		{3, MathKernTopLeft, nil, []int32{-22}},
		{3, MathKernBottomRight, []int32{100}, []int32{-33, -44}},
		{3, MathKernBottomLeft, nil, []int32{-55}},
		{7, MathKernTopRight, nil, nil},
		{7, MathKernTopLeft, nil, nil},
		{7, MathKernBottomRight, nil, nil},
		{7, MathKernBottomLeft, nil, []int32{-11}},
	} {
		got := records[tc.glyph].corners[tc.corner]
		if tc.kerns == nil {
			if got != nil {
				t.Errorf("glyph %d corner %d = %#v, want absent", tc.glyph, tc.corner, got)
			}
			continue
		}
		if got == nil || !slices.Equal(got.Heights, tc.heights) || !slices.Equal(got.Kerns, tc.kerns) {
			t.Errorf("glyph %d corner %d = %#v, want heights %v, kerns %v", tc.glyph, tc.corner, got, tc.heights, tc.kerns)
		}
	}
}

func TestParseMathKernInfoEmpty(t *testing.T) {
	t.Parallel()

	for _, data := range [][]byte{
		{0, 4, 0, 0, 0, 1, 0, 0},                                // Empty coverage and no records.
		{0, 12, 0, 1, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1, 0, 1, 0, 3}, // All corners NULL.
	} {
		records, err := parseMathKernInfo(data)
		if err != nil {
			t.Fatal(err)
		}
		if len(records) != 0 {
			t.Fatalf("kern records = %v, want empty", records)
		}
	}
}

func TestParseMathKernInfoRejectsInvalidCounts(t *testing.T) {
	t.Parallel()

	for _, count := range []uint16{0, 1, 3, 65535} {
		data := mathKernInfoFixture()
		binary.BigEndian.PutUint16(data[2:4], count)
		if _, err := parseMathKernInfo(data); err == nil {
			t.Errorf("accepted record count %d with coverage count 2", count)
		}
	}
	// Coverage is complete, but the record array cannot fit after the header.
	if _, err := parseMathKernInfo([]byte{0, 4, 0, 1, 0, 1, 0, 1, 0, 3}); err == nil {
		t.Fatal("accepted a truncated record array")
	}
}

func TestParseMathKernInfoRejectsTruncatedData(t *testing.T) {
	t.Parallel()

	data := mathKernInfoFixture()
	for size := 0; size < len(data); size++ {
		if _, err := parseMathKernInfo(data[:size]); err == nil {
			t.Errorf("accepted table truncated to %d bytes", size)
		}
	}
	for _, offset := range []uint16{uint16(len(data)), uint16(len(data) + 1), 65535} {
		bad := slices.Clone(data)
		binary.BigEndian.PutUint16(bad[4:6], offset)
		if _, err := parseMathKernInfo(bad); err == nil {
			t.Errorf("accepted out-of-range kern offset %d", offset)
		}
	}
}

// The first glyph has distinct curves at all four corners. The second shares
// its bottom-left curve with the first glyph's top-right curve. Offsets are
// relative to MathKernInfo, whose four-byte header includes the record count.
func mathKernInfoFixture() []byte {
	words := []int16{
		20, 2, // Coverage offset and MathKernCount.
		28, 34, 40, 54, // Glyph 3: top-right, top-left, bottom-right, bottom-left.
		0, 0, 0, 28, // Glyph 7: bottom-left only.
		1, 2, 3, 7, // Coverage format 1, two glyphs.
		0, -11, 0, // Offset 28: constant curve.
		0, -22, 0, // Offset 34: constant curve.
		1, 100, 0, -33, 0, -44, 0, // Offset 40: one height and two kern values.
		0, -55, 0, // Offset 54: constant curve.
	}
	data := make([]byte, 2*len(words))
	for i, word := range words {
		binary.BigEndian.PutUint16(data[2*i:], uint16(word))
	}
	return data
}
