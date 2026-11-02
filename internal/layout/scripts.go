package layout

import (
	"fmt"

	"github.com/caelyreth/untie/display"
	"github.com/caelyreth/untie/geometry"
	"github.com/caelyreth/untie/ir"
)

func (s *layoutState) layoutScripts(node *ir.Scripts, env environment, depth int) (*Fragment, error) {
	supEnv := env
	supEnv.Style = env.Style.Superscript()
	subEnv := env
	subEnv.Style = env.Style.Subscript()

	var topNode, bottomNode ir.Node
	topLeftNode, bottomLeftNode := node.TopLeft, node.BottomLeft
	topRightNode, bottomRightNode := node.TopRight, node.BottomRight
	limits := s.usesLimits(node, env)
	if limits {
		topNode, bottomNode = node.Top, node.Bottom
	} else {
		if node.Top != nil {
			if topRightNode != nil {
				return nil, fmt.Errorf("%w: semantic top script conflicts with forced side script", ErrInvalidInput)
			}
			topRightNode = node.Top
		}
		if node.Bottom != nil {
			if bottomRightNode != nil {
				return nil, fmt.Errorf("%w: semantic bottom script conflicts with forced side script", ErrInvalidInput)
			}
			bottomRightNode = node.Bottom
		}
	}

	var top, bottom, base *Fragment
	var err error
	if limits {
		// Limit widths are known before the base is measured so a large
		// operator can be stretched horizontally over its widest limit.
		top, err = s.optionalChild(topNode, supEnv, depth)
		if err != nil {
			return nil, err
		}
		setOwnHitKind(top, display.HitScript)
		bottom, err = s.optionalChild(bottomNode, subEnv, depth)
		if err != nil {
			return nil, err
		}
		setOwnHitKind(bottom, display.HitScript)
		base, err = s.layoutScriptBase(node.BaseItem, env, depth, max(widthOf(top), widthOf(bottom)))
		if err != nil {
			return nil, err
		}
	} else {
		base, err = s.layoutNode(node.BaseItem, env, depth)
		if err != nil {
			return nil, err
		}
	}
	topLeft, err := s.optionalChild(topLeftNode, supEnv, depth)
	if err != nil {
		return nil, err
	}
	setOwnHitKind(topLeft, display.HitScript)
	bottomLeft, err := s.optionalChild(bottomLeftNode, subEnv, depth)
	if err != nil {
		return nil, err
	}
	setOwnHitKind(bottomLeft, display.HitScript)
	topRight, err := s.optionalChild(topRightNode, supEnv, depth)
	if err != nil {
		return nil, err
	}
	setOwnHitKind(topRight, display.HitScript)
	bottomRight, err := s.optionalChild(bottomRightNode, subEnv, depth)
	if err != nil {
		return nil, err
	}
	setOwnHitKind(bottomRight, display.HitScript)

	metrics := s.fontMetrics(env.Style, env.Variant)
	supShift, subShift, err := s.scriptShifts(env.Style, env.Style.Cramped, base, metrics, topLeft, topRight, bottomLeft, bottomRight)
	if err != nil {
		return nil, err
	}
	topLimitShift, bottomLimitShift, err := s.limitShifts(env.Style, base, top, bottom, metrics)
	if err != nil {
		return nil, err
	}

	ascent := base.Ascent
	if value, err := add(supShift, ascentOf(topRight)); err != nil {
		return nil, err
	} else {
		ascent = max(ascent, value)
	}
	if value, err := add(supShift, ascentOf(topLeft)); err != nil {
		return nil, err
	} else {
		ascent = max(ascent, value)
	}
	if value, err := add(topLimitShift, ascentOf(top)); err != nil {
		return nil, err
	} else {
		ascent = max(ascent, value)
	}
	descent := base.Descent
	if value, err := add(subShift, descentOf(bottomRight)); err != nil {
		return nil, err
	} else {
		descent = max(descent, value)
	}
	if value, err := add(subShift, descentOf(bottomLeft)); err != nil {
		return nil, err
	} else {
		descent = max(descent, value)
	}
	if value, err := add(bottomLimitShift, descentOf(bottom)); err != nil {
		return nil, err
	} else {
		descent = max(descent, value)
	}

	spaceAfter, err := s.metricAt(metrics.SpaceAfterScript, env.Style)
	if err != nil {
		return nil, err
	}

	topLeftKern := mathKern(base, topLeft, supShift, KernTopLeft)
	bottomLeftKern := mathKern(base, bottomLeft, subShift, KernBottomLeft)
	topRightKern := mathKern(base, topRight, supShift, KernTopRight)
	bottomRightKern := mathKern(base, bottomRight, subShift, KernBottomRight)
	if base.TextLike && bottomRight != nil {
		bottomRightKern, err = sub(bottomRightKern, base.Italic)
		if err != nil {
			return nil, err
		}
	}

	var preTop, preBottom, postTop, postBottom geometry.Em
	if topLeft != nil {
		preTop, err = sum(spaceAfter, topLeft.Width, topLeftKern)
		if err != nil {
			return nil, err
		}
	}
	if bottomLeft != nil {
		preBottom, err = sum(spaceAfter, bottomLeft.Width, bottomLeftKern)
		if err != nil {
			return nil, err
		}
	}
	if topRight != nil {
		postTop, err = sum(spaceAfter, topRight.Width, topRightKern)
		if err != nil {
			return nil, err
		}
	}
	if bottomRight != nil {
		postBottom, err = sum(spaceAfter, bottomRight.Width, bottomRightKern)
		if err != nil {
			return nil, err
		}
	}
	delta := midpoint(base.Italic)
	topPre, topPost, err := limitWidths(base.Width, widthOf(top), delta, false)
	if err != nil {
		return nil, err
	}
	bottomPre, bottomPost, err := limitWidths(base.Width, widthOf(bottom), delta, true)
	if err != nil {
		return nil, err
	}

	preWidth := max(max(max(preTop, preBottom), topPre), bottomPre)
	preWidth = max(preWidth, 0)
	postWidth := max(max(max(postTop, postBottom), topPost), bottomPost)
	postWidth = max(postWidth, 0)

	width, err := sum(preWidth, base.Width, postWidth)
	if err != nil {
		return nil, err
	}
	fragment, err := newFragment(Metrics{Width: width, Ascent: ascent, Descent: descent}, ir.ClassNormal, env.Style.Size, s.scale(env.Style))
	if err != nil {
		return nil, err
	}
	fragment.BaseAscent = ascent
	fragment.BaseDescent = descent
	fragment.Italic = base.Italic
	fragment.TextLike = false

	baseX := preWidth
	if err := fragment.pushGroup(baseX, 0, base); err != nil {
		return nil, err
	}
	if topLeft != nil {
		offset, err := sub(preWidth, preTop)
		if err != nil {
			return nil, err
		}
		x, err := add(offset, spaceAfter)
		if err != nil {
			return nil, err
		}
		y, err := sub(0, supShift)
		if err != nil {
			return nil, err
		}
		if err := fragment.pushGroup(x, y, topLeft); err != nil {
			return nil, err
		}
	}
	if bottomLeft != nil {
		offset, err := sub(preWidth, preBottom)
		if err != nil {
			return nil, err
		}
		x, err := add(offset, spaceAfter)
		if err != nil {
			return nil, err
		}
		y := subShift
		if err := fragment.pushGroup(x, y, bottomLeft); err != nil {
			return nil, err
		}
	}
	if topRight != nil {
		x, err := sum(preWidth, base.Width, topRightKern)
		if err != nil {
			return nil, err
		}
		y, err := sub(0, supShift)
		if err != nil {
			return nil, err
		}
		if err := fragment.pushGroup(x, y, topRight); err != nil {
			return nil, err
		}
	}
	if bottomRight != nil {
		x, err := sum(preWidth, base.Width, bottomRightKern)
		if err != nil {
			return nil, err
		}
		y := subShift
		if err := fragment.pushGroup(x, y, bottomRight); err != nil {
			return nil, err
		}
	}
	if top != nil {
		x, err := sub(preWidth, topPre)
		if err != nil {
			return nil, err
		}
		y, err := sub(0, topLimitShift)
		if err != nil {
			return nil, err
		}
		if err := fragment.pushGroup(x, y, top); err != nil {
			return nil, err
		}
	}
	if bottom != nil {
		x, err := sub(preWidth, bottomPre)
		if err != nil {
			return nil, err
		}
		y := bottomLimitShift
		if err := fragment.pushGroup(x, y, bottom); err != nil {
			return nil, err
		}
	}
	return withClass(fragment, node.Class), nil
}

func (s *layoutState) layoutScriptBase(node ir.Node, env environment, depth int, target geometry.Em) (*Fragment, error) {
	item, ok := node.(*ir.Glyph)
	if !ok || target == 0 || item.Class != ir.ClassLarge {
		return s.layoutNode(node, env, depth)
	}
	if err := s.check(depth); err != nil {
		return nil, err
	}
	if err := s.checkTextLimit(node); err != nil {
		return nil, err
	}
	nodeEnv := s.nodeEnvironment(node, env)
	stretch := s.automaticDisplayStretch(item.Class, nodeEnv)
	if stretch == nil && target > 0 {
		stretch = &stretchSpec{axis: Horizontal, target: target}
	}
	fragment, err := s.layoutClassGlyphText(item.Text, item.Class, nodeEnv, stretch)
	if err != nil {
		return nil, err
	}
	if err := s.attachNodeHit(fragment, node, depth); err != nil {
		return nil, err
	}
	return withClass(fragment, item.Class), nil
}

func (s *layoutState) usesLimits(node *ir.Scripts, env environment) bool {
	placement := node.Limits
	if placement == ir.LimitsAuto {
		if base := node.BaseItem.NodeBase(); base != nil && base.Limits != ir.LimitsAuto {
			placement = base.Limits
		}
	}
	switch placement {
	case ir.LimitsAlways:
		return true
	case ir.LimitsNever:
		return false
	}
	class := node.Class
	if base := node.BaseItem.NodeBase(); base != nil && base.Class != ir.ClassNormal {
		class = base.Class
	}
	return env.Style.Size == Display && class == ir.ClassLarge
}

func mathKern(base, script *Fragment, shift geometry.Em, corner KernCorner) geometry.Em {
	if base == nil || script == nil {
		return 0
	}
	var top, bottom geometry.Em
	switch corner {
	case KernTopLeft, KernTopRight:
		top = base.Ascent - shift
		bottom = shift - script.Descent
	default:
		top = script.Ascent - shift
		bottom = shift - base.Descent
	}
	summed := func(height geometry.Em) geometry.Em {
		return base.kernAtHeight(corner, height) + script.kernAtHeight(corner.inverse(), height)
	}
	return max(summed(top), summed(bottom))
}

func (s *layoutState) optionalChild(node ir.Node, env environment, depth int) (*Fragment, error) {
	if node == nil {
		return nil, nil
	}
	return s.layoutNode(node, env, depth)
}

func (s *layoutState) scriptShifts(
	style Style,
	cramped bool,
	base *Fragment,
	metrics MathMetrics,
	topLeft, topRight, bottomLeft, bottomRight *Fragment,
) (geometry.Em, geometry.Em, error) {
	metric := func(value geometry.Em) (geometry.Em, error) { return s.metricAt(value, style) }
	supShiftUp, err := metric(metrics.SuperscriptShiftUp)
	if err != nil {
		return 0, 0, err
	}
	if cramped {
		supShiftUp, err = metric(metrics.SuperscriptShiftUpCramped)
		if err != nil {
			return 0, 0, err
		}
	}
	supBottomMin, err := metric(metrics.SuperscriptBottomMin)
	if err != nil {
		return 0, 0, err
	}
	supBottomMax, err := metric(metrics.SuperscriptBottomMaxWithSub)
	if err != nil {
		return 0, 0, err
	}
	supDropMax, err := metric(metrics.SuperscriptBaselineDropMax)
	if err != nil {
		return 0, 0, err
	}
	gapMin, err := metric(metrics.SubSuperscriptGapMin)
	if err != nil {
		return 0, 0, err
	}
	subShiftDown, err := metric(metrics.SubscriptShiftDown)
	if err != nil {
		return 0, 0, err
	}
	subTopMax, err := metric(metrics.SubscriptTopMax)
	if err != nil {
		return 0, 0, err
	}
	subDropMin, err := metric(metrics.SubscriptBaselineDropMin)
	if err != nil {
		return 0, 0, err
	}

	var shiftUp, shiftDown geometry.Em
	if topLeft != nil || topRight != nil {
		shiftUp = supShiftUp
		if !base.TextLike {
			value, err := sub(base.BaseAscent, supDropMax)
			if err != nil {
				return 0, 0, err
			}
			shiftUp = max(shiftUp, value)
		}
		value, err := add(supBottomMin, descentOf(topLeft))
		if err != nil {
			return 0, 0, err
		}
		shiftUp = max(shiftUp, value)
		value, err = add(supBottomMin, descentOf(topRight))
		if err != nil {
			return 0, 0, err
		}
		shiftUp = max(shiftUp, value)
	}
	if bottomLeft != nil || bottomRight != nil {
		shiftDown = subShiftDown
		if !base.TextLike {
			value, err := add(base.BaseDescent, subDropMin)
			if err != nil {
				return 0, 0, err
			}
			shiftDown = max(shiftDown, value)
		}
		value, err := sub(ascentOf(bottomLeft), subTopMax)
		if err != nil {
			return 0, 0, err
		}
		shiftDown = max(shiftDown, value)
		value, err = sub(ascentOf(bottomRight), subTopMax)
		if err != nil {
			return 0, 0, err
		}
		shiftDown = max(shiftDown, value)
	}

	for _, pair := range [][2]*Fragment{{topLeft, bottomLeft}, {topRight, bottomRight}} {
		sup, bottomScript := pair[0], pair[1]
		if sup == nil || bottomScript == nil {
			continue
		}
		supBottom, err := sub(shiftUp, sup.Descent)
		if err != nil {
			return 0, 0, err
		}
		subTop, err := sub(bottomScript.Ascent, shiftDown)
		if err != nil {
			return 0, 0, err
		}
		gap, err := sub(supBottom, subTop)
		if err != nil {
			return 0, 0, err
		}
		if gap >= gapMin {
			continue
		}
		increase, err := sub(gapMin, gap)
		if err != nil {
			return 0, 0, err
		}
		supRoom, err := sub(supBottomMax, supBottom)
		if err != nil {
			return 0, 0, err
		}
		supOnly := clamp(supRoom, 0, increase)
		rest := (increase - supOnly) / 2
		bump, err := add(supOnly, rest)
		if err != nil {
			return 0, 0, err
		}
		shiftUp, err = add(shiftUp, bump)
		if err != nil {
			return 0, 0, err
		}
		shiftDown, err = add(shiftDown, rest)
		if err != nil {
			return 0, 0, err
		}
	}
	return shiftUp, shiftDown, nil
}

func (s *layoutState) limitShifts(style Style, base, top, bottom *Fragment, metrics MathMetrics) (geometry.Em, geometry.Em, error) {
	var topShift, bottomShift geometry.Em
	if top != nil {
		rise, err := s.metricAt(metrics.UpperLimitBaselineRiseMin, style)
		if err != nil {
			return 0, 0, err
		}
		gap, err := s.metricAt(metrics.UpperLimitGapMin, style)
		if err != nil {
			return 0, 0, err
		}
		gapPart, err := add(gap, top.Descent)
		if err != nil {
			return 0, 0, err
		}
		topShift, err = add(base.Ascent, max(rise, gapPart))
		if err != nil {
			return 0, 0, err
		}
	}
	if bottom != nil {
		drop, err := s.metricAt(metrics.LowerLimitBaselineDropMin, style)
		if err != nil {
			return 0, 0, err
		}
		gap, err := s.metricAt(metrics.LowerLimitGapMin, style)
		if err != nil {
			return 0, 0, err
		}
		gapPart, err := add(gap, bottom.Ascent)
		if err != nil {
			return 0, 0, err
		}
		bottomShift, err = add(base.Descent, max(drop, gapPart))
		if err != nil {
			return 0, 0, err
		}
	}
	return topShift, bottomShift, nil
}

func limitWidths(baseWidth, scriptWidth, delta geometry.Em, below bool) (geometry.Em, geometry.Em, error) {
	difference, err := sub(scriptWidth, baseWidth)
	if err != nil {
		return 0, 0, err
	}
	half := difference / 2
	if below {
		pre, err := add(half, delta)
		if err != nil {
			return 0, 0, err
		}
		post, err := sub(half, delta)
		return pre, post, err
	}
	pre, err := sub(half, delta)
	if err != nil {
		return 0, 0, err
	}
	post, err := add(half, delta)
	return pre, post, err
}

func widthOf(fragment *Fragment) geometry.Em {
	if fragment == nil {
		return 0
	}
	return fragment.Width
}

func ascentOf(fragment *Fragment) geometry.Em {
	if fragment == nil {
		return 0
	}
	return fragment.Ascent
}

func descentOf(fragment *Fragment) geometry.Em {
	if fragment == nil {
		return 0
	}
	return fragment.Descent
}
