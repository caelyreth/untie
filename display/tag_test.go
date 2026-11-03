package display

import (
	"strings"
	"testing"

	"github.com/caelyreth/untie/geometry"
	"github.com/caelyreth/untie/source"
)

func TestTagBoxCloneCanonicalAndValidation(t *testing.T) {
	t.Parallel()

	box := taggedRangeBox(2)
	if err := box.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
	canonical, err := box.MarshalCanonical()
	if err != nil {
		t.Fatalf("MarshalCanonical() error = %v", err)
	}
	if !strings.Contains(string(canonical), `"tag":{"source":[1,2]`) ||
		!strings.Contains(string(canonical), `"items":{"start":1,"count":1}`) {
		t.Fatalf("canonical output omitted tag metadata or range: %s", canonical)
	}
	if strings.Contains(string(canonical), `"tag":{"box":`) {
		t.Fatalf("canonical output retained nested tag box: %s", canonical)
	}
	clone := box.Clone()
	clone.Items[clone.Tag.Items.Start].Bound.X++
	clone.Tag.Width++
	if box.Items[box.Tag.Items.Start].Bound.X == clone.Items[clone.Tag.Items.Start].Bound.X {
		t.Fatal("Clone shared item storage")
	}
	if box.Tag.Width == clone.Tag.Width {
		t.Fatal("Clone shared tag metadata")
	}

	constrained := constrainedTaggedRangeBox()
	constrainedClone := constrained.Clone()
	if constrainedClone.Lines[0].Tag != constrainedClone.Tag {
		t.Fatal("Clone did not preserve the borrowed line-tag identity")
	}
}

func TestTagBoxValidationRejectsMalformedRangesAndMetadata(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		mutate func(*Box)
	}{
		{name: "invalid source", mutate: func(box *Box) { box.Tag.Source = source.Detached() }},
		{name: "negative x", mutate: func(box *Box) { box.Tag.X = -1 }},
		{name: "negative width", mutate: func(box *Box) { box.Tag.Width = -1 }},
		{name: "non-manual", mutate: func(box *Box) { box.Tag.Manual = false }},
		{name: "future line", mutate: func(box *Box) { box.Tag.Line = 5 }},
		{name: "italic mismatch", mutate: func(box *Box) { box.Italic = 1 }},
		{name: "item overflow", mutate: func(box *Box) { box.Tag.Items.Count = 2 }},
		{name: "hit overflow", mutate: func(box *Box) { box.Tag.Hits.Count = 2 }},
		{name: "not suffix", mutate: func(box *Box) {
			box.Items = append(box.Items, box.Items[len(box.Items)-1])
			box.Hits = append(box.Hits, box.Hits[len(box.Hits)-1])
		}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			box := taggedRangeBox(2)
			tc.mutate(box)
			if err := box.Validate(); err == nil {
				t.Fatal("Validate() error = nil")
			}
		})
	}
}

func TestTagLogicalWidthRequiresParentOverflow(t *testing.T) {
	t.Parallel()

	box := taggedRangeBox(2)
	box.Tag.Width = 5
	if err := box.Validate(); err == nil {
		t.Fatal("Validate() ignored logical tag-width overflow")
	}
	box.Overflow = 4
	if err := box.Validate(); err != nil {
		t.Fatalf("Validate() with logical overflow error = %v", err)
	}
}

func TestConstrainedTagRequiresTargetedLineTag(t *testing.T) {
	t.Parallel()

	box := constrainedTaggedRangeBox()
	if err := box.Validate(); err != nil {
		t.Fatalf("base Validate() error = %v", err)
	}

	withoutLineTag := box.Clone()
	withoutLineTag.Lines[0].Tag = nil
	if err := withoutLineTag.Validate(); err == nil {
		t.Fatal("Validate() accepted a constrained parent tag without its targeted line tag")
	}

	withoutParentTag := box.Clone()
	withoutParentTag.Tag = nil
	if err := withoutParentTag.Validate(); err == nil {
		t.Fatal("Validate() accepted a line tag without a parent tag")
	}

	mismatched := box.Clone()
	borrowed := *mismatched.Lines[0].Tag
	borrowed.X++
	mismatched.Lines[0].Tag = &borrowed
	if err := mismatched.Validate(); err == nil {
		t.Fatal("Validate() accepted a line tag that was not the canonical parent tag")
	}

	badLineRange := box.Clone()
	badLineRange.Lines[0].Items.Count = 2
	if err := badLineRange.Validate(); err == nil {
		t.Fatal("Validate() accepted a line range that overlaps the tag")
	}
}

func TestTaggedLineValidationAndCanonical(t *testing.T) {
	t.Parallel()

	box := constrainedTaggedRangeBox()
	canonical, err := box.MarshalCanonical()
	if err != nil {
		t.Fatalf("MarshalCanonical() error = %v", err)
	}
	if !strings.Contains(string(canonical), `"lines":[{"baseline":0`) ||
		!strings.Contains(string(canonical), `"tag":{`) {
		t.Fatalf("canonical output omitted line tag: %s", canonical)
	}
}

func taggedRangeBox(tagX float32) *Box {
	formulaItem := ruleItemAt(0, 1)
	tagItem := ruleItemAt(tagX, 1)
	formulaHit := ruleHitAt(0, 1)
	tagHit := ruleHitAt(tagX, 1)
	tag := &TagBox{
		Source:  source.NewRange(1, 2),
		Manual:  true,
		Line:    FormulaTagLine,
		X:       tagX,
		Width:   1,
		Ascent:  1,
		Italic:  -0.25,
		Items:   ItemRange{Start: 1, Count: 1},
		Hits:    HitRange{Start: 1, Count: 1},
		Starred: false,
	}
	return &Box{
		Width:  tagX + 1,
		Ascent: 1,
		Italic: -0.25,
		Items:  []Item{formulaItem, tagItem},
		Hits:   []Hit{formulaHit, tagHit},
		Tag:    tag,
	}
}

func constrainedTaggedRangeBox() *Box {
	box := taggedRangeBox(9)
	box.Width = 10
	box.Lines = []LineBox{{
		Width:  1,
		Ascent: 1,
		Items:  ItemRange{Count: 1},
		Hits:   HitRange{Count: 1},
		Tag:    box.Tag,
	}}
	return box
}

func ruleItemAt(x, width float32) Item {
	return Item{
		Kind:      ItemRule,
		Paint:     Paint{Role: RoleInk},
		Bound:     geometry.Rect{X: x, Y: -1, Width: width, Height: 1},
		PathIndex: NoPath,
		Rect:      geometry.Rect{X: x, Y: -1, Width: width, Height: 1},
	}
}

func ruleHitAt(x, width float32) Hit {
	return Hit{
		Source: source.NewRange(1, 2),
		Area:   geometry.Rect{X: x, Y: -1, Width: width, Height: 1},
		Kind:   HitAtom,
	}
}
