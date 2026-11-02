package layout

import (
	"github.com/caelyreth/untie/geometry"
	"github.com/caelyreth/untie/ir"
)

func (s *layoutState) layoutLine(node *ir.Line, env environment, depth int) (*Fragment, error) {
	bodyEnv := env
	if node.Position == ir.PositionAbove {
		bodyEnv.Style.Cramped = true
	}
	body, err := s.layoutNode(node.Body, bodyEnv, depth)
	if err != nil {
		return nil, err
	}
	metrics := s.fontMetrics(env.Style, env.Variant)
	thicknessFallback := metrics.OverbarRuleThickness
	gapFallback := metrics.OverbarVerticalGap
	extraFallback := metrics.OverbarExtraAscender
	if node.Position == ir.PositionBelow {
		thicknessFallback = metrics.UnderbarRuleThickness
		gapFallback = metrics.UnderbarVerticalGap
		extraFallback = metrics.UnderbarExtraDescender
	}
	thickness, err := s.orMetric(node.Thickness, thicknessFallback, env.Style, metrics)
	if err != nil {
		return nil, err
	}
	gap, err := s.orMetric(node.Gap, gapFallback, env.Style, metrics)
	if err != nil {
		return nil, err
	}
	extra, err := s.metricAt(extraFallback, env.Style)
	if err != nil {
		return nil, err
	}

	extraHeight, err := sum(extra, thickness, gap)
	if err != nil {
		return nil, err
	}
	bodyHeight, err := body.Height()
	if err != nil {
		return nil, err
	}
	height, err := add(bodyHeight, extraHeight)
	if err != nil {
		return nil, err
	}
	var lineTop, bodyTop, baseline geometry.Em
	if node.Position == ir.PositionAbove {
		lineTop = extra
		bodyTop = extraHeight
		baseline, err = add(body.Ascent, extraHeight)
		if err != nil {
			return nil, err
		}
	} else {
		lineTop, err = sum(bodyHeight, gap)
		if err != nil {
			return nil, err
		}
		bodyTop = 0
		baseline = body.Ascent
	}
	ascent := baseline
	descent, err := sub(height, baseline)
	if err != nil {
		return nil, err
	}
	fragment, err := newFragment(Metrics{Width: body.Width, Ascent: ascent, Descent: descent}, ir.ClassNormal, env.Style.Size, s.scale(env.Style))
	if err != nil {
		return nil, err
	}
	fragment.BaseAscent = ascent
	fragment.BaseDescent = descent
	fragment.Italic = body.Italic
	fragment.TextLike = body.TextLike
	bodyY, err := sub(bodyTop, baseline)
	if err != nil {
		return nil, err
	}
	lineY, err := sub(lineTop, baseline)
	if err != nil {
		return nil, err
	}
	if err := fragment.pushGroup(0, bodyY, body); err != nil {
		return nil, err
	}
	if err := fragment.pushRule(0, lineY, &ruleDraw{width: body.Width, thickness: thickness, paint: env.Paint}); err != nil {
		return nil, err
	}
	return withClass(fragment, node.Class), nil
}
