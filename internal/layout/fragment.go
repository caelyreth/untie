package layout

import (
	"fmt"

	"github.com/caelyreth/untie/geometry"
	"github.com/caelyreth/untie/ir"
)

// Metrics are baseline-relative dimensions in root ems.
type Metrics struct {
	Width   geometry.Em
	Ascent  geometry.Em
	Descent geometry.Em
}

// Height returns ascent+descent, reporting overflow.
func (m Metrics) Height() (geometry.Em, error) {
	return add(m.Ascent, m.Descent)
}

// Fragment is the internal, immutable-after-construction layout tree. Items
// are stored in deterministic back-to-front order and positioned relative to
// the fragment's baseline origin.
type Fragment struct {
	Metrics
	Class       ir.MathClass
	Size        MathSize
	Scale       geometry.Em
	Italic      geometry.Em
	BaseAscent  geometry.Em
	BaseDescent geometry.Em
	TextLike    bool

	items []placedDraw
}

type drawKind uint8

const (
	drawGlyph drawKind = iota + 1
	drawRule
	drawGroup
)

type placedDraw struct {
	x geometry.Em
	y geometry.Em

	kind  drawKind
	glyph *GlyphFragment
	rule  *ruleDraw
	group *Fragment
}

// GlyphFragment is one placed glyph in root ems. It is the internal bridge
// between a GlyphSource result and the flattening stage.
type GlyphFragment struct {
	Glyph Glyph
	Scale geometry.Em
	Paint ir.Paint
}

type ruleDraw struct {
	width     geometry.Em
	thickness geometry.Em
	paint     ir.Paint
	vertical  bool
}

func newFragment(metrics Metrics, class ir.MathClass, size MathSize, scale geometry.Em) (*Fragment, error) {
	if metrics.Width < 0 || metrics.Ascent < 0 || metrics.Descent < 0 {
		return nil, fmt.Errorf("%w: negative fragment metrics", ErrInvalidInput)
	}
	return &Fragment{
		Metrics:     metrics,
		Class:       class,
		Size:        size,
		Scale:       scale,
		BaseAscent:  metrics.Ascent,
		BaseDescent: metrics.Descent,
	}, nil
}

func emptyFragment(size MathSize, scale geometry.Em) *Fragment {
	return &Fragment{Size: size, Scale: scale}
}

func (f *Fragment) push(x, y geometry.Em, kind drawKind, glyph *GlyphFragment, rule *ruleDraw, group *Fragment) error {
	if f == nil {
		return fmt.Errorf("%w: nil fragment", ErrInvalidInput)
	}
	f.items = append(f.items, placedDraw{
		x:     x,
		y:     y,
		kind:  kind,
		glyph: glyph,
		rule:  rule,
		group: group,
	})
	return nil
}

func (f *Fragment) pushGlyph(x, y geometry.Em, glyph *GlyphFragment) error {
	if glyph == nil {
		return fmt.Errorf("%w: nil glyph drawable", ErrInvalidInput)
	}
	return f.push(x, y, drawGlyph, glyph, nil, nil)
}

func (f *Fragment) pushRule(x, y geometry.Em, rule *ruleDraw) error {
	if rule == nil {
		return fmt.Errorf("%w: nil rule drawable", ErrInvalidInput)
	}
	return f.push(x, y, drawRule, nil, rule, nil)
}

func (f *Fragment) pushGroup(x, y geometry.Em, group *Fragment) error {
	if group == nil {
		return fmt.Errorf("%w: nil group", ErrInvalidInput)
	}
	return f.push(x, y, drawGroup, nil, nil, group)
}

func withClass(f *Fragment, class ir.MathClass) *Fragment {
	if f != nil && class != ir.ClassNormal {
		f.Class = class
	}
	return f
}
