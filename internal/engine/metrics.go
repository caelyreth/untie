package engine

import (
	"fmt"

	"github.com/caelyreth/untie/font"
	"github.com/caelyreth/untie/geometry"
	"github.com/caelyreth/untie/internal/layout"
)

func (a *adapter) extractMathMetrics() (layout.MathMetrics, error) {
	info, ok := a.fonts.Face(font.MathFace)
	if !ok {
		return layout.MathMetrics{}, fmt.Errorf("%w: missing math face", ErrInvalidInput)
	}
	metric := func(c font.MathConstant) (geometry.Em, error) {
		value, ok := a.fonts.MathConstant(font.MathFace, c)
		if !ok {
			return 0, nil
		}
		em, err := a.emFromUnits(int64(value), info.UnitsPerEm)
		if err != nil {
			return 0, fmt.Errorf("MATH constant %s: %w", c, err)
		}
		return em, nil
	}
	axis, err := metric(font.AxisHeight)
	if err != nil {
		return layout.MathMetrics{}, err
	}
	percent := func(c font.MathConstant, fallback int16) int16 {
		value, ok := a.fonts.MathConstant(font.MathFace, c)
		if !ok || value < 0 || value > 100 {
			return fallback
		}
		return int16(value)
	}
	spaceWidth := geometry.Em(geometry.EmScale / 3)
	if glyph, ok := a.fonts.MapRune(font.MathFace, ' '); ok {
		if value, ok := a.fonts.GlyphAdvance(glyph); ok {
			if em, err := a.emFromUnits(int64(value), info.UnitsPerEm); err == nil {
				spaceWidth = em
			}
		}
	}
	var xHeight geometry.Em
	if value, ok := a.fonts.VerticalMetrics(font.MathFace); ok {
		xHeight, err = a.emFromUnits(int64(value.XHeight), info.UnitsPerEm)
		if err != nil {
			return layout.MathMetrics{}, err
		}
	}

	values := make(map[font.MathConstant]geometry.Em)
	constants := []font.MathConstant{
		font.SuperscriptShiftUp,
		font.SuperscriptShiftUpCramped,
		font.SuperscriptBottomMin,
		font.SuperscriptBaselineDropMax,
		font.SubscriptShiftDown,
		font.SubscriptTopMax,
		font.SubscriptBaselineDropMin,
		font.SubSuperscriptGapMin,
		font.SuperscriptBottomMaxWithSubscript,
		font.SpaceAfterScript,
		font.UpperLimitGapMin,
		font.UpperLimitBaselineRiseMin,
		font.LowerLimitGapMin,
		font.LowerLimitBaselineDropMin,
		font.StretchStackTopShiftUp,
		font.StretchStackBottomShiftDown,
		font.StretchStackGapAboveMin,
		font.StretchStackGapBelowMin,
		font.StackTopShiftUp,
		font.StackTopDisplayStyleShiftUp,
		font.StackBottomShiftDown,
		font.StackBottomDisplayStyleShiftDown,
		font.StackGapMin,
		font.StackDisplayStyleGapMin,
		font.FractionNumeratorShiftUp,
		font.FractionNumeratorDisplayStyleShiftUp,
		font.FractionDenominatorShiftDown,
		font.FractionDenominatorDisplayStyleShiftDown,
		font.FractionNumeratorGapMin,
		font.FractionNumDisplayStyleGapMin,
		font.FractionRuleThickness,
		font.FractionDenominatorGapMin,
		font.FractionDenomDisplayStyleGapMin,
		font.SkewedFractionVerticalGap,
		font.SkewedFractionHorizontalGap,
		font.OverbarVerticalGap,
		font.OverbarRuleThickness,
		font.OverbarExtraAscender,
		font.UnderbarVerticalGap,
		font.UnderbarRuleThickness,
		font.UnderbarExtraDescender,
		font.RadicalVerticalGap,
		font.RadicalDisplayStyleVerticalGap,
		font.RadicalRuleThickness,
		font.RadicalExtraAscender,
		font.RadicalKernBeforeDegree,
		font.RadicalKernAfterDegree,
		font.AccentBaseHeight,
		font.FlattenedAccentBaseHeight,
	}
	for _, constant := range constants {
		value, err := metric(constant)
		if err != nil {
			return layout.MathMetrics{}, err
		}
		values[constant] = value
	}

	return layout.MathMetrics{
		ScriptPercentScaleDown:        percent(font.ScriptPercentScaleDown, 70),
		ScriptScriptPercentScaleDown:  percent(font.ScriptScriptPercentScaleDown, 50),
		SpaceWidth:                    spaceWidth,
		XHeight:                       xHeight,
		AccentBaseHeight:              values[font.AccentBaseHeight],
		FlattenedAccentBaseHeight:     values[font.FlattenedAccentBaseHeight],
		AxisHeight:                    axis,
		SuperscriptShiftUp:            values[font.SuperscriptShiftUp],
		SuperscriptShiftUpCramped:     values[font.SuperscriptShiftUpCramped],
		SuperscriptBottomMin:          values[font.SuperscriptBottomMin],
		SuperscriptBaselineDropMax:    values[font.SuperscriptBaselineDropMax],
		SubscriptShiftDown:            values[font.SubscriptShiftDown],
		SubscriptTopMax:               values[font.SubscriptTopMax],
		SubscriptBaselineDropMin:      values[font.SubscriptBaselineDropMin],
		SubSuperscriptGapMin:          values[font.SubSuperscriptGapMin],
		SuperscriptBottomMaxWithSub:   values[font.SuperscriptBottomMaxWithSubscript],
		SpaceAfterScript:              values[font.SpaceAfterScript],
		UpperLimitGapMin:              values[font.UpperLimitGapMin],
		UpperLimitBaselineRiseMin:     values[font.UpperLimitBaselineRiseMin],
		LowerLimitGapMin:              values[font.LowerLimitGapMin],
		LowerLimitBaselineDropMin:     values[font.LowerLimitBaselineDropMin],
		StretchStackTopShiftUp:        values[font.StretchStackTopShiftUp],
		StretchStackBottomShiftDown:   values[font.StretchStackBottomShiftDown],
		StretchStackGapAboveMin:       values[font.StretchStackGapAboveMin],
		StretchStackGapBelowMin:       values[font.StretchStackGapBelowMin],
		StackTopShiftUp:               values[font.StackTopShiftUp],
		StackTopDisplayStyleShiftUp:   values[font.StackTopDisplayStyleShiftUp],
		StackBottomShiftDown:          values[font.StackBottomShiftDown],
		StackBottomDisplayShiftDown:   values[font.StackBottomDisplayStyleShiftDown],
		StackGapMin:                   values[font.StackGapMin],
		StackDisplayStyleGapMin:       values[font.StackDisplayStyleGapMin],
		FractionNumeratorShiftUp:      values[font.FractionNumeratorShiftUp],
		FractionNumeratorDisplayUp:    values[font.FractionNumeratorDisplayStyleShiftUp],
		FractionDenominatorShiftDown:  values[font.FractionDenominatorShiftDown],
		FractionDenominatorDisplayDn:  values[font.FractionDenominatorDisplayStyleShiftDown],
		FractionNumeratorGapMin:       values[font.FractionNumeratorGapMin],
		FractionNumeratorDisplayGap:   values[font.FractionNumDisplayStyleGapMin],
		FractionRuleThickness:         values[font.FractionRuleThickness],
		FractionDenominatorGapMin:     values[font.FractionDenominatorGapMin],
		FractionDenominatorDisplayGap: values[font.FractionDenomDisplayStyleGapMin],
		SkewedFractionVerticalGap:     values[font.SkewedFractionVerticalGap],
		SkewedFractionHorizontalGap:   values[font.SkewedFractionHorizontalGap],
		OverbarVerticalGap:            values[font.OverbarVerticalGap],
		OverbarRuleThickness:          values[font.OverbarRuleThickness],
		OverbarExtraAscender:          values[font.OverbarExtraAscender],
		UnderbarVerticalGap:           values[font.UnderbarVerticalGap],
		UnderbarRuleThickness:         values[font.UnderbarRuleThickness],
		UnderbarExtraDescender:        values[font.UnderbarExtraDescender],
		RadicalVerticalGap:            values[font.RadicalVerticalGap],
		RadicalDisplayStyleGap:        values[font.RadicalDisplayStyleVerticalGap],
		RadicalRuleThickness:          values[font.RadicalRuleThickness],
		RadicalExtraAscender:          values[font.RadicalExtraAscender],
		RadicalKernBeforeDegree:       values[font.RadicalKernBeforeDegree],
		RadicalKernAfterDegree:        values[font.RadicalKernAfterDegree],
		RadicalDegreeBottomRaisePct:   percent(font.RadicalDegreeBottomRaisePercent, 60),
	}, nil
}
