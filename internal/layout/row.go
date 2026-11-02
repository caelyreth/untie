package layout

import (
	"github.com/caelyreth/untie/geometry"
	"github.com/caelyreth/untie/ir"
)

func (s *layoutState) layoutRow(nodes []ir.Node, env environment, depth int) (*Fragment, error) {
	row := emptyFragment(env.Style.Size, s.scale(env.Style))
	row.Class = ir.ClassNormal
	row.TextLike = true

	var x, ascent, descent, italic geometry.Em
	var previousClass *ir.MathClass
	var previousNode ir.Node
	var previousEnv environment
	var pendingSpace geometry.Em
	var pendingVisible, pendingWeak bool
	var afterExplicitSpace bool

	flushSpace := func() error {
		if !pendingVisible || pendingSpace == 0 {
			pendingVisible = false
			pendingWeak = false
			return nil
		}
		var err error
		x, err = add(x, pendingSpace)
		if err != nil {
			return err
		}
		pendingSpace = 0
		pendingVisible = false
		pendingWeak = false
		afterExplicitSpace = true
		return nil
	}

	for _, node := range nodes {
		if err := s.check(depth); err != nil {
			return nil, err
		}
		if space, ok := node.(*ir.Space); ok {
			nodeEnv := s.nodeEnvironment(node, env)
			if space.Type == ir.SpaceIgnored {
				continue
			}
			metrics := s.fontMetrics(nodeEnv.Style, nodeEnv.Variant)
			var width geometry.Em
			var err error
			if space.Type == ir.SpaceInterword {
				width, err = s.metricAt(metrics.SpaceWidth, nodeEnv.Style)
			} else {
				width, err = s.length(space.Amount, nodeEnv.Style, metrics)
			}
			if err != nil {
				return nil, err
			}
			if space.Weak && pendingVisible && pendingWeak {
				pendingSpace = max(pendingSpace, width)
			} else {
				if err := flushSpace(); err != nil {
					return nil, err
				}
				pendingSpace = width
				pendingVisible = true
				pendingWeak = space.Weak
			}
			afterExplicitSpace = true
			continue
		}
		if _, ok := node.(*ir.Break); ok {
			continue
		}
		if _, ok := node.(*ir.AlignMarker); ok {
			continue
		}

		nodeEnv := s.nodeEnvironment(node, env)
		fragment, err := s.layoutNode(node, env, depth)
		if err != nil {
			return nil, err
		}
		class := NormalizeClass(previousClass, fragment.Class)
		fragment.Class = class

		var explicitBefore geometry.Em
		var hasExplicit bool
		if previousNode != nil {
			if value, ok, err := s.rightSpacing(previousNode, previousEnv); err != nil {
				return nil, err
			} else if ok {
				explicitBefore, err = add(explicitBefore, value)
				if err != nil {
					return nil, err
				}
				hasExplicit = true
			}
		}
		if value, ok, err := s.leftSpacing(node, nodeEnv); err != nil {
			return nil, err
		} else if ok {
			explicitBefore, err = add(explicitBefore, value)
			if err != nil {
				return nil, err
			}
			hasExplicit = true
		}

		if pendingVisible {
			if err := flushSpace(); err != nil {
				return nil, err
			}
		} else if !hasExplicit && !afterExplicitSpace && previousClass != nil &&
			!previousNode.NodeBase().IgnoreAsSpace && !node.NodeBase().IgnoreAsSpace {
			local := MathSpacing(*previousClass, class, nodeEnv.Style)
			if local != 0 {
				auto, err := s.metricAt(local, nodeEnv.Style)
				if err != nil {
					return nil, err
				}
				explicitBefore = auto
			}
		}
		if explicitBefore != 0 {
			x, err = add(x, explicitBefore)
			if err != nil {
				return nil, err
			}
		}

		if err := row.pushGroup(x, 0, fragment); err != nil {
			return nil, err
		}
		x, err = add(x, fragment.Width)
		if err != nil {
			return nil, err
		}
		ascent = max(ascent, fragment.Ascent)
		descent = max(descent, fragment.Descent)
		if !fragment.TextLike {
			row.TextLike = false
		}
		if fragment.Width != 0 {
			italic = fragment.Italic
		}
		previousClass = &class
		previousNode = node
		previousEnv = nodeEnv
		afterExplicitSpace = false
	}

	if pendingVisible {
		if err := flushSpace(); err != nil {
			return nil, err
		}
	}
	if previousNode != nil {
		right, ok, err := s.rightSpacing(previousNode, previousEnv)
		if err != nil {
			return nil, err
		}
		if ok {
			x, err = add(x, right)
			if err != nil {
				return nil, err
			}
		}
	}

	row.Width = x
	row.Ascent = ascent
	row.Descent = descent
	row.BaseAscent = ascent
	row.BaseDescent = descent
	row.Italic = italic
	return row, nil
}

func (s *layoutState) leftSpacing(node ir.Node, env environment) (geometry.Em, bool, error) {
	base := node.NodeBase()
	if base == nil || base.LeftSpacing.IsZero() {
		return 0, false, nil
	}
	metrics := s.fontMetrics(env.Style, env.Variant)
	value, err := s.length(base.LeftSpacing, env.Style, metrics)
	return value, true, err
}

func (s *layoutState) rightSpacing(node ir.Node, env environment) (geometry.Em, bool, error) {
	base := node.NodeBase()
	if base == nil || base.RightSpacing.IsZero() {
		return 0, false, nil
	}
	metrics := s.fontMetrics(env.Style, env.Variant)
	value, err := s.length(base.RightSpacing, env.Style, metrics)
	return value, true, err
}
