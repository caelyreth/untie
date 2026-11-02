package layout

import (
	"github.com/caelyreth/untie/geometry"
	"github.com/caelyreth/untie/ir"
)

func (s *layoutState) layoutRadical(node *ir.Radical, env environment, depth int) (*Fragment, error) {
	radEnv := env
	radEnv.Style = env.Style.WithCramped(true)
	radicand, err := s.layoutNode(node.Radicand, radEnv, depth)
	if err != nil {
		return nil, err
	}
	indexEnv := env
	indexEnv.Style = env.Style.WithSize(ScriptScript).WithCramped(true)
	var index *Fragment
	if node.Index != nil {
		index, err = s.layoutNode(node.Index, indexEnv, depth)
		if err != nil {
			return nil, err
		}
	}

	metrics := s.fontMetrics(env.Style, env.Variant)
	thickness, err := s.metricAt(metrics.RadicalRuleThickness, env.Style)
	if err != nil {
		return nil, err
	}
	gapMetric := metrics.RadicalVerticalGap
	if env.Style.Size == Display {
		gapMetric = metrics.RadicalDisplayStyleGap
	}
	gap, err := s.metricAt(gapMetric, env.Style)
	if err != nil {
		return nil, err
	}
	height, err := add(radicand.Ascent, radicand.Descent)
	if err != nil {
		return nil, err
	}
	target, err := sum(height, thickness, gap)
	if err != nil {
		return nil, err
	}
	sign, err := s.layoutGlyphText("√", env, &stretchSpec{axis: Vertical, target: target})
	if err != nil {
		return nil, err
	}
	extraAscender, err := s.metricAt(metrics.RadicalExtraAscender, env.Style)
	if err != nil {
		return nil, err
	}
	kernBefore, err := s.metricAt(metrics.RadicalKernBeforeDegree, env.Style)
	if err != nil {
		return nil, err
	}
	kernAfter, err := s.metricAt(metrics.RadicalKernAfterDegree, env.Style)
	if err != nil {
		return nil, err
	}
	signHeight, err := add(sign.Ascent, sign.Descent)
	if err != nil {
		return nil, err
	}
	freeGap, err := sub(signHeight, thickness, height)
	if err != nil {
		return nil, err
	}
	freeGap, err = add(freeGap, gap)
	if err != nil {
		return nil, err
	}
	gap = max(gap, freeGap/2)
	signAscent, err := sum(radicand.Ascent, gap, thickness)
	if err != nil {
		return nil, err
	}
	signDescent, err := sub(signHeight, signAscent)
	if err != nil {
		return nil, err
	}
	innerAscent, err := add(signAscent, extraAscender)
	if err != nil {
		return nil, err
	}
	ascent, descent := innerAscent, signDescent
	var sqrtOffset, indexShiftX, indexShiftUp geometry.Em
	if index != nil {
		sqrtOffset, err = sum(kernBefore, index.Width, kernAfter)
		if err != nil {
			return nil, err
		}
		raisePct := int64(metrics.RadicalDegreeBottomRaisePct)
		diff, err := sub(innerAscent, descent)
		if err != nil {
			return nil, err
		}
		raise, err := mulDiv(diff, raisePct, 100)
		if err != nil {
			return nil, err
		}
		indexShiftUp, err = add(raise, index.Descent)
		if err != nil {
			return nil, err
		}
		indexTop, err := add(indexShiftUp, index.Ascent)
		if err != nil {
			return nil, err
		}
		ascent = max(ascent, indexTop)
		if sqrtOffset < 0 {
			indexShiftX, err = sub(0, sqrtOffset)
			if err != nil {
				return nil, err
			}
		}
	}
	signX := max(sqrtOffset, 0)
	radicandX, err := add(signX, sign.Width)
	if err != nil {
		return nil, err
	}
	radicandTop, err := sub(ascent, radicand.Ascent)
	if err != nil {
		return nil, err
	}
	signTop, err := sub(radicandTop, gap, thickness)
	if err != nil {
		return nil, err
	}
	width, err := add(radicandX, radicand.Width)
	if err != nil {
		return nil, err
	}
	fragment, err := newFragment(Metrics{Width: width, Ascent: ascent, Descent: descent}, ir.ClassNormal, env.Style.Size, s.scale(env.Style))
	if err != nil {
		return nil, err
	}
	fragment.BaseAscent = ascent
	fragment.BaseDescent = descent
	fragment.TextLike = false
	if index != nil {
		indexTop, err := sub(ascent, index.Ascent, indexShiftUp)
		if err != nil {
			return nil, err
		}
		indexY, err := baselineAtTop(indexTop, index.Ascent, ascent)
		if err != nil {
			return nil, err
		}
		if err := fragment.pushGroup(indexShiftX+kernBefore, indexY, index); err != nil {
			return nil, err
		}
	}
	signY, err := baselineAtTop(signTop, sign.Ascent, ascent)
	if err != nil {
		return nil, err
	}
	lineY, err := sub(signTop, ascent)
	if err != nil {
		return nil, err
	}
	radicandY, err := baselineAtTop(radicandTop, radicand.Ascent, ascent)
	if err != nil {
		return nil, err
	}
	if err := fragment.pushGroup(signX, signY, sign); err != nil {
		return nil, err
	}
	if node.Radicand != nil {
		if err := fragment.pushRule(radicandX, lineY, &ruleDraw{
			width:     radicand.Width,
			thickness: thickness,
			paint:     env.Paint,
		}); err != nil {
			return nil, err
		}
	}
	if err := fragment.pushGroup(radicandX, radicandY, radicand); err != nil {
		return nil, err
	}
	return withClass(fragment, node.Class), nil
}
