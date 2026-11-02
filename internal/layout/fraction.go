package layout

import (
	"fmt"

	"github.com/caelyreth/untie/geometry"
	"github.com/caelyreth/untie/ir"
)

const defaultFractionPadding geometry.Em = 6_554 // 0.1 em

func (s *layoutState) layoutFraction(node *ir.Fraction, env environment, depth int) (*Fragment, error) {
	numEnv := env
	numEnv.Style = env.Style.Numerator()
	denEnv := env
	denEnv.Style = env.Style.Denominator()

	numerator, err := s.layoutNode(node.Numerator, numEnv, depth)
	if err != nil {
		return nil, err
	}
	denominator, err := s.layoutNode(node.Denominator, denEnv, depth)
	if err != nil {
		return nil, err
	}

	switch node.Style {
	case ir.FractionVertical, ir.FractionNoBar:
		return s.layoutVerticalFraction(node, env, numerator, denominator)
	case ir.FractionHorizontal, ir.FractionSkewed:
		return s.layoutSlashFraction(node, env, numerator, denominator)
	default:
		return nil, fmt.Errorf("%w: fraction style %d", ErrInvalidInput, node.Style)
	}
}

func (s *layoutState) layoutVerticalFraction(node *ir.Fraction, env environment, numerator, denominator *Fragment) (*Fragment, error) {
	metrics := s.fontMetrics(env.Style, env.Variant)
	var paddingLocal geometry.Em
	if !node.Padding.IsZero() {
		var err error
		paddingLocal, err = s.length(node.Padding, env.Style, metrics)
		if err != nil {
			return nil, err
		}
	} else {
		var err error
		paddingLocal, err = s.metricAt(defaultFractionPadding, env.Style)
		if err != nil {
			return nil, err
		}
	}
	axis, err := s.metricAt(metrics.AxisHeight, env.Style)
	if err != nil {
		return nil, err
	}
	thickness, err := s.metricAt(metrics.FractionRuleThickness, env.Style)
	if err != nil {
		return nil, err
	}

	lineWidth := max(numerator.Width, denominator.Width)
	width, err := sum(lineWidth, paddingLocal, paddingLocal)
	if err != nil {
		return nil, err
	}
	var baseline, lineTop, numTop, denTop, ascent, descent geometry.Em
	showBar := node.Bar && node.Style != ir.FractionNoBar
	if showBar {
		shiftUp, shiftDown, numGapMin, denGapMin, err := s.fractionMetrics(env.Style, metrics)
		if err != nil {
			return nil, err
		}
		axisHalf, err := add(axis, thickness/2)
		if err != nil {
			return nil, err
		}
		numBase, err := sum(axisHalf, numerator.Descent)
		if err != nil {
			return nil, err
		}
		numGap, err := sub(shiftUp, numBase)
		if err != nil {
			return nil, err
		}
		numGap = max(numGap, numGapMin)
		denBase, err := sum(shiftDown, axis)
		if err != nil {
			return nil, err
		}
		denBase, err = sub(denBase, thickness/2, denominator.Ascent)
		if err != nil {
			return nil, err
		}
		denGap := max(denBase, denGapMin)
		height, err := sum(numerator.Ascent, numerator.Descent, numGap, thickness, denGap, denominator.Ascent, denominator.Descent)
		if err != nil {
			return nil, err
		}
		lineTop, err = sum(numerator.Ascent, numerator.Descent, numGap)
		if err != nil {
			return nil, err
		}
		baseline, err = sum(lineTop, thickness/2, axis)
		if err != nil {
			return nil, err
		}
		ascent = baseline
		descent, err = sub(height, baseline)
		if err != nil {
			return nil, err
		}
		numTop = 0
		denTop, err = sub(height, denominator.Ascent, denominator.Descent)
		if err != nil {
			return nil, err
		}
	} else {
		shiftUp, shiftDown, gapMin, err := s.stackMetrics(env.Style, metrics)
		if err != nil {
			return nil, err
		}
		upGap, err := sub(shiftUp, numerator.Descent)
		if err != nil {
			return nil, err
		}
		downGap, err := sub(shiftDown, denominator.Ascent)
		if err != nil {
			return nil, err
		}
		gap, err := sum(upGap, downGap)
		if err != nil {
			return nil, err
		}
		gap = max(gap, gapMin)
		height, err := sum(numerator.Ascent, numerator.Descent, gap, denominator.Ascent, denominator.Descent)
		if err != nil {
			return nil, err
		}
		extra, err := sub(gapMin, gap)
		if err != nil {
			return nil, err
		}
		baseline, err = sum(numerator.Ascent, shiftUp, max(extra, 0)/2)
		if err != nil {
			return nil, err
		}
		ascent = baseline
		descent, err = sub(height, baseline)
		if err != nil {
			return nil, err
		}
		numTop = 0
		denTop, err = sub(height, denominator.Ascent, denominator.Descent)
		if err != nil {
			return nil, err
		}
	}

	fragment, err := newFragment(Metrics{Width: width, Ascent: ascent, Descent: descent}, ir.ClassNormal, env.Style.Size, s.scale(env.Style))
	if err != nil {
		return nil, err
	}
	fragment.BaseAscent = ascent
	fragment.BaseDescent = descent
	fragment.Italic = numerator.Italic
	fragment.TextLike = false

	numX := (width - numerator.Width) / 2
	denX := (width - denominator.Width) / 2
	numY, err := sub(numTop, baseline)
	if err != nil {
		return nil, err
	}
	denY, err := sub(denTop, baseline)
	if err != nil {
		return nil, err
	}
	lineY, err := sub(lineTop, baseline)
	if err != nil {
		return nil, err
	}
	if err := fragment.pushGroup(numX, numY, numerator); err != nil {
		return nil, err
	}
	if err := fragment.pushGroup(denX, denY, denominator); err != nil {
		return nil, err
	}
	if showBar {
		if err := fragment.pushRule((width-lineWidth)/2, lineY, &ruleDraw{
			width:     lineWidth,
			thickness: thickness,
			paint:     env.Paint,
		}); err != nil {
			return nil, err
		}
	}
	return withClass(fragment, node.Class), nil
}

func (s *layoutState) fractionMetrics(style Style, metrics MathMetrics) (geometry.Em, geometry.Em, geometry.Em, geometry.Em, error) {
	display := style.Size == Display
	shiftUp := metrics.FractionNumeratorShiftUp
	shiftDown := metrics.FractionDenominatorShiftDown
	numGap := metrics.FractionNumeratorGapMin
	denGap := metrics.FractionDenominatorGapMin
	if display {
		shiftUp = metrics.FractionNumeratorDisplayUp
		shiftDown = metrics.FractionDenominatorDisplayDn
		numGap = metrics.FractionNumeratorDisplayGap
		denGap = metrics.FractionDenominatorDisplayGap
	}
	up, err := s.metricAt(shiftUp, style)
	if err != nil {
		return 0, 0, 0, 0, err
	}
	down, err := s.metricAt(shiftDown, style)
	if err != nil {
		return 0, 0, 0, 0, err
	}
	ng, err := s.metricAt(numGap, style)
	if err != nil {
		return 0, 0, 0, 0, err
	}
	dg, err := s.metricAt(denGap, style)
	return up, down, ng, dg, err
}

func (s *layoutState) stackMetrics(style Style, metrics MathMetrics) (geometry.Em, geometry.Em, geometry.Em, error) {
	display := style.Size == Display
	up, down, gap := metrics.StackTopShiftUp, metrics.StackBottomShiftDown, metrics.StackGapMin
	if display {
		up, down, gap = metrics.StackTopDisplayStyleShiftUp, metrics.StackBottomDisplayShiftDown, metrics.StackDisplayStyleGapMin
	}
	u, err := s.metricAt(up, style)
	if err != nil {
		return 0, 0, 0, err
	}
	d, err := s.metricAt(down, style)
	if err != nil {
		return 0, 0, 0, err
	}
	g, err := s.metricAt(gap, style)
	return u, d, g, err
}

func (s *layoutState) layoutSlashFraction(node *ir.Fraction, env environment, numerator, denominator *Fragment) (*Fragment, error) {
	slashEnv := env
	slashEnv.Style = env.Style
	slashText := "⁄"
	slash, err := s.layoutGlyphText(slashText, slashEnv, nil)
	if err != nil {
		slash, err = s.layoutGlyphText("/", slashEnv, nil)
		if err != nil {
			return nil, err
		}
	}
	parts := []*Fragment{numerator, slash, denominator}
	width, err := sum(numerator.Width, slash.Width, denominator.Width)
	if err != nil {
		return nil, err
	}
	ascent := max(max(numerator.Ascent, slash.Ascent), denominator.Ascent)
	descent := max(max(numerator.Descent, slash.Descent), denominator.Descent)
	fragment, err := newFragment(Metrics{Width: width, Ascent: ascent, Descent: descent}, ir.ClassNormal, env.Style.Size, s.scale(env.Style))
	if err != nil {
		return nil, err
	}
	x := geometry.Em(0)
	for _, part := range parts {
		if err := fragment.pushGroup(x, 0, part); err != nil {
			return nil, err
		}
		x, err = add(x, part.Width)
		if err != nil {
			return nil, err
		}
	}
	fragment.TextLike = false
	return withClass(fragment, node.Class), nil
}
