package font

import (
	"encoding/binary"
	"fmt"
	"math"
)

func readGlyphCount(maxp []byte) (int, error) {
	if len(maxp) < 6 {
		return 0, fmt.Errorf("maxp table is truncated")
	}
	count := int(binary.BigEndian.Uint16(maxp[4:6]))
	if count == 0 {
		return 0, fmt.Errorf("maxp table declares no glyphs")
	}
	return count, nil
}

func readFontBounds(head []byte) (Bounds, error) {
	if len(head) < 44 {
		return Bounds{}, fmt.Errorf("head table is truncated")
	}
	return Bounds{
		XMin: int32(int16(binary.BigEndian.Uint16(head[36:38]))),
		YMin: -int32(int16(binary.BigEndian.Uint16(head[42:44]))),
		XMax: int32(int16(binary.BigEndian.Uint16(head[40:42]))),
		YMax: -int32(int16(binary.BigEndian.Uint16(head[38:40]))),
	}, nil
}

func roundedMetric(value float32, name string) (int32, error) {
	number := float64(value)
	if math.IsNaN(number) || math.IsInf(number, 0) {
		return 0, fmt.Errorf("%s is not finite", name)
	}
	rounded := math.Round(number)
	if rounded < math.MinInt32 || rounded > math.MaxInt32 {
		return 0, fmt.Errorf("%s is outside the supported design-unit range", name)
	}
	return int32(rounded), nil
}
