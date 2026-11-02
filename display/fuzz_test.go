package display

import (
	"bytes"
	"encoding/binary"
	"testing"

	"github.com/caelyreth/untie/geometry"
	"github.com/caelyreth/untie/source"
)

func FuzzBoxOperations(f *testing.F) {
	f.Add([]byte{})
	f.Add([]byte{1, 2, 3, 4, 5, 6, 7, 8})
	f.Add(bytes.Repeat([]byte{0xff}, 64))
	f.Fuzz(func(t *testing.T, data []byte) {
		box := randomBox(data)
		_ = box.Validate()
		_ = box.Clone()
		_, _ = box.At(geometry.Point{})
		_ = box.Covering(source.NewRange(0, 16))
		first, err := box.MarshalCanonical()
		if err != nil {
			return
		}
		second, err := box.Clone().MarshalCanonical()
		if err != nil {
			t.Fatalf("clone canonicalization: %v", err)
		}
		if !bytes.Equal(first, second) {
			t.Fatal("clone changed canonical output")
		}
	})
}

func randomBox(data []byte) *Box {
	next := func() byte {
		if len(data) == 0 {
			return 0
		}
		value := data[0]
		data = data[1:]
		return value
	}
	number := func() float32 {
		return float32(int8(next())) / 4
	}
	box := &Box{
		Width: fuzzAbsFloat(number()), Ascent: fuzzAbsFloat(number()), Descent: fuzzAbsFloat(number()),
		Italic: fuzzAbsFloat(number()), AxisHeight: fuzzAbsFloat(number()), Overflow: fuzzAbsFloat(number()),
	}
	itemCount := int(next() % 8)
	for range itemCount {
		item := Item{
			Kind: ItemKind(next() % 3),
			Paint: Paint{
				Role:        Role(next() % 3),
				Explicit:    RGBA{R: next(), G: next(), B: next(), A: next()},
				HasExplicit: next()%2 == 0,
			},
			Bound:  rect(number),
			Rect:   rect(number),
			FontID: string([]byte{'f', 'o', 'n', 't', next() % 4}),
			Glyph:  binary.BigEndian.Uint16([]byte{next(), next()}),
			At:     geometry.Point{X: number(), Y: number()},
			Em:     fuzzAbsFloat(number()),
		}
		if item.Kind == ItemGlyphPath || next()%2 == 0 {
			item.Path = make([]geometry.Segment, int(next()%5))
			for i := range item.Path {
				item.Path[i].Op = geometry.SegOp(next() % 5)
				for j := range item.Path[i].Points {
					item.Path[i].Points[j] = geometry.Point{X: number(), Y: number()}
				}
			}
		}
		box.Items = append(box.Items, item)
	}
	hitCount := int(next() % 8)
	for range hitCount {
		start := int(next() % 16)
		box.Hits = append(box.Hits, Hit{
			Source: source.NewRange(start, start+int(next()%8)),
			Area:   rect(number),
			Depth:  int(next() % 8),
			Kind:   HitKind(next() % (byte(HitOther) + 1)),
		})
	}
	breakCount := int(next() % 8)
	for range breakCount {
		box.Breaks = append(box.Breaks, Break{
			X:       fuzzAbsFloat(number()),
			Penalty: int(int8(next())),
			Kind:    BreakKind(next() % (byte(BreakExplicit) + 1)),
		})
	}
	return box
}

func rect(number func() float32) geometry.Rect {
	return geometry.Rect{
		X: number(), Y: number(),
		Width: fuzzAbsFloat(number()), Height: fuzzAbsFloat(number()),
	}
}

func fuzzAbsFloat(value float32) float32 {
	if value < 0 {
		return -value
	}
	return value
}
