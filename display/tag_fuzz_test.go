package display

import (
	"bytes"
	"testing"

	"github.com/caelyreth/untie/geometry"
	"github.com/caelyreth/untie/source"
)

func FuzzTaggedLineGraph(f *testing.F) {
	f.Add([]byte{})
	f.Add([]byte{0, 1, 2, 3, 4, 5})
	f.Add([]byte{255, 254, 253, 252, 251, 250})
	f.Fuzz(func(t *testing.T, data []byte) {
		box := fuzzValidTaggedLineGraph(data)
		if err := box.Validate(); err != nil {
			t.Fatalf("generated tagged line graph is invalid: %v", err)
		}
		if box.Tag == nil || len(box.Lines) != 1 || box.Lines[0].Tag == nil {
			t.Fatal("generated graph lost its tag/line relationship")
		}
		if box.Tag.Line != FormulaTagLine || box.Lines[0].Tag.Line != FormulaTagLine {
			t.Fatalf("tag lines = parent %d line %d", box.Tag.Line, box.Lines[0].Tag.Line)
		}
		if box.Lines[0].Box.Tag != nil {
			t.Fatal("generated line box contains a nested tag")
		}

		clone := box.Clone()
		if err := clone.Validate(); err != nil {
			t.Fatalf("Clone() produced an invalid graph: %v", err)
		}
		first, err := box.MarshalCanonical()
		if err != nil {
			t.Fatalf("MarshalCanonical() error = %v", err)
		}
		second, err := clone.MarshalCanonical()
		if err != nil {
			t.Fatalf("clone MarshalCanonical() error = %v", err)
		}
		if !bytes.Equal(first, second) {
			t.Fatal("tagged line clone changed canonical output")
		}

		malformed := []struct {
			name   string
			mutate func(*Box)
		}{
			{
				name: "missing line tag",
				mutate: func(box *Box) {
					box.Lines[0].Tag = nil
				},
			},
			{
				name: "nested line tag",
				mutate: func(box *Box) {
					lineTag := *box.Lines[0].Tag
					box.Lines[0].Box.Tag = &lineTag
				},
			},
			{
				name: "zero formula line",
				mutate: func(box *Box) {
					box.Tag.Line = 0
				},
			},
			{
				name: "non-manual tag",
				mutate: func(box *Box) {
					box.Tag.Manual = false
				},
			},
		}
		selected := malformed[len(data)%len(malformed)]
		bad := box.Clone()
		selected.mutate(bad)
		if err := bad.Validate(); err == nil {
			t.Fatalf("%s mutation passed Validate()", selected.name)
		}
	})
}

func fuzzValidTaggedLineGraph(data []byte) *Box {
	byteAt := func(index int) byte {
		if len(data) == 0 {
			return 0
		}
		return data[index%len(data)]
	}
	floatAt := func(index int) float32 {
		return float32(byteAt(index)%16) + 1
	}

	lineWidth := floatAt(0)
	tagWidth := floatAt(1)
	parentWidth := lineWidth + tagWidth + floatAt(2)
	tagX := parentWidth - tagWidth
	frameWidth := parentWidth - tagWidth
	offset := float32(0)
	switch byteAt(3) % 3 {
	case 1:
		offset = (frameWidth - lineWidth) / 2
	case 2:
		offset = frameWidth - lineWidth
	}

	lineItem := fuzzTagItem(0, lineWidth)
	tagItem := fuzzTagItem(tagX, tagWidth)
	lineHit := fuzzTagHit(0, lineWidth)
	tagHit := fuzzTagHit(tagX, tagWidth)
	line := &Box{
		Width:  lineWidth,
		Ascent: 1,
		Items:  []Item{lineItem},
		Hits:   []Hit{lineHit},
	}
	tag := &TagBox{
		Box: &Box{
			Width:  tagWidth,
			Ascent: 1,
			Items:  []Item{fuzzTagItem(0, tagWidth)},
			Hits:   []Hit{fuzzTagHit(0, tagWidth)},
		},
		Source: source.NewRange(0, 1),
		Manual: true,
		Line:   FormulaTagLine,
		X:      tagX,
	}
	translatedLine := lineItem
	translatedLine.Bound.X += offset
	translatedLine.Rect.X += offset
	translatedLine.At.X += offset
	translatedLineHit := lineHit
	translatedLineHit.Area.X += offset
	return &Box{
		Width:  parentWidth,
		Ascent: 1,
		Lines:  []LineBox{{OffsetX: offset, Box: line, Tag: tag}},
		Items:  []Item{translatedLine, tagItem},
		Hits:   []Hit{translatedLineHit, tagHit},
		Tag:    tag,
	}
}

func fuzzTagItem(x, width float32) Item {
	return Item{
		Kind:  ItemRule,
		Paint: Paint{Role: RoleInk},
		Bound: geometry.Rect{X: x, Y: -1, Width: width, Height: 1},
		Rect:  geometry.Rect{X: x, Y: -1, Width: width, Height: 1},
		At:    geometry.Point{X: x, Y: -1},
	}
}

func fuzzTagHit(x, width float32) Hit {
	return Hit{
		Source: source.NewRange(0, 1),
		Area:   geometry.Rect{X: x, Y: -1, Width: width, Height: 1},
		Kind:   HitAtom,
	}
}
