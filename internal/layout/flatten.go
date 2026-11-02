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
	width, err := s.host(root.Width)
	if err != nil {
		return nil, err
	}
	ascent, err := s.host(root.Ascent)
	if err != nil {
		return nil, err
	}
	descent, err := s.host(root.Descent)
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
	layout       *layoutState
	box          *display.Box
	pathSegments int
	operations   int
}

func (f *flattenState) visit(fragment *Fragment, x, y geometry.Em) error {
	if fragment == nil {
		return fmt.Errorf("%w: nil fragment in flatten", ErrInvalidInput)
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
			item, err := f.glyphItem(itemX, itemY, draw.glyph)
			if err != nil {
				return err
			}
			if item.Kind == display.ItemGlyphPath {
				f.box.Items = append(f.box.Items, item)
			}
			if f.layout.request.GlyphRefs || item.Kind == display.ItemGlyphRef {
				ref := display.Item{
					Kind:   display.ItemGlyphRef,
					Paint:  toDisplayPaint(draw.glyph.Paint),
					Bound:  item.Bound,
					FontID: draw.glyph.Glyph.FontID,
					Glyph:  draw.glyph.Glyph.ID,
					At:     geometry.Point{X: toFloat(itemX), Y: toFloat(itemY)},
					Em:     item.Em,
				}
				f.box.Items = append(f.box.Items, ref)
			}
			if len(f.box.Items) > f.layout.limits.MaxOutputItems {
				return fmt.Errorf("%w: output item count", ErrLimit)
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
		default:
			return fmt.Errorf("%w: unknown draw kind %d", ErrInvalidInput, draw.kind)
		}
	}
	return nil
}

func (f *flattenState) glyphItem(x, y geometry.Em, glyph *GlyphFragment) (display.Item, error) {
	if glyph == nil {
		return display.Item{}, fmt.Errorf("%w: nil glyph", ErrInvalidInput)
	}
	item := display.Item{
		Kind:   display.ItemGlyphRef,
		Paint:  toDisplayPaint(glyph.Paint),
		FontID: glyph.Glyph.FontID,
		Glyph:  glyph.Glyph.ID,
		At:     geometry.Point{X: toFloat(x), Y: toFloat(y)},
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
			X:      toFloat(x),
			Y:      toFloat(y) - ascent,
			Width:  width,
			Height: ascent + descent,
		}
		return item, nil
	}

	path := make([]geometry.Segment, len(glyph.Glyph.Outline))
	f.pathSegments += len(path)
	if f.pathSegments > f.layout.limits.MaxPathSegments {
		return display.Item{}, fmt.Errorf("%w: path segment count %d", ErrLimit, f.pathSegments)
	}
	var minX, minY, maxX, maxY float32
	havePoint := false
	for i, segment := range glyph.Glyph.Outline {
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
