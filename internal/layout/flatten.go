package layout

import (
	"fmt"
	"math"

	"github.com/caelyreth/untie/display"
	"github.com/caelyreth/untie/geometry"
	"github.com/caelyreth/untie/ir"
)

func (s *layoutState) flatten(root *Fragment) (*display.Box, error) {
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

	state := flattenState{
		layout: s,
		stack:  make(map[*Fragment]struct{}),
		box: &display.Box{
			Width:      width,
			Ascent:     ascent,
			Descent:    descent,
			Italic:     italic,
			AxisHeight: axisHeight,
		},
	}
	if err := state.visit(root, 0, 0); err != nil {
		return nil, err
	}
	if len(state.box.Items) > s.limits.MaxOutputItems {
		return nil, fmt.Errorf("%w: output item count %d", ErrLimit, len(state.box.Items))
	}
	state.box.Overflow = state.overflow()
	if state.layout.mode == ir.Display && state.layout.request.MaxWidth > 0 {
		state.box.Overflow = maxFloat(state.box.Overflow, state.box.Width-state.layout.request.MaxWidth)
		state.box.Overflow = maxFloat(0, state.box.Overflow)
	}
	return state.box, nil
}

type flattenState struct {
	layout     *layoutState
	box        *display.Box
	stack      map[*Fragment]struct{}
	visits     int
	operations int
}

func (f *flattenState) visit(fragment *Fragment, x, y geometry.Em) error {
	if fragment == nil {
		return fmt.Errorf("%w: nil fragment in flatten", ErrInvalidInput)
	}
	f.visits++
	if f.visits > f.layout.limits.MaxNodes {
		return fmt.Errorf("%w: fragment visit count %d", ErrLimit, f.visits)
	}
	if _, exists := f.stack[fragment]; exists {
		return fmt.Errorf("%w: cyclic fragment graph", ErrInvalidInput)
	}
	if len(f.stack) >= f.layout.limits.MaxNesting {
		return fmt.Errorf("%w: fragment nesting depth", ErrLimit)
	}
	f.stack[fragment] = struct{}{}
	defer delete(f.stack, fragment)
	if err := f.layout.appendFragmentHits(f, fragment, x, y); err != nil {
		return err
	}
	if len(f.box.Hits) > f.layout.limits.MaxNodes {
		return fmt.Errorf("%w: source hit count %d", ErrLimit, len(f.box.Hits))
	}
	for _, draw := range fragment.items {
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
				partGlyph := &GlyphFragment{
					Glyph: part.Glyph,
					Scale: glyph.Scale,
					Paint: glyph.Paint,
				}
				if err := f.appendGlyphItem(partX, partY, partGlyph, false); err != nil {
					return err
				}
			}
		case drawRule:
			item, err := f.ruleItem(itemX, itemY, draw.rule)
			if err != nil {
				return err
			}
			f.box.Items = append(f.box.Items, item)
			if len(f.box.Items) > f.layout.limits.MaxOutputItems {
				return fmt.Errorf("%w: output item count", ErrLimit)
			}
		case drawPath:
			item, err := f.pathItem(itemX, itemY, draw.path)
			if err != nil {
				return err
			}
			f.box.Items = append(f.box.Items, item)
			if len(f.box.Items) > f.layout.limits.MaxOutputItems {
				return fmt.Errorf("%w: output item count", ErrLimit)
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
		f.box.Items = append(f.box.Items, item)
	}
	if exactRef && (f.layout.request.GlyphRefs || item.Kind == display.ItemGlyphRef) {
		ref := display.Item{
			Kind:   display.ItemGlyphRef,
			Paint:  toDisplayPaint(glyph.Paint),
			Bound:  item.Bound,
			FontID: glyph.Glyph.FontID,
			Glyph:  glyph.Glyph.ID,
			At:     item.At,
			Em:     item.Em,
		}
		f.box.Items = append(f.box.Items, ref)
	}
	if len(f.box.Items) > f.layout.limits.MaxOutputItems {
		return fmt.Errorf("%w: output item count", ErrLimit)
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
		Kind:   display.ItemGlyphRef,
		Paint:  toDisplayPaint(glyph.Paint),
		FontID: glyph.Glyph.FontID,
		Glyph:  glyph.Glyph.ID,
		At:     geometry.Point{X: hostX, Y: hostY},
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

	path := make([]geometry.Segment, len(glyph.Glyph.Outline))
	var minX, minY, maxX, maxY float32
	havePoint := false
	for i, segment := range glyph.Glyph.Outline {
		if err := f.tick(); err != nil {
			return display.Item{}, err
		}
		pointCount, ok := segmentPointCount(segment.Op)
		if !ok {
			return display.Item{}, fmt.Errorf("%w: invalid path operation %d", ErrInvalidInput, segment.Op)
		}
		path[i].Op = segment.Op
		for j := 0; j < pointCount; j++ {
			point := segment.Points[j]
			px, err := add(x, point.X)
			if err != nil {
				return display.Item{}, err
			}
			py, err := add(y, point.Y)
			if err != nil {
				return display.Item{}, err
			}
			hostX, err := f.layout.host(px)
			if err != nil {
				return display.Item{}, err
			}
			hostY, err := f.layout.host(py)
			if err != nil {
				return display.Item{}, err
			}
			path[i].Points[j] = geometry.Point{X: hostX, Y: hostY}
			if !havePoint {
				minX, maxX = hostX, hostX
				minY, maxY = hostY, hostY
				havePoint = true
			} else {
				minX = minFloat(minX, hostX)
				maxX = maxFloat(maxX, hostX)
				minY = minFloat(minY, hostY)
				maxY = maxFloat(maxY, hostY)
			}
		}
	}
	item.Kind = display.ItemGlyphPath
	item.Path = path
	if havePoint {
		item.Bound = geometry.Rect{X: minX, Y: minY, Width: maxX - minX, Height: maxY - minY}
	}
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
	segments := make([]geometry.Segment, len(path.path))
	var minX, minY, maxX, maxY float32
	havePoint := false
	for i, segment := range path.path {
		if err := f.tick(); err != nil {
			return display.Item{}, err
		}
		pointCount, ok := segmentPointCount(segment.Op)
		if !ok {
			return display.Item{}, fmt.Errorf("%w: invalid path operation %d", ErrInvalidInput, segment.Op)
		}
		segments[i].Op = segment.Op
		for j := 0; j < pointCount; j++ {
			point := segment.Points[j]
			px, err := add(x, point.X)
			if err != nil {
				return display.Item{}, err
			}
			py, err := add(y, point.Y)
			if err != nil {
				return display.Item{}, err
			}
			hx, err := f.layout.host(px)
			if err != nil {
				return display.Item{}, err
			}
			hy, err := f.layout.host(py)
			if err != nil {
				return display.Item{}, err
			}
			segments[i].Points[j] = geometry.Point{X: hx, Y: hy}
			if !havePoint {
				minX, maxX, minY, maxY = hx, hx, hy, hy
				havePoint = true
			} else {
				minX = minFloat(minX, hx)
				maxX = maxFloat(maxX, hx)
				minY = minFloat(minY, hy)
				maxY = maxFloat(maxY, hy)
			}
		}
	}
	bound := geometry.Rect{X: hostX, Y: hostY}
	if havePoint {
		bound = geometry.Rect{X: minX, Y: minY, Width: maxX - minX, Height: maxY - minY}
	}
	return display.Item{
		Kind:  display.ItemGlyphPath,
		Paint: toDisplayPaint(path.paint),
		Bound: bound,
		Path:  segments,
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
	switch op {
	case geometry.SegMoveTo, geometry.SegLineTo:
		return 1, true
	case geometry.SegQuadTo:
		return 2, true
	case geometry.SegCubeTo:
		return 3, true
	case geometry.SegClose:
		return 0, true
	default:
		return 0, false
	}
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
		Kind:  display.ItemRule,
		Paint: toDisplayPaint(rule.paint),
		Bound: rect,
		Rect:  rect,
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
