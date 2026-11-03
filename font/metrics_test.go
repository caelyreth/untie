package font

import (
	"encoding/binary"
	"math"
	"testing"
)

func TestReadGlyphCount(t *testing.T) {
	maxp := make([]byte, 6)
	binary.BigEndian.PutUint16(maxp[4:6], 42)
	count, err := readGlyphCount(maxp)
	if err != nil {
		t.Fatal(err)
	}
	if count != 42 {
		t.Fatalf("readGlyphCount = %d, want 42", count)
	}

	binary.BigEndian.PutUint16(maxp[4:6], 0)
	if _, err := readGlyphCount(maxp); err == nil {
		t.Fatal("readGlyphCount accepted zero glyphs")
	}
	if _, err := readGlyphCount(maxp[:5]); err == nil {
		t.Fatal("readGlyphCount accepted a truncated table")
	}
}

func TestReadFontBoundsConvertsToYDown(t *testing.T) {
	head := make([]byte, 44)
	putInt16(head[36:38], -100)
	putInt16(head[38:40], -300)
	putInt16(head[40:42], 500)
	putInt16(head[42:44], 700)

	got, err := readFontBounds(head)
	if err != nil {
		t.Fatal(err)
	}
	want := Bounds{XMin: -100, YMin: -700, XMax: 500, YMax: 300}
	if got != want {
		t.Fatalf("readFontBounds = %#v, want %#v", got, want)
	}
	if _, err := readFontBounds(head[:43]); err == nil {
		t.Fatal("readFontBounds accepted a truncated table")
	}
}

func TestRoundedMetricRejectsInvalidValues(t *testing.T) {
	if got, err := roundedMetric(12.6, "test"); err != nil || got != 13 {
		t.Fatalf("roundedMetric = %d, %v", got, err)
	}
	for _, value := range []float32{
		float32(math.NaN()),
		float32(math.Inf(1)),
		float32(math.Inf(-1)),
	} {
		if _, err := roundedMetric(value, "test"); err == nil {
			t.Fatalf("roundedMetric accepted %v", value)
		}
	}
}

func putInt16(dst []byte, value int16) {
	binary.BigEndian.PutUint16(dst, uint16(value))
}
