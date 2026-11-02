// Package display defines the immutable output contract for UI hosts.
package display

import "github.com/caelyreth/untie/geometry"

// Align is horizontal display alignment.
type Align uint8

const (
	AlignStart Align = iota
	AlignCenter
	AlignEnd
)

// Style is a layout request in host units.
type Style struct {
	Size        float32
	MaxWidth    float32
	Align       Align
	ScriptLevel int
	Features    []string
	GlyphRefs   bool
}

// Role is a theme-resolved paint role.
type Role uint8

const (
	RoleInk Role = iota
	RoleError
	RolePlaceholder
)

// RGBA is an explicit output color.
type RGBA struct {
	R uint8
	G uint8
	B uint8
	A uint8
}

// Paint is a role or explicit color.
type Paint struct {
	Role        Role
	Explicit    RGBA
	HasExplicit bool
}

// ItemKind identifies one flattened drawing item.
type ItemKind uint8

const (
	ItemGlyphPath ItemKind = iota
	ItemRule
	ItemGlyphRef
)

// Item is one already-positioned drawing instruction.
type Item struct {
	Kind  ItemKind
	Paint Paint
	Bound geometry.Rect

	Path []geometry.Segment
	Rect geometry.Rect

	FontID string
	Glyph  uint16
	At     geometry.Point
	Em     float32
}

// Box is a flattened formula relative to the start of its baseline.
type Box struct {
	Width      float32
	Ascent     float32
	Descent    float32
	Italic     float32
	AxisHeight float32
	Overflow   float32

	Items []Item
}

// Clone returns an independent copy of the box.
func (b *Box) Clone() *Box {
	if b == nil {
		return nil
	}
	out := *b
	out.Items = make([]Item, len(b.Items))
	for i, item := range b.Items {
		out.Items[i] = item
		if item.Path != nil {
			out.Items[i].Path = append([]geometry.Segment(nil), item.Path...)
		}
	}
	return &out
}
