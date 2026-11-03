package layout

import (
	"fmt"
	"math"

	"github.com/caelyreth/untie/display"
	"github.com/caelyreth/untie/geometry"
	"github.com/caelyreth/untie/ir"
)

func (s *layoutState) flatten(root *Fragment) (*display.Box, error) {
	if s.mode == ir.Display && s.request.MaxWidth > 0 {
		return s.flattenDisplayLines(root)
	}
	return s.flattenFragment(root)
}

func (s *layoutState) flattenFragment(root *Fragment) (*display.Box, error) {
	if root == nil {
		return nil, fmt.Errorf("%w: nil root fragment", ErrInvalidInput)
	}
	rootAscent, rootDescent := root.Ascent, root.Descent
	if s.mode == ir.Inline {
		// Typst lays inline equations into paragraph line boxes with the font's
		// cap height as a floor and subtracts the line-leading slack from ink
		// extents. Item positions remain baseline-relative.
		capHeight, err := s.metricAt(s.metrics.CapHeight, s.rootStyle)
		if err != nil {
			return nil, err
		}
		const inlineSlackPercent = int64(455) // 0.65 leading * 0.7
		slack, err := mulDiv(geometry.Em(geometry.EmScale), inlineSlackPercent, 1000)
		if err != nil {
			return nil, err
		}
		slack, err = s.metricAt(slack, s.rootStyle)
		if err != nil {
			return nil, err
		}
		rootAscent = max(capHeight, root.Ascent-slack)
		rootDescent = max(0, root.Descent-slack)
	}
	width, err := s.host(root.Width)
	if err != nil {
		return nil, err
	}
	ascent, err := s.host(rootAscent)
	if err != nil {
		return nil, err
	}
	descent, err := s.host(rootDescent)
	if err != nil {
		return nil, err
	}
	italic, err := s.host(root.Italic)
	if err != nil {
		return nil, err
	}
	axis, err := s.metricAt(s.metrics.AxisHeight, s.rootStyle)
	if err != nil {
		return nil, err
	}
	axisHeight, err := s.host(axis)
	if err != nil {
		return nil, err
	}

	box := &display.Box{
		Width:      width,
		Ascent:     ascent,
		Descent:    descent,
		Italic:     italic,
		AxisHeight: axisHeight,
		Items:      make([]display.Item, 0, s.outputItemCapacity(root.flatItems)),
		Paths:      make([]display.PathTemplate, 0, s.outputItemCapacity(root.flatItems)),
		Hits:       make([]display.Hit, 0, s.outputHitCapacity(root.flatHits)),
	}
	state := newFlattenState(s, box, 0, 0)
	if err := state.visit(root, 0, 0); err != nil {
		return nil, err
	}
	if s.mode == ir.Inline {
		state.box.Breaks, err = s.displayBreaks(s.ctx, root.breaks)
		if err != nil {
			return nil, err
		}
	}
	state.box.Overflow = state.overflow()
	return state.box, nil
}

func (s *layoutState) outputItemCapacity(want int) int {
	return boundedCapacity(want, s.limits.MaxOutputItems)
}

func (s *layoutState) outputHitCapacity(want int) int {
	return boundedCapacity(want, s.limits.MaxNodes)
}

func boundedCapacity(want, limit int) int {
	switch {
	case want < 0:
		return 0
	case want > limit:
		return limit
	default:
		return want
	}
}

func reserveDisplayItems(items []display.Item, additional, limit int) []display.Item {
	want := saturatedAdd(len(items), additional)
	capacity := boundedCapacity(want, limit)
	if capacity <= cap(items) {
		return items
	}
	grown := make([]display.Item, len(items), capacity)
	copy(grown, items)
	return grown
}

func reserveDisplayHits(hits []display.Hit, additional, limit int) []display.Hit {
	want := saturatedAdd(len(hits), additional)
	capacity := boundedCapacity(want, limit)
	if capacity <= cap(hits) {
		return hits
	}
	grown := make([]display.Hit, len(hits), capacity)
	copy(grown, hits)
	return grown
}

func reserveDisplayPaths(paths []display.PathTemplate, additional, limit int) []display.PathTemplate {
	want := saturatedAdd(len(paths), additional)
	capacity := boundedCapacity(want, limit)
	if capacity <= cap(paths) {
		return paths
	}
	grown := make([]display.PathTemplate, len(paths), capacity)
	copy(grown, paths)
	return grown
}

type flattenState struct {
	layout         *layoutState
	box            *display.Box
	stack          []*Fragment
	inlinePathKeys [inlinePathCacheSize]pathTemplateKey
	inlinePathIDs  [inlinePathCacheSize]int
	inlinePaths    int
	pathCache      map[pathTemplateKey]int
	offsetX        float32
	offsetY        float32
	visits         int
	operations     int
}

type pathTemplateKey struct {
	first   *Segment
	length  int
	fontID  string
	glyphID uint16
	exact   bool
}

const inlinePathCacheSize = 8

func newFlattenState(layout *layoutState, box *display.Box, offsetX, offsetY float32) flattenState {
	return flattenState{
		layout:  layout,
		box:     box,
		offsetX: offsetX,
		offsetY: offsetY,
	}
}

func (f *flattenState) appendItem(item display.Item) error {
	if f.offsetX != 0 || f.offsetY != 0 {
		var err error
		item, err = f.layout.offsetItem(item, f.offsetX, f.offsetY)
		if err != nil {
			return err
		}
	}
	f.box.Items = append(f.box.Items, item)
	if len(f.box.Items) > f.layout.limits.MaxOutputItems {
		return fmt.Errorf("%w: output item count", ErrLimit)
	}
	return nil
}

func (f *flattenState) appendHit(hit display.Hit) error {
	if f.offsetX != 0 {
		var err error
		hit.Area.X, err = f.layout.addHost(hit.Area.X, f.offsetX)
		if err != nil {
			return err
		}
	}
	if f.offsetY != 0 {
		var err error
		hit.Area.Y, err = f.layout.addHost(hit.Area.Y, f.offsetY)
		if err != nil {
			return err
		}
	}
	f.box.Hits = append(f.box.Hits, hit)
	if len(f.box.Hits) > f.layout.limits.MaxNodes {
		return fmt.Errorf("%w: source hit count %d", ErrLimit, len(f.box.Hits))
	}
	return nil
}

func (s *layoutState) offsetItem(item display.Item, dx, dy float32) (display.Item, error) {
	var err error
	item.Bound.X, err = s.addHost(item.Bound.X, dx)
	if err != nil {
		return display.Item{}, err
	}
	item.Bound.Y, err = s.addHost(item.Bound.Y, dy)
	if err != nil {
		return display.Item{}, err
	}
	item.Rect.X, err = s.addHost(item.Rect.X, dx)
	if err != nil {
		return display.Item{}, err
	}
	item.Rect.Y, err = s.addHost(item.Rect.Y, dy)
	if err != nil {
		return display.Item{}, err
	}
	item.At.X, err = s.addHost(item.At.X, dx)
	if err != nil {
		return display.Item{}, err
	}
	item.At.Y, err = s.addHost(item.At.Y, dy)
	if err != nil {
		return display.Item{}, err
	}
	return item, nil
}

func (f *flattenState) visit(fragment *Fragment, x, y geometry.Em) error {
	if fragment == nil {
		return fmt.Errorf("%w: nil fragment in flatten", ErrInvalidInput)
	}
	f.visits++
	if f.visits > f.layout.limits.MaxNodes {
		return fmt.Errorf("%w: fragment visit count %d", ErrLimit, f.visits)
	}
	for _, ancestor := range f.stack {
		if ancestor == fragment {
			return fmt.Errorf("%w: cyclic fragment graph", ErrInvalidInput)
		}
	}
	if len(f.stack) >= f.layout.limits.MaxNesting {
		return fmt.Errorf("%w: fragment nesting depth", ErrLimit)
	}
	f.stack = append(f.stack, fragment)
	defer func() {
		f.stack = f.stack[:len(f.stack)-1]
	}()
	if err := f.layout.appendFragmentHits(f, fragment, x, y); err != nil {
		return err
	}
	for drawIndex := 0; drawIndex < fragment.drawCount; drawIndex++ {
		draw := fragment.drawAt(drawIndex)
		if err := f.tick(); err != nil {
			return err
		}
		itemX, err := add(x, draw.x)
		if err != nil {
			return err
		}
		itemY, err := add(y, draw.y)
		if err != nil {
			return err
		}
		switch draw.kind {
		case drawGroup:
			if err := f.visit(draw.group, itemX, itemY); err != nil {
				return err
			}
		case drawGlyph:
			glyph := draw.glyph
			if glyph == nil {
				return fmt.Errorf("%w: nil glyph", ErrInvalidInput)
			}
			if len(glyph.Glyph.Parts) == 0 {
				if err := f.appendGlyphItem(itemX, itemY, glyph, glyph.Glyph.ExactGlyphRef); err != nil {
					return err
				}
				continue
			}
			for _, part := range glyph.Glyph.Parts {
				partX, err := add(itemX, part.At.X)
				if err != nil {
					return err
				}
				partY, err := add(itemY, part.At.Y)
				if err != nil {
					return err
				}
				partGlyph := f.layout.newGlyphFragment(part.Glyph, glyph.Scale, glyph.Paint)
				if err := f.appendGlyphItem(partX, partY, partGlyph, false); err != nil {
					return err
				}
			}
		case drawRule:
			item, err := f.ruleItem(itemX, itemY, draw.rule)
			if err != nil {
				return err
			}
			if err := f.appendItem(item); err != nil {
				return err
			}
		case drawPath:
			item, err := f.pathItem(itemX, itemY, draw.path)
			if err != nil {
				return err
			}
			if err := f.appendItem(item); err != nil {
				return err
			}
		default:
			return fmt.Errorf("%w: unknown draw kind %d", ErrInvalidInput, draw.kind)
		}
	}
	return nil
}

func (f *flattenState) appendGlyphItem(x, y geometry.Em, glyph *GlyphFragment, exactRef bool) error {
	item, err := f.glyphItem(x, y, glyph)
	if err != nil {
		return err
	}
	if !exactRef && item.Kind == display.ItemGlyphRef {
		return fmt.Errorf("%w: inexact glyph reference without an outline", ErrInvalidInput)
	}
	if item.Kind == display.ItemGlyphPath {
		if err := f.appendItem(item); err != nil {
			return err
		}
	}
	if exactRef && (f.layout.request.GlyphRefs || item.Kind == display.ItemGlyphRef) {
		originY, err := add(y, glyph.Glyph.OriginY)
		if err != nil {
			return err
		}
		hostOriginY, err := f.layout.host(originY)
		if err != nil {
			return err
		}
		ref := display.Item{
			Kind:      display.ItemGlyphRef,
			Paint:     toDisplayPaint(glyph.Paint),
			Bound:     item.Bound,
			PathIndex: display.NoPath,
			FontID:    glyph.Glyph.FontID,
			Glyph:     glyph.Glyph.ID,
			At:        geometry.Point{X: item.At.X, Y: hostOriginY},
			Em:        item.Em,
		}
		if err := f.appendItem(ref); err != nil {
			return err
		}
	}
	return nil
}

func (f *flattenState) glyphItem(x, y geometry.Em, glyph *GlyphFragment) (display.Item, error) {
	if glyph == nil {
		return display.Item{}, fmt.Errorf("%w: nil glyph", ErrInvalidInput)
	}
	hostX, err := f.layout.host(x)
	if err != nil {
		return display.Item{}, err
	}
	hostY, err := f.layout.host(y)
	if err != nil {
		return display.Item{}, err
	}
	item := display.Item{
		Kind:      display.ItemGlyphRef,
		Paint:     toDisplayPaint(glyph.Paint),
		PathIndex: display.NoPath,
		FontID:    glyph.Glyph.FontID,
		Glyph:     glyph.Glyph.ID,
		At:        geometry.Point{X: hostX, Y: hostY},
	}
	em, err := f.layout.host(glyph.Scale)
	if err != nil {
		return display.Item{}, err
	}
	item.Em = em
	if len(glyph.Glyph.Outline) == 0 {
		width, err := f.layout.host(glyph.Glyph.Advance)
		if err != nil {
			return display.Item{}, err
		}
		ascent, err := f.layout.host(glyph.Glyph.Ascent)
		if err != nil {
			return display.Item{}, err
		}
		descent, err := f.layout.host(glyph.Glyph.Descent)
		if err != nil {
			return display.Item{}, err
		}
		item.Bound = geometry.Rect{
			X:      hostX,
			Y:      hostY - ascent,
			Width:  width,
			Height: ascent + descent,
		}
		return item, nil
	}

	key, cacheable := glyphPathTemplateKey(glyph.Glyph)
	pathIndex, err := f.pathTemplate(key, cacheable, glyph.Glyph.Outline)
	if err != nil {
		return display.Item{}, err
	}
	bound, err := f.layout.placedPathBound(f.box.Paths[pathIndex].Bound, item.At, item.Em)
	if err != nil {
		return display.Item{}, err
	}
	item.Kind = display.ItemGlyphPath
	item.PathIndex = pathIndex
	item.Bound = bound
	return item, nil
}

func (f *flattenState) pathItem(x, y geometry.Em, path *pathDraw) (display.Item, error) {
	if path == nil {
		return display.Item{}, fmt.Errorf("%w: nil path", ErrInvalidInput)
	}
	if len(path.path) == 0 {
		return display.Item{}, fmt.Errorf("%w: empty path", ErrInvalidInput)
	}
	if err := f.layout.reservePathSegments(len(path.path)); err != nil {
		return display.Item{}, err
	}
	hostX, err := f.layout.host(x)
	if err != nil {
		return display.Item{}, err
	}
	hostY, err := f.layout.host(y)
	if err != nil {
		return display.Item{}, err
	}
	key, cacheable := pathTemplateKeyFor(path.path)
	pathIndex, err := f.pathTemplate(key, cacheable, path.path)
	if err != nil {
		return display.Item{}, err
	}
	em, err := f.layout.host(geometry.Em(geometry.EmScale))
	if err != nil {
		return display.Item{}, err
	}
	at := geometry.Point{X: hostX, Y: hostY}
	bound, err := f.layout.placedPathBound(f.box.Paths[pathIndex].Bound, at, em)
	if err != nil {
		return display.Item{}, err
	}
	return display.Item{
		Kind:      display.ItemGlyphPath,
		Paint:     toDisplayPaint(path.paint),
		Bound:     bound,
		PathIndex: pathIndex,
		At:        at,
		Em:        em,
	}, nil
}

func pathTemplateKeyFor(path Path) (pathTemplateKey, bool) {
	if len(path) == 0 {
		return pathTemplateKey{}, false
	}
	return pathTemplateKey{first: &path[0], length: len(path)}, true
}

func glyphPathTemplateKey(glyph Glyph) (pathTemplateKey, bool) {
	if glyph.ExactGlyphRef && glyph.FontID != "" && glyph.ID != 0 {
		return pathTemplateKey{
			fontID:  glyph.FontID,
			glyphID: glyph.ID,
			exact:   true,
		}, true
	}
	return pathTemplateKeyFor(glyph.Outline)
}

func (f *flattenState) pathTemplate(key pathTemplateKey, cacheable bool, path Path) (int, error) {
	if cacheable {
		if index, ok := f.cachedPathTemplate(key); ok {
			return index, nil
		}
	}
	template := display.PathTemplate{
		Segments: make([]geometry.Segment, len(path)),
	}
	var minX, minY, maxX, maxY float32
	havePoint := false
	for i, segment := range path {
		if err := f.tick(); err != nil {
			return 0, err
		}
		pointCount, ok := segmentPointCount(segment.Op)
		if !ok {
			return 0, fmt.Errorf("%w: invalid path operation %d", ErrInvalidInput, segment.Op)
		}
		out := &template.Segments[i]
		out.Op = segment.Op
		for j := 0; j < pointCount; j++ {
			point := segment.Points[j]
			x := float32(point.X.Float())
			y := float32(point.Y.Float())
			out.Points[j] = geometry.Point{X: x, Y: y}
			if !havePoint {
				minX, maxX, minY, maxY = x, x, y, y
				havePoint = true
				continue
			}
			minX = minFloat(minX, x)
			maxX = maxFloat(maxX, x)
			minY = minFloat(minY, y)
			maxY = maxFloat(maxY, y)
		}
	}
	if havePoint {
		template.Bound = geometry.Rect{
			X:      minX,
			Y:      minY,
			Width:  maxX - minX,
			Height: maxY - minY,
		}
	}
	index := len(f.box.Paths)
	f.box.Paths = append(f.box.Paths, template)
	if cacheable {
		f.cachePathTemplate(key, index)
	}
	return index, nil
}

func (f *flattenState) cachedPathTemplate(key pathTemplateKey) (int, bool) {
	for i := 0; i < f.inlinePaths; i++ {
		if f.inlinePathKeys[i] == key {
			return f.inlinePathIDs[i], true
		}
	}
	if f.pathCache == nil {
		return 0, false
	}
	index, ok := f.pathCache[key]
	return index, ok
}

func (f *flattenState) cachePathTemplate(key pathTemplateKey, index int) {
	if f.inlinePaths < len(f.inlinePathKeys) {
		f.inlinePathKeys[f.inlinePaths] = key
		f.inlinePathIDs[f.inlinePaths] = index
		f.inlinePaths++
		return
	}
	if f.pathCache == nil {
		f.pathCache = make(map[pathTemplateKey]int)
	}
	f.pathCache[key] = index
}

func (s *layoutState) placedPathBound(local geometry.Rect, at geometry.Point, em float32) (geometry.Rect, error) {
	x := float64(at.X) + float64(local.X)*float64(em)
	y := float64(at.Y) + float64(local.Y)*float64(em)
	width := float64(local.Width) * float64(em)
	height := float64(local.Height) * float64(em)
	for _, value := range []float64{x, y, width, height} {
		if math.IsNaN(value) || math.IsInf(value, 0) || math.Abs(value) > float64(s.limits.MaxDimension) {
			return geometry.Rect{}, fmt.Errorf("%w: placed path bound", ErrLimit)
		}
	}
	return geometry.Rect{
		X:      float32(x),
		Y:      float32(y),
		Width:  float32(width),
		Height: float32(height),
	}, nil
}

func (f *flattenState) tick() error {
	f.operations++
	if f.operations%64 != 0 {
		return nil
	}
	return f.layout.ctx.Err()
}

func segmentPointCount(op geometry.SegOp) (int, bool) {
	if op > geometry.SegClose {
		return 0, false
	}
	return [...]int{1, 1, 2, 3, 0}[op], true
}

func (f *flattenState) ruleItem(x, y geometry.Em, rule *ruleDraw) (display.Item, error) {
	if rule == nil {
		return display.Item{}, fmt.Errorf("%w: nil rule", ErrInvalidInput)
	}
	hostX, err := f.layout.host(x)
	if err != nil {
		return display.Item{}, err
	}
	hostY, err := f.layout.host(y)
	if err != nil {
		return display.Item{}, err
	}
	width, err := f.layout.host(rule.width)
	if err != nil {
		return display.Item{}, err
	}
	thickness, err := f.layout.host(rule.thickness)
	if err != nil {
		return display.Item{}, err
	}
	rect := geometry.Rect{X: hostX, Y: hostY, Width: width, Height: thickness}
	if rule.vertical {
		rect.Width, rect.Height = thickness, width
	}
	return display.Item{
		Kind:      display.ItemRule,
		Paint:     toDisplayPaint(rule.paint),
		Bound:     rect,
		PathIndex: display.NoPath,
		Rect:      rect,
	}, nil
}

func (s *layoutState) host(value geometry.Em) (float32, error) {
	subunits, err := mulDiv(value, s.hostSize, int64(geometry.EmScale))
	if err != nil {
		return 0, err
	}
	if s.limits.MaxDimension > 0 {
		maxSubunits, ok := checkedMultiply(int64(s.limits.MaxDimension), hostSubunits)
		if !ok || int64(subunits) > maxSubunits || int64(subunits) < -maxSubunits {
			return 0, fmt.Errorf("%w: dimension %d", ErrLimit, subunits)
		}
	}
	return toFloat(subunits), nil
}

func toFloat(subunits geometry.Em) float32 {
	if subunits == 0 {
		return 0
	}
	return float32(float64(subunits) / float64(hostSubunits))
}

func checkedMultiply(a, b int64) (int64, bool) {
	if a == 0 || b == 0 {
		return 0, true
	}
	if a > math.MaxInt64/b {
		return 0, false
	}
	return a * b, true
}

func (f *flattenState) overflow() float32 {
	var overflow float32
	for _, item := range f.box.Items {
		bound := item.Bound
		left := maxFloat(0, -bound.X)
		right := maxFloat(0, bound.X+bound.Width-f.box.Width)
		top := maxFloat(0, -bound.Y-f.box.Ascent)
		bottom := maxFloat(0, bound.Y+bound.Height-f.box.Descent)
		overflow = maxFloat(overflow, maxFloat(maxFloat(left, right), maxFloat(top, bottom)))
	}
	return overflow
}

func toDisplayPaint(paint ir.Paint) display.Paint {
	if paint.HasExplicit {
		return display.Paint{
			Explicit: display.RGBA{
				R: paint.Explicit.R,
				G: paint.Explicit.G,
				B: paint.Explicit.B,
				A: paint.Explicit.A,
			},
			HasExplicit: true,
		}
	}
	role := display.RoleInk
	switch paint.Role {
	case ir.PaintError:
		role = display.RoleError
	case ir.PaintPlaceholder:
		role = display.RolePlaceholder
	}
	return display.Paint{Role: role}
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
