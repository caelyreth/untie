// Package display defines the immutable output contract for UI hosts.
package display

import (
	"slices"

	"github.com/caelyreth/untie/geometry"
	"github.com/caelyreth/untie/source"
)

// Align is horizontal placement within a constrained display line frame.
// Inline layout and display layout with MaxWidth zero ignore it.
type Align uint8

const (
	AlignStart Align = iota
	AlignCenter
	AlignEnd
)

// FormulaTagLine identifies a tag that belongs to the complete formula rather
// than one rendered display line. It is the only Phase-1 TagBox line value.
const FormulaTagLine = -1

// Style is a layout request in host units.
type Style struct {
	Size float32
	// MaxWidth enables constrained display line boxes when positive. A
	// constrained Box.Width is the requested frame width.
	MaxWidth    float32
	Align       Align
	ScriptLevel int
	Features    []string
	GlyphRefs   bool
	// GenerateAlt requests Box.Alt generation through the public Engine
	// facade. Expr.Alt remains available independently for parsed expressions.
	GenerateAlt bool
}

// ItemRange identifies a contiguous range in Box.Items.
//
// Start is the zero-based index of the first item and Count is the number of
// items. An empty range is valid. Ranges returned by layout always refer to
// parent-frame storage and never require a coordinate translation.
type ItemRange struct {
	Start int
	Count int
}

// End returns Start+Count.
func (r ItemRange) End() int {
	return r.Start + r.Count
}

// HitRange identifies a contiguous range in Box.Hits.
//
// Start is the zero-based index of the first hit and Count is the number of
// hits. An empty range is valid. Ranges returned by layout always refer to
// parent-frame storage.
type HitRange struct {
	Start int
	Count int
}

// End returns Start+Count.
func (r HitRange) End() int {
	return r.Start + r.Count
}

// LineBox is metadata for one display line inside a constrained Box.
//
// Baseline is the line baseline's vertical offset from the parent box baseline,
// positive downward. Adjacent baselines are separated by the previous line's
// descent plus the current line's ascent; no additional leading is invented.
// OffsetX is the nonnegative horizontal offset of the line inside the parent's
// MaxWidth-wide alignment frame. Items and Hits are contiguous ranges into the
// owning Box's parent-frame slices. Overflow is max(line-local ink overflow,
// line width minus the parent frame width); the parent Box.Overflow covers the
// assembled aligned aggregate. When Tag is non-nil, line alignment is computed
// in the reduced frame MaxWidth-Tag.Width; an overfull formula line uses zero
// offset and the tag follows the formula.
type LineBox struct {
	Baseline float32
	OffsetX  float32
	Width    float32
	Ascent   float32
	Descent  float32
	Overflow float32
	Items    ItemRange
	Hits     HitRange
	// Tag is parent-frame metadata for a tag associated with this line. Its
	// range is separate from the line range. Nil means no rendered tag.
	Tag *TagBox
}

// TagBox is equation-tag metadata. Its Items and Hits ranges index the owning
// Box's parent-frame slices. X and Y are parent-frame coordinates and do not
// move with LineBox.OffsetX. Line is FormulaTagLine for the formula-level
// Phase-1 contract.
type TagBox struct {
	Source  source.Range
	Manual  bool
	Starred bool
	// Line is FormulaTagLine. Zero-based row tags are reserved for future
	// multiline equation-tag support and are not emitted by Phase 1.
	Line    int
	X       float32
	Y       float32
	Width   float32
	Ascent  float32
	Descent float32
	Italic  float32
	Items   ItemRange
	Hits    HitRange
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
	Width   float32
	Ascent  float32
	Descent float32
	// Italic is the signed OpenType MATH italic correction at the end of the
	// box. It may be negative; hosts must not treat it as a nonnegative size.
	Italic     float32
	AxisHeight float32
	// Overflow is a nonnegative lower bound for rendered ink outside the box and,
	// for constrained display boxes, the largest logical line-width excess.
	Overflow float32

	Items  []Item
	Hits   []Hit
	Breaks []Break
	Alt    Alt
	// Tag is a single-line display equation tag in parent coordinates. Its
	// Items and Hits ranges identify its records in this Box. It is excluded
	// from formula-root breaks.
	Tag *TagBox
	// Lines is populated only for display layout with a positive Style.MaxWidth.
	// A constrained display result always has at least one line, including when
	// the complete formula fits. Its Width is the requested MaxWidth, even when
	// every line is narrower or an unbreakable line overflows it. Lines are
	// source ordered and index the single parent-frame Items and Hits storage.
	Lines []LineBox
}

// Alt is semantic alternative text derived from the neutral expression tree.
type Alt struct {
	Speech string
	Text   string
}

// Clone returns an independent copy of the alternative text.
func (a Alt) Clone() Alt {
	return a
}

// Clone returns an independent copy of the box.
func (b *Box) Clone() *Box {
	if b == nil {
		return nil
	}
	return cloneBox(b, true)
}

func cloneBox(b *Box, cloneNested bool) *Box {
	out := *b
	out.Items = cloneItems(b.Items)
	out.Hits = append([]Hit(nil), b.Hits...)
	out.Breaks = append([]Break(nil), b.Breaks...)
	if b.Tag != nil {
		tag := *b.Tag
		out.Tag = &tag
	}
	if !cloneNested || len(b.Lines) == 0 {
		out.Lines = nil
		return &out
	}
	out.Lines = make([]LineBox, len(b.Lines))
	for i, line := range b.Lines {
		out.Lines[i] = line
		if line.Tag != nil {
			tag := *line.Tag
			out.Lines[i].Tag = &tag
		}
	}
	return &out
}

func cloneItems(items []Item) []Item {
	if len(items) == 0 {
		return nil
	}
	out := make([]Item, len(items))
	for i, item := range items {
		out[i] = item
		out[i].Path = append([]geometry.Segment(nil), item.Path...)
	}
	return out
}

// HitKind identifies the source semantic represented by a hit area.
type HitKind uint8

const (
	HitAtom HitKind = iota
	HitGroup
	HitScript
	HitFractionNumerator
	HitFractionDenominator
	HitRadical
	HitFence
	HitTableCell
	HitRow
	HitOther
)

// Hit maps a source range to one placed area.
type Hit struct {
	Source source.Range
	Area   geometry.Rect
	Depth  int
	Kind   HitKind
}

// BreakKind describes why an inline break point is legal.
type BreakKind uint8

const (
	BreakRelation BreakKind = iota
	BreakBinary
	BreakExplicit
)

// Break is an inline wrapping opportunity relative to the box baseline origin.
// Relation, binary, and explicit kinds describe why a cut is legal; they do not
// carry a relative cost. Display wrapping is width-greedy and chooses the
// earliest source cut when multiple opportunities have equal width.
type Break struct {
	X    float32
	Kind BreakKind
}

// At returns the innermost hit containing p.
func (b *Box) At(p geometry.Point) (Hit, bool) {
	if b == nil {
		return Hit{}, false
	}
	var best Hit
	found := false
	for _, hit := range b.Hits {
		if !contains(hit.Area, p) {
			continue
		}
		if !found || betterHit(hit, best) {
			best = hit
			found = true
		}
	}
	return best, found
}

// Covering returns the placed areas intersecting source range r. Areas on the
// same line that overlap or touch are merged deterministically.
func (b *Box) Covering(r source.Range) []geometry.Rect {
	if b == nil || r.IsDetached() || !r.IsValid() {
		return nil
	}
	areas := make([]geometry.Rect, 0)
	for _, hit := range b.Hits {
		if hit.Source.IsDetached() || !hit.Source.IsValid() {
			continue
		}
		if hit.Source.End <= r.Start || hit.Source.Start >= r.End {
			continue
		}
		areas = append(areas, hit.Area)
	}
	slices.SortFunc(areas, func(a, b geometry.Rect) int {
		if a.Y != b.Y {
			if a.Y < b.Y {
				return -1
			}
			return 1
		}
		if a.X != b.X {
			if a.X < b.X {
				return -1
			}
			return 1
		}
		if a.Width != b.Width {
			if a.Width < b.Width {
				return -1
			}
			return 1
		}
		if a.Height < b.Height {
			return -1
		}
		if a.Height > b.Height {
			return 1
		}
		return 0
	})
	out := areas[:0]
	for _, area := range areas {
		if len(out) == 0 {
			out = append(out, area)
			continue
		}
		last := &out[len(out)-1]
		if last.Y == area.Y && last.Height == area.Height &&
			(area.X <= last.X+last.Width || absFloat(area.X-last.X-last.Width) < 1e-5) {
			right := maxFloat(last.X+last.Width, area.X+area.Width)
			last.Width = right - last.X
			continue
		}
		out = append(out, area)
	}
	return out
}

func contains(r geometry.Rect, p geometry.Point) bool {
	return p.X >= r.X && p.X <= r.X+r.Width && p.Y >= r.Y && p.Y <= r.Y+r.Height
}

func betterHit(candidate, current Hit) bool {
	if candidate.Depth != current.Depth {
		return candidate.Depth > current.Depth
	}
	candidateArea := candidate.Area.Width * candidate.Area.Height
	currentArea := current.Area.Width * current.Area.Height
	if candidateArea != currentArea {
		return candidateArea < currentArea
	}
	if candidate.Source.Start != current.Source.Start {
		return candidate.Source.Start < current.Source.Start
	}
	return candidate.Source.End < current.Source.End
}

func absFloat(v float32) float32 {
	if v < 0 {
		return -v
	}
	return v
}

func minFloat(a, b float32) float32 {
	if a < b {
		return a
	}
	return b
}

func maxFloat(a, b float32) float32 {
	if a > b {
		return a
	}
	return b
}
