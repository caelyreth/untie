package layout

import (
	"fmt"

	"github.com/caelyreth/untie/display"
	"github.com/caelyreth/untie/geometry"
	"github.com/caelyreth/untie/ir"
)

func (s *layoutState) layoutFenced(node *ir.Fenced, env environment, depth int) (*Fragment, error) {
	body, err := s.layoutRow(node.Body, env, depth)
	if err != nil {
		return nil, err
	}
	bodyHeight, err := body.Height()
	if err != nil {
		return nil, err
	}
	target := bodyHeight
	if matrixFenceBody(node) {
		// Matrix constructors stretch their delimiters relative to 1.1 times
		// the table body height, independently of \left...\right balancing.
		target, err = mulDiv(bodyHeight, 11, 10)
		if err != nil {
			return nil, err
		}
	} else if node.Balanced {
		metrics := s.fontMetrics(env.Style, env.Variant)
		axis, err := s.metricAt(metrics.AxisHeight, env.Style)
		if err != nil {
			return nil, err
		}
		above, err := sub(body.Ascent, axis)
		if err != nil {
			return nil, err
		}
		below, err := add(body.Descent, axis)
		if err != nil {
			return nil, err
		}
		target, err = mulDiv(max(above, below), 2, 1)
		if err != nil {
			return nil, err
		}
	}
	target, err = s.delimiterSizingTarget(node.Sizing, target, env)
	if err != nil {
		return nil, err
	}
	open, err := s.fenceDelimiter(node.Open, env, depth, target)
	if err != nil {
		return nil, err
	}
	close, err := s.fenceDelimiter(node.Close, env, depth, target)
	if err != nil {
		return nil, err
	}

	width := geometry.Em(0)
	ascent, descent := body.Ascent, body.Descent
	for _, part := range []*Fragment{open, body, close} {
		if part == nil {
			continue
		}
		width, err = add(width, part.Width)
		if err != nil {
			return nil, err
		}
		ascent = max(ascent, part.Ascent)
		descent = max(descent, part.Descent)
	}
	fragment, err := s.newFragment(Metrics{Width: width, Ascent: ascent, Descent: descent}, ir.ClassNormal, env.Style.Size, s.scale(env.Style))
	if err != nil {
		return nil, err
	}
	drawCount := 0
	for _, part := range []*Fragment{open, body, close} {
		if part != nil {
			drawCount++
		}
	}
	fragment.reserveDraws(drawCount)
	fragment.BaseAscent = ascent
	fragment.BaseDescent = descent
	fragment.TextLike = false
	x := geometry.Em(0)
	for _, part := range []*Fragment{open, body, close} {
		if part == nil {
			continue
		}
		if err := fragment.pushGroup(x, 0, part); err != nil {
			return nil, err
		}
		x, err = add(x, part.Width)
		if err != nil {
			return nil, err
		}
	}
	switch {
	case close != nil:
		fragment.Italic = close.Italic
	case body != nil:
		fragment.Italic = body.Italic
	case open != nil:
		fragment.Italic = open.Italic
	}
	return withClass(fragment, node.Class), nil
}

func (s *layoutState) delimiterSizingTarget(
	sizing ir.DelimiterSizing,
	contentTarget geometry.Em,
	env environment,
) (geometry.Em, error) {
	if !sizing.Valid() {
		return 0, fmt.Errorf("%w: invalid delimiter sizing", ErrInvalidInput)
	}
	switch sizing.Kind {
	case ir.DelimiterSizingContent:
		return contentTarget, nil
	case ir.DelimiterSizingRelative:
		const percentScale = int64(100 * 1_000_000)
		return mulDiv(contentTarget, sizing.Value.Value, percentScale)
	case ir.DelimiterSizingAbsolute:
		metrics := s.fontMetrics(env.Style, env.Variant)
		return s.length(sizing.Value, env.Style, metrics)
	default:
		return 0, fmt.Errorf("%w: unknown delimiter sizing kind %d", ErrInvalidInput, sizing.Kind)
	}
}

func matrixFenceBody(node *ir.Fenced) bool {
	if node == nil || len(node.Body) != 1 {
		return false
	}
	_, ok := node.Body[0].(*ir.Table)
	return ok
}

func (s *layoutState) fenceDelimiter(node ir.Node, env environment, depth int, target geometry.Em) (*Fragment, error) {
	if node == nil {
		return nil, nil
	}
	nodeEnv := s.nodeEnvironment(node, env)
	shortFall, err := s.metricAt(geometry.Em(geometry.EmScale/10), nodeEnv.Style)
	if err != nil {
		return nil, err
	}
	stretch := &stretchSpec{axis: Vertical, target: target, shortFall: shortFall}
	switch item := node.(type) {
	case *ir.Glyph:
		if err := s.check(depth); err != nil {
			return nil, err
		}
		if err := s.checkTextLimit(node); err != nil {
			return nil, err
		}
		fragment, err := s.layoutGlyphText(item.Text, nodeEnv, stretch)
		if err != nil {
			return nil, err
		}
		if err := s.attachNodeHitAs(fragment, node, depth, display.HitFence); err != nil {
			return nil, err
		}
		return withClass(fragment, item.Class), nil
	case *ir.Operator:
		if err := s.check(depth); err != nil {
			return nil, err
		}
		if err := s.checkTextLimit(node); err != nil {
			return nil, err
		}
		fragment, err := s.layoutGlyphText(item.Text, nodeEnv, stretch)
		if err != nil {
			return nil, err
		}
		if err := s.attachNodeHitAs(fragment, node, depth, display.HitFence); err != nil {
			return nil, err
		}
		return withClass(fragment, item.Class), nil
	default:
		return s.layoutNode(node, nodeEnv, depth)
	}
}
