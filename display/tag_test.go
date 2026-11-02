package display

import (
	"strings"
	"testing"

	"github.com/caelyreth/untie/geometry"
	"github.com/caelyreth/untie/source"
)

func TestTagBoxCloneCanonicalAndValidation(t *testing.T) {
	t.Parallel()

	tag := validDisplayTagBox(2, 0)
	parent := &Box{
		Width:    3,
		Ascent:   1,
		Items:    translatedBoxItems(tag),
		Hits:     translatedTagHits(tag),
		Tag:      tag,
		Overflow: 0,
	}
	if err := parent.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
	canonical, err := parent.MarshalCanonical()
	if err != nil {
		t.Fatalf("MarshalCanonical() error = %v", err)
	}
	if !strings.Contains(string(canonical), `"tag":{"box":`) ||
		!strings.Contains(string(canonical), `"source":[1,2]`) {
		t.Fatalf("canonical output omitted tag: %s", canonical)
	}
	clone := parent.Clone()
	clone.Tag.Box.Items[0].Bound.X++
	clone.Tag.Box.Hits[0].Area.X++
	if parent.Tag.Box.Items[0].Bound.X == clone.Tag.Box.Items[0].Bound.X {
		t.Fatal("Clone shared tag item storage")
	}
	if parent.Tag.Box.Hits[0].Area.X == clone.Tag.Box.Hits[0].Area.X {
		t.Fatal("Clone shared tag hit storage")
	}
}

func TestTagBoxValidationRejectsMalformedContracts(t *testing.T) {
	t.Parallel()

	base := validDisplayTagBox(2, 0)
	tests := []struct {
		name   string
		mutate func(*Box)
	}{
		{
			name: "missing aggregate item",
			mutate: func(box *Box) {
				box.Items = box.Items[:len(box.Items)-1]
			},
		},
		{
			name: "wrong aggregate translation",
			mutate: func(box *Box) {
				box.Items[len(box.Items)-1].Bound.X++
			},
		},
		{
			name: "nested tag",
			mutate: func(box *Box) {
				inner := validDisplayTagBox(0, 0)
				box.Tag.Box.Tag = inner
			},
		},
		{
			name: "invalid source",
			mutate: func(box *Box) {
				box.Tag.Source = source.Detached()
			},
		},
		{
			name: "negative x",
			mutate: func(box *Box) {
				box.Tag.X = -1
			},
		},
		{
			name: "starred non-manual",
			mutate: func(box *Box) {
				box.Tag.Manual = false
				box.Tag.Starred = true
			},
		},
		{
			name: "italic does not match tag",
			mutate: func(box *Box) {
				box.Italic = 1
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			tag := cloneTestTag(base)
			box := &Box{
				Width:  3,
				Ascent: 1,
				Items:  translatedBoxItems(tag),
				Hits:   translatedTagHits(tag),
				Tag:    tag,
			}
			tc.mutate(box)
			if err := box.Validate(); err == nil {
				t.Fatal("Validate() error = nil")
			}
		})
	}
}

func TestTagLogicalWidthRequiresParentOverflow(t *testing.T) {
	t.Parallel()

	tag := validDisplayTagBox(2, 0)
	parent := &Box{
		Width:  3,
		Ascent: 1,
		Items:  translatedBoxItems(tag),
		Hits:   translatedTagHits(tag),
		Tag:    tag,
	}
	if err := parent.Validate(); err != nil {
		t.Fatalf("base Validate() error = %v", err)
	}
	parent.Tag.Box.Width = 5
	if err := parent.Validate(); err == nil {
		t.Fatal("Validate() ignored logical tag-width overflow")
	}
	parent.Overflow = 4
	if err := parent.Validate(); err != nil {
		t.Fatalf("Validate() with logical overflow error = %v", err)
	}
}

func TestConstrainedTagRequiresTargetedLineTag(t *testing.T) {
	t.Parallel()

	tag := validDisplayTagBox(9, 0)
	line := &Box{
		Width:  1,
		Ascent: 1,
		Items: []Item{{
			Kind:  ItemRule,
			Paint: Paint{Role: RoleInk},
			Bound: geometry.Rect{Y: -1, Width: 1, Height: 0.1},
			Rect:  geometry.Rect{Y: -1, Width: 1, Height: 0.1},
		}},
	}
	parent := &Box{
		Width:  10,
		Ascent: 1,
		Lines:  []LineBox{{Box: line, Tag: tag}},
		Items:  append(cloneItems(line.Items), translatedBoxItems(tag)...),
		Hits:   translatedTagHits(tag),
		Tag:    tag,
	}
	if err := parent.Validate(); err != nil {
		t.Fatalf("base Validate() error = %v", err)
	}

	withoutLineTag := parent.Clone()
	withoutLineTag.Lines[0].Tag = nil
	if err := withoutLineTag.Validate(); err == nil {
		t.Fatal("Validate() accepted a constrained parent tag without its targeted line tag")
	}

	wrongLine := parent.Clone()
	wrongLine.Tag.Line = 1
	wrongLine.Lines[0].Tag.Line = 1
	if err := wrongLine.Validate(); err == nil {
		t.Fatal("Validate() accepted an out-of-range tag line association")
	}
}

func TestTaggedLineValidationAndCanonical(t *testing.T) {
	t.Parallel()

	tag := validDisplayTagBox(9, 0)
	line := &Box{
		Width:  1,
		Ascent: 1,
		Items: []Item{{
			Kind:  ItemRule,
			Paint: Paint{Role: RoleInk},
			Bound: geometry.Rect{Y: -1, Width: 1, Height: 0.1},
			Rect:  geometry.Rect{Y: -1, Width: 1, Height: 0.1},
		}},
	}
	parent := &Box{
		Width:  10,
		Ascent: 1,
		Lines: []LineBox{{
			Baseline: 0,
			OffsetX:  0,
			Box:      line,
			Tag:      tag,
		}},
		Items: append(cloneItems(line.Items), translatedBoxItems(tag)...),
		Hits:  translatedTagHits(tag),
		Tag:   tag,
	}
	if err := parent.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
	canonical, err := parent.MarshalCanonical()
	if err != nil {
		t.Fatalf("MarshalCanonical() error = %v", err)
	}
	if !strings.Contains(string(canonical), `"lines":[{"baseline":0,"offsetX":0,"box":`) ||
		!strings.Contains(string(canonical), `"tag":{"box":`) {
		t.Fatalf("canonical output omitted line tag: %s", canonical)
	}

	parent.Lines[0].Tag.X++
	if err := parent.Validate(); err == nil {
		t.Fatal("Validate() accepted mismatched parent and line tag metadata")
	}
}

func validDisplayTagBox(x, y float32) *TagBox {
	return &TagBox{
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
				Source: source.NewRange(1, 2),
				Area:   geometry.Rect{Width: 1, Height: 1},
				Kind:   HitAtom,
			}},
		},
		Source: source.NewRange(1, 2),
		Manual: true,
		Line:   0,
		X:      x,
		Y:      y,
	}
}

func cloneTestTag(tag *TagBox) *TagBox {
	out := *tag
	out.Box = tag.Box.Clone()
	return &out
}

func translatedBoxItems(tag *TagBox) []Item {
	out := cloneItems(tag.Box.Items)
	for i := range out {
		out[i].Bound.X += tag.X
		out[i].Bound.Y += tag.Y
		out[i].Rect.X += tag.X
		out[i].Rect.Y += tag.Y
		out[i].At.X += tag.X
		out[i].At.Y += tag.Y
	}
	return out
}

func translatedTagHits(tag *TagBox) []Hit {
	out := append([]Hit(nil), tag.Box.Hits...)
	for i := range out {
		out[i].Area.X += tag.X
		out[i].Area.Y += tag.Y
	}
	return out
}
