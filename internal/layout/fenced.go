package layout

import (
	"github.com/caelyreth/untie/geometry"
	"github.com/caelyreth/untie/ir"
)

func (s *layoutState) layoutFenced(node *ir.Fenced, env environment, depth int) (*Fragment, error) {
	body, err := s.layoutRow(node.Body, env, depth)
	if err != nil {
		return nil, err
	}
	target, err := body.Height()
	if err != nil {
		return nil, err
	}
	if node.Balanced {
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
	fragment, err := newFragment(Metrics{Width: width, Ascent: ascent, Descent: descent}, ir.ClassNormal, env.Style.Size, s.scale(env.Style))
	if err != nil {
		return nil, err
	}
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
	return withClass(fragment, node.Class), nil
}

func (s *layoutState) fenceDelimiter(node ir.Node, env environment, depth int, target geometry.Em) (*Fragment, error) {
	if node == nil {
		return nil, nil
	}
	nodeEnv := s.nodeEnvironment(node, env)
	switch item := node.(type) {
	case *ir.Glyph:
		if err := s.check(depth); err != nil {
			return nil, err
		}
		if err := s.checkTextLimit(node); err != nil {
			return nil, err
		}
		fragment, err := s.layoutGlyphText(item.Text, nodeEnv, &stretchSpec{axis: Vertical, target: target})
		if err != nil {
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
		fragment, err := s.layoutGlyphText(item.Text, nodeEnv, &stretchSpec{axis: Vertical, target: target})
		if err != nil {
			return nil, err
		}
		return withClass(fragment, item.Class), nil
	default:
		return s.layoutNode(node, nodeEnv, depth)
	}
}
