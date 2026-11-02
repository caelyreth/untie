package layout

import (
	"fmt"

	"github.com/caelyreth/untie/display"
	"github.com/caelyreth/untie/geometry"
	"github.com/caelyreth/untie/ir"
	"github.com/caelyreth/untie/source"
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
	Class        ir.MathClass
	Size         MathSize
	Scale        geometry.Em
	Italic       geometry.Em
	BaseAscent   geometry.Em
	BaseDescent  geometry.Em
	AccentTop    geometry.Em
	AccentBottom geometry.Em
	InkBottom    geometry.Em
	HasInkBottom bool
	Kerns        MathKernInfo
	TextLike     bool

	items  []placedDraw
	hits   []sourceHit
	ownHit int
}

type sourceHit struct {
	source source.Range
	x      geometry.Em
	y      geometry.Em
	width  geometry.Em
	height geometry.Em
	depth  int
	kind   display.HitKind
}

type drawKind uint8

const (
	drawGlyph drawKind = iota + 1
	drawRule
	drawGroup
	drawPath
)

type placedDraw struct {
	x geometry.Em
	y geometry.Em

	kind  drawKind
	glyph *GlyphFragment
	rule  *ruleDraw
	group *Fragment
	path  *pathDraw
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

type pathDraw struct {
	path  Path
	paint ir.Paint
}

func newFragment(metrics Metrics, class ir.MathClass, size MathSize, scale geometry.Em) (*Fragment, error) {
	if metrics.Width < 0 || metrics.Ascent < 0 || metrics.Descent < 0 {
		return nil, fmt.Errorf("%w: negative fragment metrics", ErrInvalidInput)
	}
	return &Fragment{
		Metrics:      metrics,
		Class:        class,
		Size:         size,
		Scale:        scale,
		BaseAscent:   metrics.Ascent,
		BaseDescent:  metrics.Descent,
		AccentTop:    metrics.Width / 2,
		AccentBottom: metrics.Width / 2,
		ownHit:       -1,
	}, nil
}

// NewFragment constructs an empty fragment for an extension port. Callers can
// add measured child groups, rules, or paths before returning it.
func NewFragment(metrics Metrics, class ir.MathClass, size MathSize, scale geometry.Em) (*Fragment, error) {
	return newFragment(metrics, class, size, scale)
}

func emptyFragment(size MathSize, scale geometry.Em) *Fragment {
	return &Fragment{Size: size, Scale: scale, ownHit: -1}
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

// AddGroup places child relative to the receiver's baseline origin.
func (f *Fragment) AddGroup(x, y geometry.Em, child *Fragment) error {
	return f.pushGroup(x, y, child)
}

// AddRule places a horizontal or vertical filled rule.
func (f *Fragment) AddRule(x, y, width, thickness geometry.Em, paint ir.Paint, vertical bool) error {
	if width < 0 || thickness < 0 {
		return fmt.Errorf("%w: negative rule dimension", ErrInvalidInput)
	}
	return f.pushRule(x, y, &ruleDraw{
		width:     width,
		thickness: thickness,
		paint:     paint,
		vertical:  vertical,
	})
}

// AddPath places a fillable path. Multiple contours use opposite winding when
// a hole is intended, so hosts can use either nonzero or even-odd filling.
func (f *Fragment) AddPath(x, y geometry.Em, path Path, paint ir.Paint) error {
	if len(path) == 0 {
		return fmt.Errorf("%w: empty path", ErrInvalidInput)
	}
	return f.pushPath(x, y, &pathDraw{
		path:  append(Path(nil), path...),
		paint: paint,
	})
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

func (f *Fragment) pushPath(x, y geometry.Em, path *pathDraw) error {
	if f == nil {
		return fmt.Errorf("%w: nil fragment", ErrInvalidInput)
	}
	if path == nil || len(path.path) == 0 {
		return fmt.Errorf("%w: empty path drawable", ErrInvalidInput)
	}
	f.items = append(f.items, placedDraw{x: x, y: y, kind: drawPath, path: path})
	return nil
}

func (f *Fragment) includePathBounds(path Path) {
	if f == nil || len(path) == 0 {
		return
	}
	minY, maxY := geometry.Em(0), geometry.Em(0)
	have := false
	for _, segment := range path {
		for _, point := range segment.Points {
			if !have {
				minY, maxY = point.Y, point.Y
				have = true
				continue
			}
			minY = min(minY, point.Y)
			maxY = max(maxY, point.Y)
		}
	}
	if !have {
		return
	}
	f.Ascent = max(f.Ascent, -minY)
	f.Descent = max(f.Descent, maxY)
}

func withClass(f *Fragment, class ir.MathClass) *Fragment {
	if f != nil && class != ir.ClassNormal {
		f.Class = class
	}
	return f
}

func (f *Fragment) kernAtHeight(corner KernCorner, height geometry.Em) geometry.Em {
	if f == nil {
		return 0
	}
	table := f.Kerns.at(corner)
	if table == nil || len(table.Kerns) == 0 {
		return 0
	}
	index := 0
	for index < len(table.Heights) && height >= table.Heights[index] {
		index++
	}
	if index >= len(table.Kerns) {
		index = len(table.Kerns) - 1
	}
	return table.Kerns[index]
}
