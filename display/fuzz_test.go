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
	f.Add([]byte{0, 0, 0, 0x80})
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
		alt := randomAlt(data)
		if err := alt.Validate(); err == nil {
			altFirst, err := alt.MarshalCanonical()
			if err != nil {
				t.Fatalf("alt canonicalization: %v", err)
			}
			altSecond, err := alt.Clone().MarshalCanonical()
			if err != nil {
				t.Fatalf("alt clone canonicalization: %v", err)
			}
			if !bytes.Equal(altFirst, altSecond) {
				t.Fatal("alt clone changed canonical output")
			}
		}
	})
}

func randomAlt(data []byte) Alt {
	return Alt{
		Speech: string(data),
		Text:   string(data),
	}
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
		return float32(int8(next())) / 8
	}
	box := &Box{
		Width:  fuzzAbsFloat(number()) + 1,
		Ascent: 1,
	}
	itemCount := int(next() % 6)
	for range itemCount {
		item := Item{
			Kind:      ItemKind(next() % 3),
			PathIndex: NoPath,
			Paint: Paint{
				Role:        Role(next() % 3),
				Explicit:    RGBA{R: next(), G: next(), B: next(), A: next()},
				HasExplicit: next()%2 == 0,
			},
			Bound:  fuzzRect(number),
			Rect:   fuzzRect(number),
			FontID: string([]byte{'f', 'o', 'n', 't', next() % 4}),
			Glyph:  binary.BigEndian.Uint16([]byte{next(), next()}),
			At:     geometry.Point{X: number(), Y: number()},
			Em:     fuzzAbsFloat(number()) + 0.25,
		}
		if item.Kind == ItemGlyphPath {
			segments := make([]geometry.Segment, int(next()%4)+1)
			var bound fuzzBounds
			for i := range segments {
				segments[i].Op = geometry.SegOp(next() % 5)
				points, _ := segmentPointCount(segments[i].Op)
				for j := 0; j < points; j++ {
					segments[i].Points[j] = geometry.Point{X: number(), Y: number()}
					bound.add(segments[i].Points[j])
				}
			}
			item.PathIndex = len(box.Paths)
			box.Paths = append(box.Paths, PathTemplate{
				Segments: segments,
				Bound:    bound.rect(),
			})
		}
		box.Items = append(box.Items, item)
	}
	hitCount := int(next() % 6)
	for range hitCount {
		start := int(next() % 8)
		box.Hits = append(box.Hits, Hit{
			Source: source.NewRange(start, start+int(next()%4)+1),
			Area:   fuzzRect(number),
			Depth:  int(next() % 8),
			Kind:   HitKind(next() % (byte(HitOther) + 1)),
		})
	}

	switch next() % 3 {
	case 0:
		box.Lines = []LineBox{{
			Width:  box.Width,
			Ascent: box.Ascent,
			Items:  ItemRange{Count: len(box.Items)},
			Hits:   HitRange{Count: len(box.Hits)},
		}}
	case 1:
		tagStart := len(box.Items)
		hitStart := len(box.Hits)
		tagX := box.Width
		box.Items = append(box.Items, ruleItemAt(tagX, 1))
		box.Hits = append(box.Hits, ruleHitAt(tagX, 1))
		box.Tag = &TagBox{
			Source: source.NewRange(0, 1),
			Manual: true,
			Line:   FormulaTagLine,
			X:      tagX,
			Width:  1,
			Ascent: 1,
			Items:  ItemRange{Start: tagStart, Count: 1},
			Hits:   HitRange{Start: hitStart, Count: 1},
		}
		box.Width += 1
	case 2:
		lineCount := int(next()%3) + 1
		itemStart, hitStart := 0, 0
		box.Ascent = float32(lineCount)
		box.Descent = 0
		box.Overflow = 1_000
		box.Lines = make([]LineBox, lineCount)
		for i := range box.Lines {
			itemCount := len(box.Items) - itemStart
			hitCount := len(box.Hits) - hitStart
			if i < lineCount-1 {
				itemCount = int(next() % byte(itemCount+1))
				hitCount = int(next() % byte(hitCount+1))
			}
			box.Lines[i] = LineBox{
				Baseline: float32(i),
				Width:    box.Width,
				Ascent:   1,
				Overflow: 1_000,
				Items:    ItemRange{Start: itemStart, Count: itemCount},
				Hits:     HitRange{Start: hitStart, Count: hitCount},
			}
			itemStart += itemCount
			hitStart += hitCount
		}
	}
	return box
}

type fuzzBounds struct {
	minX float32
	minY float32
	maxX float32
	maxY float32
	have bool
}

func (b *fuzzBounds) add(point geometry.Point) {
	if !b.have {
		b.minX, b.maxX = point.X, point.X
		b.minY, b.maxY = point.Y, point.Y
		b.have = true
		return
	}
	b.minX = minFloat(b.minX, point.X)
	b.minY = minFloat(b.minY, point.Y)
	b.maxX = maxFloat(b.maxX, point.X)
	b.maxY = maxFloat(b.maxY, point.Y)
}

func (b fuzzBounds) rect() geometry.Rect {
	if !b.have {
		return geometry.Rect{}
	}
	return geometry.Rect{
		X:      b.minX,
		Y:      b.minY,
		Width:  b.maxX - b.minX,
		Height: b.maxY - b.minY,
	}
}

func fuzzRect(number func() float32) geometry.Rect {
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
