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
	kerns        *MathKernInfo
	TextLike     bool
	// StretchableHorizontal mirrors the base glyph's MATH construction.
	StretchableHorizontal bool

	firstDraw placedDraw
	moreDraws []placedDraw
	drawCount int
	// flatItems and flatHits are exact finalized subtree counts. A child must
	// be complete before it is placed with pushGroup, as required by the
	// immutable-after-construction fragment contract.
	flatItems int
	flatHits  int

	firstHit sourceHit
	moreHits []sourceHit
	hitCount int
	ownHit   int

	breaks []fragmentBreak
}

type fragmentBreak struct {
	x    geometry.Em
	kind display.BreakKind
	// cut is the number of source-ordered top-level placed groups before this
	// opportunity. It is independent of paint order and x coordinates.
	cut int
}

type sourceHit struct {
	source source.Range
	x      geometry.Em
	y      geometry.Em
	width  geometry.Em
	height geometry.Em
	depth  uint32
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

const (
	firstFragmentChunk = 4
	maxFragmentChunk   = 32
)

// fragmentArena owns the layout-created fragments for one layout pass. Chunks
// are never reused or moved, so returned pointers remain valid until the
// layoutState itself is collected.
type fragmentArena struct {
	chunks [][]Fragment
	used   int
}

const (
	firstGlyphChunk = 8
	maxGlyphChunk   = 32
)

type glyphArena struct {
	chunks [][]GlyphFragment
	used   int
}

func (a *glyphArena) reserve(count int) {
	if count <= 0 || len(a.chunks) != 0 {
		return
	}
	a.chunks = append(a.chunks, make([]GlyphFragment, count))
}

func (a *glyphArena) allocate() *GlyphFragment {
	last := len(a.chunks) - 1
	if last < 0 || a.used == len(a.chunks[last]) {
		size := firstGlyphChunk
		if last >= 0 {
			size = min(len(a.chunks[last])*2, maxGlyphChunk)
		}
		a.chunks = append(a.chunks, make([]GlyphFragment, size))
		last++
		a.used = 0
	}
	glyph := &a.chunks[last][a.used]
	a.used++
	return glyph
}

func (a *fragmentArena) allocate() *Fragment {
	last := len(a.chunks) - 1
	if last < 0 || a.used == len(a.chunks[last]) {
		size := firstFragmentChunk
		if last >= 0 {
			size = min(len(a.chunks[last])*2, maxFragmentChunk)
		}
		a.chunks = append(a.chunks, make([]Fragment, size))
		last++
		a.used = 0
	}
	fragment := &a.chunks[last][a.used]
	a.used++
	return fragment
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

func (s *layoutState) newFragment(metrics Metrics, class ir.MathClass, size MathSize, scale geometry.Em) (*Fragment, error) {
	if metrics.Width < 0 || metrics.Ascent < 0 || metrics.Descent < 0 {
		return nil, fmt.Errorf("%w: negative fragment metrics", ErrInvalidInput)
	}
	fragment := s.fragments.allocate()
	*fragment = Fragment{
		Metrics:      metrics,
		Class:        class,
		Size:         size,
		Scale:        scale,
		BaseAscent:   metrics.Ascent,
		BaseDescent:  metrics.Descent,
		AccentTop:    metrics.Width / 2,
		AccentBottom: metrics.Width / 2,
		ownHit:       -1,
	}
	return fragment, nil
}

func (s *layoutState) emptyFragment(size MathSize, scale geometry.Em) *Fragment {
	fragment := s.fragments.allocate()
	*fragment = Fragment{Size: size, Scale: scale, ownHit: -1}
	return fragment
}

func (s *layoutState) newGlyphFragment(glyph Glyph, scale geometry.Em, paint ir.Paint) *GlyphFragment {
	fragment := s.glyphs.allocate()
	*fragment = GlyphFragment{Glyph: glyph, Scale: scale, Paint: paint}
	return fragment
}

func (f *Fragment) push(x, y geometry.Em, kind drawKind, glyph *GlyphFragment, rule *ruleDraw, group *Fragment) error {
	if f == nil {
		return fmt.Errorf("%w: nil fragment", ErrInvalidInput)
	}
	f.appendDraw(placedDraw{
		x:     x,
		y:     y,
		kind:  kind,
		glyph: glyph,
		rule:  rule,
		group: group,
	})
	return nil
}

func (f *Fragment) appendDraw(draw placedDraw) {
	if f.drawCount == 0 {
		f.firstDraw = draw
	} else {
		f.moreDraws = append(f.moreDraws, draw)
	}
	f.drawCount++
	f.flatItems = saturatedAdd(f.flatItems, drawFlatItems(draw))
	if draw.kind == drawGroup && draw.group != nil {
		f.flatHits = saturatedAdd(f.flatHits, draw.group.flatHits)
	}
}

func (f *Fragment) drawAt(index int) placedDraw {
	if index == 0 {
		return f.firstDraw
	}
	return f.moreDraws[index-1]
}

// reserveDraws sets an exact tail capacity without allocating for the first
// draw, which is stored inline. Callers should use known construction counts.
func (f *Fragment) reserveDraws(count int) {
	if count <= 1 || cap(f.moreDraws) >= count-1 {
		return
	}
	draws := make([]placedDraw, len(f.moreDraws), count-1)
	copy(draws, f.moreDraws)
	f.moreDraws = draws
}

func drawFlatItems(draw placedDraw) int {
	switch draw.kind {
	case drawGroup:
		if draw.group != nil {
			return draw.group.flatItems
		}
	case drawGlyph:
		if draw.glyph != nil && len(draw.glyph.Glyph.Parts) > 0 {
			return len(draw.glyph.Glyph.Parts)
		}
	}
	return 1
}

func saturatedAdd(a, b int) int {
	maxInt := int(^uint(0) >> 1)
	if b > 0 && a > maxInt-b {
		return maxInt
	}
	return a + b
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
	f.appendDraw(placedDraw{x: x, y: y, kind: drawPath, path: path})
	return nil
}

func withClass(f *Fragment, class ir.MathClass) *Fragment {
	if f != nil && class != ir.ClassNormal {
		f.Class = class
	}
	return f
}

func (f *Fragment) kernAtHeight(corner KernCorner, height geometry.Em) (geometry.Em, error) {
	if f == nil {
		return 0, nil
	}
	if f.kerns == nil {
		return 0, nil
	}
	table := f.kerns.at(corner)
	if table == nil || len(table.Kerns) == 0 {
		return 0, nil
	}
	factor := f.Scale
	if factor != 0 && factor != geometry.Em(geometry.EmScale) {
		var err error
		height, err = unscale(height, factor)
		if err != nil {
			return 0, err
		}
	}
	index := 0
	for index < len(table.Heights) && height >= table.Heights[index] {
		index++
	}
	if index >= len(table.Kerns) {
		index = len(table.Kerns) - 1
	}
	if factor != 0 && factor != geometry.Em(geometry.EmScale) {
		return scale(table.Kerns[index], factor)
	}
	return table.Kerns[index], nil
}
