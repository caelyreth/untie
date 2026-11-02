package engine

import (
	"math"
	"strings"
	"unicode"

	"github.com/caelyreth/untie/font"
	"github.com/caelyreth/untie/geometry"
	"github.com/caelyreth/untie/internal/layout"
	"github.com/caelyreth/untie/internal/mathdata"
	"github.com/caelyreth/untie/ir"
)

type adapter struct {
	fonts *font.FontSet
}

func newAdapter(fonts *font.FontSet) *adapter {
	return &adapter{fonts: fonts}
}

func (a *adapter) MathMetrics(_ layout.Style, _ ir.Variant) layout.MathMetrics {
	info, ok := a.fonts.Face(font.MathFace)
	if !ok {
		return layout.MathMetrics{}
	}
	metric := func(c font.MathConstant) geometry.Em {
		value, ok := a.fonts.MathConstant(font.MathFace, c)
		if !ok {
			return 0
		}
		em, err := geometry.FontUnitsToEm(int64(value), int64(info.UnitsPerEm))
		if err != nil {
			return 0
		}
		return em
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
			if em, err := geometry.FontUnitsToEm(int64(value), int64(info.UnitsPerEm)); err == nil {
				spaceWidth = em
			}
		}
	}
	metrics, ok := a.fonts.VerticalMetrics(font.MathFace)
	var xHeight geometry.Em
	if ok {
		xHeight, _ = geometry.FontUnitsToEm(int64(metrics.XHeight), int64(info.UnitsPerEm))
	}
	return layout.MathMetrics{
		ScriptPercentScaleDown:        percent(font.ScriptPercentScaleDown, 70),
		ScriptScriptPercentScaleDown:  percent(font.ScriptScriptPercentScaleDown, 50),
		SpaceWidth:                    spaceWidth,
		XHeight:                       xHeight,
		AxisHeight:                    metric(font.AxisHeight),
		SuperscriptShiftUp:            metric(font.SuperscriptShiftUp),
		SuperscriptShiftUpCramped:     metric(font.SuperscriptShiftUpCramped),
		SuperscriptBottomMin:          metric(font.SuperscriptBottomMin),
		SuperscriptBaselineDropMax:    metric(font.SuperscriptBaselineDropMax),
		SubscriptShiftDown:            metric(font.SubscriptShiftDown),
		SubscriptTopMax:               metric(font.SubscriptTopMax),
		SubscriptBaselineDropMin:      metric(font.SubscriptBaselineDropMin),
		SubSuperscriptGapMin:          metric(font.SubSuperscriptGapMin),
		SuperscriptBottomMaxWithSub:   metric(font.SuperscriptBottomMaxWithSubscript),
		SpaceAfterScript:              metric(font.SpaceAfterScript),
		UpperLimitGapMin:              metric(font.UpperLimitGapMin),
		UpperLimitBaselineRiseMin:     metric(font.UpperLimitBaselineRiseMin),
		LowerLimitGapMin:              metric(font.LowerLimitGapMin),
		LowerLimitBaselineDropMin:     metric(font.LowerLimitBaselineDropMin),
		StretchStackTopShiftUp:        metric(font.StretchStackTopShiftUp),
		StretchStackBottomShiftDown:   metric(font.StretchStackBottomShiftDown),
		StretchStackGapAboveMin:       metric(font.StretchStackGapAboveMin),
		StretchStackGapBelowMin:       metric(font.StretchStackGapBelowMin),
		StackTopShiftUp:               metric(font.StackTopShiftUp),
		StackTopDisplayStyleShiftUp:   metric(font.StackTopDisplayStyleShiftUp),
		StackBottomShiftDown:          metric(font.StackBottomShiftDown),
		StackBottomDisplayShiftDown:   metric(font.StackBottomDisplayStyleShiftDown),
		StackGapMin:                   metric(font.StackGapMin),
		StackDisplayStyleGapMin:       metric(font.StackDisplayStyleGapMin),
		FractionNumeratorShiftUp:      metric(font.FractionNumeratorShiftUp),
		FractionNumeratorDisplayUp:    metric(font.FractionNumeratorDisplayStyleShiftUp),
		FractionDenominatorShiftDown:  metric(font.FractionDenominatorShiftDown),
		FractionDenominatorDisplayDn:  metric(font.FractionDenominatorDisplayStyleShiftDown),
		FractionNumeratorGapMin:       metric(font.FractionNumeratorGapMin),
		FractionNumeratorDisplayGap:   metric(font.FractionNumDisplayStyleGapMin),
		FractionRuleThickness:         metric(font.FractionRuleThickness),
		FractionDenominatorGapMin:     metric(font.FractionDenominatorGapMin),
		FractionDenominatorDisplayGap: metric(font.FractionDenomDisplayStyleGapMin),
		SkewedFractionVerticalGap:     metric(font.SkewedFractionVerticalGap),
		SkewedFractionHorizontalGap:   metric(font.SkewedFractionHorizontalGap),
		OverbarVerticalGap:            metric(font.OverbarVerticalGap),
		OverbarRuleThickness:          metric(font.OverbarRuleThickness),
		OverbarExtraAscender:          metric(font.OverbarExtraAscender),
		UnderbarVerticalGap:           metric(font.UnderbarVerticalGap),
		UnderbarRuleThickness:         metric(font.UnderbarRuleThickness),
		UnderbarExtraDescender:        metric(font.UnderbarExtraDescender),
		RadicalVerticalGap:            metric(font.RadicalVerticalGap),
		RadicalDisplayStyleGap:        metric(font.RadicalDisplayStyleVerticalGap),
		RadicalRuleThickness:          metric(font.RadicalRuleThickness),
		RadicalExtraAscender:          metric(font.RadicalExtraAscender),
		RadicalKernBeforeDegree:       metric(font.RadicalKernBeforeDegree),
		RadicalKernAfterDegree:        metric(font.RadicalKernAfterDegree),
		RadicalDegreeBottomRaisePct:   percent(font.RadicalDegreeBottomRaisePercent, 60),
	}
}

func (a *adapter) Glyph(req layout.GlyphRequest) (layout.Glyph, bool) {
	return a.glyph(req, nil)
}

func (a *adapter) Stretch(req layout.StretchRequest) (layout.Glyph, bool) {
	return a.glyph(req.GlyphRequest, &req)
}

func (a *adapter) ShapeText(req layout.TextRequest) (layout.TextRun, bool) {
	face := a.faceFor(ir.FontText)
	if _, ok := a.fonts.Face(face); !ok {
		return layout.TextRun{}, false
	}
	run := layout.TextRun{}
	var pen geometry.Em
	for _, r := range req.Text {
		mapped, ok := a.fonts.MapRune(face, r)
		if !ok {
			return layout.TextRun{}, false
		}
		glyph, ok := a.glyphFromFont(mapped, req.Style, false, "")
		if !ok {
			return layout.TextRun{}, false
		}
		run.Glyphs = append(run.Glyphs, layout.PositionedGlyph{
			Glyph: glyph,
			At:    layout.Point{X: pen, Y: 0},
		})
		var err error
		pen, err = pen.Add(glyph.Advance)
		if err != nil {
			return layout.TextRun{}, false
		}
		run.Ascent = max(run.Ascent, glyph.Ascent)
		run.Descent = max(run.Descent, glyph.Descent)
		run.Italic = glyph.Italic
	}
	run.Width = pen
	return run, true
}

func (a *adapter) glyph(req layout.GlyphRequest, stretch *layout.StretchRequest) (layout.Glyph, bool) {
	face := a.faceFor(req.Font)
	text := req.Text
	if req.Font == ir.FontMath {
		text = styledMathText(text, req.Variant)
	}

	mapped, ok := a.mapText(face, text)
	if !ok && face != font.MathFace {
		mapped, ok = a.mapText(font.MathFace, text)
	}
	if !ok {
		for fallback, _ := font.FallbackFace(0); ; fallback++ {
			if _, exists := a.fonts.Face(fallback); !exists {
				break
			}
			if mapped, ok = a.mapText(fallback, text); ok {
				face = fallback
				break
			}
		}
	}
	if !ok {
		return layout.Glyph{}, false
	}

	scriptStyle := font.NoScriptVariant
	switch req.Style.Size {
	case layout.Script:
		scriptStyle = font.ScriptVariant
	case layout.ScriptScript:
		scriptStyle = font.ScriptScriptVariant
	}
	flatten := containsFeature(req.Features, "flac")
	if shaped, ok := a.fonts.ShapeMath(face, []font.GlyphID{mapped.ID}, scriptStyle, flatten); ok && len(shaped) == 1 {
		mapped.ID = shaped[0]
	}

	if stretch != nil {
		if result, ok := a.stretchGlyph(mapped, *stretch, req.Text); ok {
			return result, true
		}
	}
	return a.glyphFromFont(mapped, req.Style, true, req.Text)
}

func (a *adapter) stretchGlyph(base font.Glyph, req layout.StretchRequest, text string) (layout.Glyph, bool) {
	info, ok := a.fonts.Face(base.Face)
	if !ok {
		return layout.Glyph{}, false
	}
	axis := font.VerticalAxis
	if req.Axis == layout.Horizontal {
		axis = font.HorizontalAxis
	}
	construction, ok := a.fonts.MathConstruction(base, axis)
	if !ok {
		return layout.Glyph{}, false
	}
	target := a.toDesignUnits(req.Target, info.UnitsPerEm)
	var best *font.SizeVariant
	for i := range construction.Variants {
		variant := &construction.Variants[i]
		if variant.Advance >= target {
			if best == nil || variant.Advance < best.Advance {
				best = variant
			}
		} else if best == nil || variant.Advance > best.Advance {
			best = variant
		}
	}
	glyph := base
	if best != nil {
		if best.Advance >= target || construction.Assembly == nil {
			glyph.ID = best.Glyph
		}
	}
	if best == nil || (best.Advance < target && construction.Assembly != nil) {
		if assembly := a.assemble(base, construction, target, axis); assembly != nil {
			return *assembly, true
		}
	}
	if glyph.ID == base.ID && stretchShortfall(base, target, axis, a.fonts) > 0 {
		return layout.Glyph{}, false
	}
	return a.glyphFromFont(glyph, req.Style, true, text)
}

func (a *adapter) assemble(base font.Glyph, construction font.MathConstruction, target int32, axis font.MathAxis) *layout.Glyph {
	if construction.Assembly == nil || len(construction.Assembly.Parts) == 0 {
		return nil
	}
	parts := construction.Assembly.Parts
	var fixed int32
	var extenders []font.AssemblyPart
	for _, part := range parts {
		if part.Extender {
			extenders = append(extenders, part)
		} else {
			fixed += part.FullAdvance
		}
	}
	if len(extenders) == 0 {
		return nil
	}
	repeats := int32(1)
	if target > fixed {
		repeats = max32(1, ceilDiv(target-fixed, extenders[0].FullAdvance))
	}
	sequence := make([]font.AssemblyPart, 0, len(parts)+int(repeats))
	sequence = append(sequence, parts...)
	for i := int32(1); i < repeats; i++ {
		sequence = append(sequence, extenders...)
	}
	info, ok := a.fonts.Face(base.Face)
	if !ok {
		return nil
	}
	var advanced int32
	minX := int32(math.MaxInt32)
	minY := int32(math.MaxInt32)
	maxX := int32(math.MinInt32)
	maxY := int32(math.MinInt32)
	outline := make([]font.Segment, 0)
	for _, part := range sequence {
		partGlyph := font.Glyph{Face: base.Face, ID: part.Glyph}
		data, ok := a.fonts.Outline(partGlyph)
		if !ok {
			return nil
		}
		offset := advanced - part.StartConnector
		if axis == font.HorizontalAxis {
			outline = append(outline, translateSegments(data.Segments, offset, 0)...)
		} else {
			outline = append(outline, translateSegments(data.Segments, 0, offset)...)
		}
		for _, segment := range data.Segments {
			for _, point := range segment.Points {
				x := point.X
				y := point.Y
				if axis == font.HorizontalAxis {
					x += offset
				} else {
					y += offset
				}
				minX = min32(minX, x)
				minY = min32(minY, y)
				maxX = max32(maxX, x)
				maxY = max32(maxY, y)
			}
		}
		advanced += part.FullAdvance - part.EndConnector
	}
	// Reuse glyph assembly as a synthetic outline-only glyph. The selected ID is
	// the base for identity; the drawable geometry is the assembled outline.
	glyph, ok := a.glyphFromFont(base, layout.Style{}, true, "")
	if !ok {
		return nil
	}
	glyph.Outline = a.designPath(outline, info.UnitsPerEm)
	if axis == font.HorizontalAxis {
		glyph.Advance, _ = geometry.FontUnitsToEm(int64(advanced), int64(info.UnitsPerEm))
		glyph.Ascent, _ = geometry.FontUnitsToEm(int64(max32(0, -minY)), int64(info.UnitsPerEm))
		glyph.Descent, _ = geometry.FontUnitsToEm(int64(max32(0, maxY)), int64(info.UnitsPerEm))
	} else {
		glyph.Ascent, _ = geometry.FontUnitsToEm(int64(max32(0, -minY)), int64(info.UnitsPerEm))
		glyph.Descent, _ = geometry.FontUnitsToEm(int64(max32(0, maxY)), int64(info.UnitsPerEm))
	}
	_ = minX
	_ = maxX
	return &glyph
}

func (a *adapter) glyphFromFont(mapped font.Glyph, style layout.Style, math bool, text string) (layout.Glyph, bool) {
	info, ok := a.fonts.Face(mapped.Face)
	if !ok {
		return layout.Glyph{}, false
	}
	metrics, ok := a.fonts.GlyphMetrics(mapped)
	if !ok {
		return layout.Glyph{}, false
	}
	outline, ok := a.fonts.Outline(mapped)
	if !ok {
		return layout.Glyph{}, false
	}
	advance, err := geometry.FontUnitsToEm(int64(metrics.Advance), int64(info.UnitsPerEm))
	if err != nil {
		return layout.Glyph{}, false
	}
	ascent := int32(0)
	descent := int32(0)
	if bounds := outline.Bounds; !bounds.Empty() {
		if bounds.YMin < 0 {
			ascent = -bounds.YMin
		}
		if bounds.YMax > 0 {
			descent = bounds.YMax
		}
	}
	ascentEm, err := geometry.FontUnitsToEm(int64(ascent), int64(info.UnitsPerEm))
	if err != nil {
		return layout.Glyph{}, false
	}
	descentEm, err := geometry.FontUnitsToEm(int64(descent), int64(info.UnitsPerEm))
	if err != nil {
		return layout.Glyph{}, false
	}
	result := layout.Glyph{
		ID:       uint16(mapped.ID),
		FontID:   info.ID,
		Text:     text,
		Advance:  advance,
		Ascent:   ascentEm,
		Descent:  descentEm,
		TextLike: math && isTextLike(text),
		Outline:  a.designPath(outline.Segments, info.UnitsPerEm),
	}
	if value, ok := a.fonts.ItalicCorrection(mapped); ok {
		result.Italic, _ = geometry.FontUnitsToEm(int64(value), int64(info.UnitsPerEm))
	}
	if value, ok := a.fonts.TopAccentAttachment(mapped); ok {
		result.TopAccent, _ = geometry.FontUnitsToEm(int64(value), int64(info.UnitsPerEm))
		result.HasTopAccent = true
	}
	return result, true
}

func (a *adapter) designPath(segments []font.Segment, upem uint16) layout.Path {
	path := make(layout.Path, len(segments))
	for i, segment := range segments {
		path[i].Op = segment.Op
		for j, point := range segment.Points {
			x, _ := geometry.FontUnitsToEm(int64(point.X), int64(upem))
			y, _ := geometry.FontUnitsToEm(int64(point.Y), int64(upem))
			path[i].Points[j] = layout.Point{X: x, Y: y}
		}
	}
	return path
}

func (a *adapter) faceFor(role ir.FontRole) font.FaceRef {
	if role == ir.FontText {
		if _, ok := a.fonts.Face(font.TextFace); ok {
			return font.TextFace
		}
	}
	return font.MathFace
}

func (a *adapter) mapText(face font.FaceRef, text string) (font.Glyph, bool) {
	runes := []rune(text)
	if len(runes) == 1 {
		return a.fonts.MapRune(face, runes[0])
	}
	if len(runes) == 2 && isVariationSelector(runes[1]) {
		return a.fonts.MapVariation(face, runes[0], runes[1])
	}
	return font.Glyph{}, false
}

func (a *adapter) toDesignUnits(value geometry.Em, upem uint16) int32 {
	scaled := float64(value) * float64(upem) / float64(geometry.EmScale)
	if scaled > math.MaxInt32 {
		return math.MaxInt32
	}
	if scaled < math.MinInt32 {
		return math.MinInt32
	}
	return int32(math.Round(scaled))
}

func styledMathText(text string, variant ir.Variant) string {
	alpha, ok := alphaVariant(variant)
	if !ok {
		if variant != ir.VariantAuto {
			return text
		}
		alpha = mathdata.VariantItalic
	}
	var out strings.Builder
	for _, r := range text {
		styled, selector, ok := mathdata.VariantRune(r, alpha)
		if !ok {
			out.WriteRune(r)
			continue
		}
		out.WriteRune(styled)
		if selector != 0 {
			out.WriteRune(selector)
		}
	}
	return out.String()
}

func alphaVariant(variant ir.Variant) (mathdata.AlphaVariant, bool) {
	switch variant {
	case ir.VariantNormal:
		return mathdata.VariantPlain, true
	case ir.VariantItalic:
		return mathdata.VariantItalic, true
	case ir.VariantBold:
		return mathdata.VariantBold, true
	case ir.VariantBoldItalic:
		return mathdata.VariantBoldItalic, true
	case ir.VariantDoubleStruck:
		return mathdata.VariantDoubleStruck, true
	case ir.VariantScript:
		return mathdata.VariantScript, true
	case ir.VariantBoldScript:
		return mathdata.VariantBoldScript, true
	case ir.VariantFraktur:
		return mathdata.VariantFraktur, true
	case ir.VariantBoldFraktur:
		return mathdata.VariantBoldFraktur, true
	case ir.VariantSans:
		return mathdata.VariantSansSerif, true
	case ir.VariantBoldSans:
		return mathdata.VariantSansSerifBold, true
	case ir.VariantMonospace:
		return mathdata.VariantMonospace, true
	default:
		return 0, false
	}
}

func isTextLike(text string) bool {
	for _, r := range text {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			continue
		}
		return false
	}
	return text != ""
}

func isVariationSelector(r rune) bool {
	return r >= 0xFE00 && r <= 0xFE0F
}

func containsFeature(features []string, tag string) bool {
	for _, feature := range features {
		name := feature
		if index := strings.IndexByte(name, '='); index >= 0 {
			name = name[:index]
		}
		if name == tag {
			return true
		}
	}
	return false
}

func translateSegments(segments []font.Segment, dx, dy int32) []font.Segment {
	out := append([]font.Segment(nil), segments...)
	for i := range out {
		for j := range out[i].Points {
			out[i].Points[j].X += dx
			out[i].Points[j].Y += dy
		}
	}
	return out
}

func stretchShortfall(base font.Glyph, target int32, axis font.MathAxis, fonts *font.FontSet) int32 {
	outline, ok := fonts.Outline(base)
	if !ok {
		return math.MaxInt32
	}
	extent := int32(0)
	if axis == font.HorizontalAxis {
		extent = outline.Bounds.Width()
	} else {
		extent = outline.Bounds.Height()
	}
	if extent >= target {
		return 0
	}
	return target - extent
}

func ceilDiv(a, b int32) int32 {
	if b <= 0 {
		return 1
	}
	return (a + b - 1) / b
}

func min32(a, b int32) int32 {
	if a < b {
		return a
	}
	return b
}

func max32(a, b int32) int32 {
	if a > b {
		return a
	}
	return b
}

func max(a, b geometry.Em) geometry.Em {
	if a > b {
		return a
	}
	return b
}
