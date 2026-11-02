package layout

import (
	"github.com/caelyreth/untie/geometry"
	"github.com/caelyreth/untie/ir"
)

// MathMetrics contains the font constants used by this layout layer. All
// values are base ems, independent of the math style. Layout applies the
// style scale exactly once; adapters must not pre-scale values by style.
//
// This is intentionally a data port, not a font representation. A FontSet
// adapter extracts these constants from the OpenType MATH table once.
type MathMetrics struct {
	ScriptPercentScaleDown       int16
	ScriptScriptPercentScaleDown int16

	SpaceWidth geometry.Em
	XHeight    geometry.Em
	// CapHeight drives the minimum inline equation ascent.
	CapHeight geometry.Em
	// AccentBaseHeight and FlattenedAccentBaseHeight come from the font's
	// typographic metrics. Zero uses the New Computer Modern-compatible
	// fallbacks in normalized.
	AccentBaseHeight          geometry.Em
	FlattenedAccentBaseHeight geometry.Em
	// DisplayOperatorMinHeight is the minimum vertical extent for large
	// operators in display style.
	DisplayOperatorMinHeight geometry.Em

	AxisHeight geometry.Em

	SuperscriptShiftUp            geometry.Em
	SuperscriptShiftUpCramped     geometry.Em
	SuperscriptBottomMin          geometry.Em
	SuperscriptBaselineDropMax    geometry.Em
	SubscriptShiftDown            geometry.Em
	SubscriptTopMax               geometry.Em
	SubscriptBaselineDropMin      geometry.Em
	SubSuperscriptGapMin          geometry.Em
	SuperscriptBottomMaxWithSub   geometry.Em
	SpaceAfterScript              geometry.Em
	UpperLimitGapMin              geometry.Em
	UpperLimitBaselineRiseMin     geometry.Em
	LowerLimitGapMin              geometry.Em
	LowerLimitBaselineDropMin     geometry.Em
	StretchStackTopShiftUp        geometry.Em
	StretchStackBottomShiftDown   geometry.Em
	StretchStackGapAboveMin       geometry.Em
	StretchStackGapBelowMin       geometry.Em
	StackTopShiftUp               geometry.Em
	StackTopDisplayStyleShiftUp   geometry.Em
	StackBottomShiftDown          geometry.Em
	StackBottomDisplayShiftDown   geometry.Em
	StackGapMin                   geometry.Em
	StackDisplayStyleGapMin       geometry.Em
	FractionNumeratorShiftUp      geometry.Em
	FractionNumeratorDisplayUp    geometry.Em
	FractionDenominatorShiftDown  geometry.Em
	FractionDenominatorDisplayDn  geometry.Em
	FractionNumeratorGapMin       geometry.Em
	FractionNumeratorDisplayGap   geometry.Em
	FractionRuleThickness         geometry.Em
	FractionDenominatorGapMin     geometry.Em
	FractionDenominatorDisplayGap geometry.Em
	SkewedFractionVerticalGap     geometry.Em
	SkewedFractionHorizontalGap   geometry.Em
	OverbarVerticalGap            geometry.Em
	OverbarRuleThickness          geometry.Em
	OverbarExtraAscender          geometry.Em
	UnderbarVerticalGap           geometry.Em
	UnderbarRuleThickness         geometry.Em
	UnderbarExtraDescender        geometry.Em
	RadicalVerticalGap            geometry.Em
	RadicalDisplayStyleGap        geometry.Em
	RadicalRuleThickness          geometry.Em
	RadicalExtraAscender          geometry.Em
	RadicalKernBeforeDegree       geometry.Em
	RadicalKernAfterDegree        geometry.Em
	RadicalDegreeBottomRaisePct   int16
}

func (m MathMetrics) normalized() MathMetrics {
	if m.ScriptPercentScaleDown <= 0 || m.ScriptPercentScaleDown > 100 {
		m.ScriptPercentScaleDown = 70
	}
	if m.ScriptScriptPercentScaleDown <= 0 || m.ScriptScriptPercentScaleDown > 100 {
		m.ScriptScriptPercentScaleDown = 50
	}
	if m.RadicalDegreeBottomRaisePct < 0 || m.RadicalDegreeBottomRaisePct > 100 {
		m.RadicalDegreeBottomRaisePct = 60
	}
	if m.AccentBaseHeight <= 0 {
		m.AccentBaseHeight = 29_491 // 0.45 em
	}
	if m.FlattenedAccentBaseHeight <= 0 {
		m.FlattenedAccentBaseHeight = 43_516 // 0.664 em
	}
	if m.CapHeight <= 0 {
		m.CapHeight = 44_761 // 0.683 em
	}
	return m
}

func (s *layoutState) scale(style Style) geometry.Em {
	switch style.Size {
	case Display, Text:
		return geometry.Em(geometry.EmScale)
	case Script:
		return geometry.Em(s.metrics.ScriptPercentScaleDown) * geometry.Em(geometry.EmScale) / 100
	case ScriptScript:
		return geometry.Em(s.metrics.ScriptScriptPercentScaleDown) * geometry.Em(geometry.EmScale) / 100
	default:
		return 0
	}
}

func (s *layoutState) metricAt(value geometry.Em, style Style) (geometry.Em, error) {
	return mulDiv(value, int64(s.scale(style)), int64(geometry.EmScale))
}

// StyleFont supplies metrics for a math style and alphabet variant.
type StyleFont interface {
	MathMetrics(style Style, variant ir.Variant) MathMetrics
}
