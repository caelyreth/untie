package display

import (
	"bytes"
	"encoding/binary"
	"math"
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
		return float32(int8(next())) / 4
	}
	if next()%5 == 0 {
		return fuzzTaggedConstrainedBox()
	}
	box := &Box{
		Width: fuzzAbsFloat(number()), Ascent: fuzzAbsFloat(number()), Descent: fuzzAbsFloat(number()),
		Italic: number(), AxisHeight: fuzzAbsFloat(number()), Overflow: fuzzAbsFloat(number()),
		Alt: Alt{Speech: string(data), Text: string(data)},
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
			X:    fuzzAbsFloat(number()),
			Kind: BreakKind(next() % (byte(BreakExplicit) + 1)),
		})
	}
	if next()%3 == 0 {
		line := &Box{
			Width:      box.Width,
			Ascent:     box.Ascent,
			Descent:    box.Descent,
			Italic:     box.Italic,
			AxisHeight: box.AxisHeight,
			Overflow:   box.Overflow,
		}
		box.Lines = []LineBox{{
			Baseline: 0,
			OffsetX:  fuzzAbsFloat(number()),
			Box:      line,
		}}
	} else if next()%4 == 0 {
		tag := &TagBox{
			Box: &Box{
				Width:  1,
				Ascent: 1,
				Items: []Item{{
					Kind:  ItemRule,
					Paint: Paint{Role: RoleInk},
					Bound: geometry.Rect{Y: -1, Width: 1, Height: 0.1},
					Rect:  geometry.Rect{Y: -1, Width: 1, Height: 0.1},
				}},
				Hits: []Hit{{
					Source: source.NewRange(0, 1),
					Area:   geometry.Rect{Y: -1, Width: 1, Height: 1},
					Kind:   HitAtom,
				}},
			},
			Source: source.NewRange(0, 1),
			Manual: true,
			Line:   FormulaTagLine,
			X:      fuzzAbsFloat(number()),
		}
		for _, item := range tag.Box.Items {
			translated := item
			translated.Bound.X += tag.X
			translated.Rect.X += tag.X
			translated.At.X += tag.X
			box.Items = append(box.Items, translated)
		}
		for _, hit := range tag.Box.Hits {
			translated := hit
			translated.Area.X += tag.X
			box.Hits = append(box.Hits, translated)
		}
		box.Tag = tag
		box.Italic = tag.Box.Italic
		box.Width = maxFloat(box.Width, tag.X+tag.Box.Width)
		box.Ascent = maxFloat(box.Ascent, tag.Box.Ascent)
		box.Overflow = math.MaxFloat32 / 4
	}
	return box
}

func fuzzTaggedConstrainedBox() *Box {
	lineItem := Item{
		Kind:  ItemRule,
		Paint: Paint{Role: RoleInk},
		Bound: geometry.Rect{Y: -1, Width: 1, Height: 0.1},
		Rect:  geometry.Rect{Y: -1, Width: 1, Height: 0.1},
	}
	line := &Box{
		Width:  1,
		Ascent: 1,
		Items:  []Item{lineItem},
		Hits: []Hit{{
			Source: source.NewRange(0, 1),
			Area:   geometry.Rect{Y: -1, Width: 1, Height: 1},
			Kind:   HitAtom,
		}},
	}
	tagItem := lineItem
	tagItem.Bound.X = 2
	tagItem.Rect.X = 2
	tagItem.At.X = 2
	tag := &TagBox{
		Box: &Box{
			Width:  1,
			Ascent: 1,
			Items:  []Item{lineItem},
			Hits: []Hit{{
				Source: source.NewRange(0, 1),
				Area:   geometry.Rect{Y: -1, Width: 1, Height: 1},
				Kind:   HitAtom,
			}},
		},
		Source: source.NewRange(0, 1),
		Manual: true,
		Line:   FormulaTagLine,
		X:      2,
	}
	return &Box{
		Width:  3,
		Ascent: 1,
		Lines:  []LineBox{{Box: line, Tag: tag}},
		Items:  []Item{lineItem, tagItem},
		Hits: []Hit{
			{
				Source: source.NewRange(0, 1),
				Area:   geometry.Rect{Y: -1, Width: 1, Height: 1},
				Kind:   HitAtom,
			},
			{
				Source: source.NewRange(0, 1),
				Area:   geometry.Rect{X: 2, Y: -1, Width: 1, Height: 1},
				Kind:   HitAtom,
			},
		},
		Tag: tag,
	}
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
