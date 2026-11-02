package engine

import (
	"context"
	"fmt"

	"github.com/caelyreth/untie/font"
	"github.com/caelyreth/untie/geometry"
	"github.com/caelyreth/untie/internal/layout"
	"github.com/caelyreth/untie/ir"
)

func (a *adapter) ShapeText(req layout.TextRequest) (layout.TextRun, error) {
	return a.shapeRun(req.Context, req.Text, req.Style, req.Variant, req.Font, req.Features, req.Tracking, false)
}

func (a *adapter) ShapeOperator(req layout.OperatorRequest) (layout.TextRun, error) {
	return a.shapeRun(req.Context, req.Text, req.Style, req.Variant, ir.FontMath, req.Features, 0, true)
}

func (a *adapter) shapeRun(
	ctx context.Context,
	text string,
	style layout.Style,
	variant ir.Variant,
	role ir.FontRole,
	features []string,
	tracking geometry.Em,
	operator bool,
) (layout.TextRun, error) {
	if err := requestContext(ctx); err != nil {
		return layout.TextRun{}, err
	}
	if len(text) > a.limits.MaxTextBytes {
		return layout.TextRun{}, fmt.Errorf("%w: text byte count %d", layout.ErrLimit, len(text))
	}
	requestFeatures, err := textFeatures(features)
	if err != nil {
		return layout.TextRun{}, err
	}
	if operator {
		variant = ir.VariantNormal
		requestFeatures, err = mathFeatures(features, style)
		if err != nil {
			return layout.TextRun{}, err
		}
	}
	faces := a.faceOrder(role)
	runes := []rune(text)
	run := layout.TextRun{}
	var pen geometry.Em
	pathSegments := 0
	for i := 0; i < len(runes); {
		if err := checkEvery(ctx, i, 16); err != nil {
			return layout.TextRun{}, err
		}
		styled, ok := styleRune(runes[i], variant, operator || role == ir.FontMath)
		if !ok {
			return layout.TextRun{}, fmt.Errorf("%w: variant %d for %q", layout.ErrMissingGlyph, variant, string(runes[i]))
		}
		if i+1 < len(runes) && isVariationSelector(runes[i+1]) {
			styledRunes := []rune(styled)
			if len(styledRunes) == 1 || !isVariationSelector(styledRunes[len(styledRunes)-1]) {
				styled += string(runes[i+1])
			}
			i++
		}
		mapped, _, ok := a.mapTextAcrossFaces(faces, styled)
		if !ok {
			return layout.TextRun{}, fmt.Errorf("%w: %q", layout.ErrMissingGlyph, string(runes[i]))
		}
		mapped, ok = a.shapeFeatures(mapped, requestFeatures)
		if !ok {
			return layout.TextRun{}, fmt.Errorf("%w: shaping %q", layout.ErrMissingGlyph, string(runes[i]))
		}
		glyph, err := a.glyphFromFont(mapped, style, role == ir.FontMath, string(runes[i]))
		if err != nil {
			return layout.TextRun{}, err
		}
		if len(run.Glyphs) >= a.limits.MaxOutputItems {
			return layout.TextRun{}, fmt.Errorf("%w: output item count", layout.ErrLimit)
		}
		if err := reserveSegments(&pathSegments, len(glyph.Outline), a.limits.MaxPathSegments); err != nil {
			return layout.TextRun{}, err
		}
		run.Glyphs = append(run.Glyphs, layout.PositionedGlyph{Glyph: glyph, At: layout.Point{X: pen}})
		next, err := pen.Add(glyph.Advance)
		if err != nil {
			return layout.TextRun{}, err
		}
		next, err = next.Add(tracking)
		if err != nil {
			return layout.TextRun{}, err
		}
		pen = next
		run.Ascent = maxEm(run.Ascent, glyph.Ascent)
		run.Descent = maxEm(run.Descent, glyph.Descent)
		run.Italic = glyph.Italic
		i++
	}
	if pen < 0 {
		return layout.TextRun{}, fmt.Errorf("%w: negative text advance", layout.ErrInvalidInput)
	}
	run.Width = pen
	return run, nil
}

func (a *adapter) shapedGlyph(req layout.GlyphRequest) (font.Glyph, string, bool) {
	faces := a.faceOrder(req.Font)
	styled, ok := styledText(req.Text, req.Variant, req.Font == ir.FontMath)
	if !ok {
		return font.Glyph{}, "", false
	}
	mapped, _, ok := a.mapTextAcrossFaces(faces, styled)
	if !ok {
		return font.Glyph{}, "", false
	}
	features, err := mathFeatures(req.Features, req.Style)
	if err != nil {
		return font.Glyph{}, "", false
	}
	mapped, ok = a.shapeFeatures(mapped, features)
	return mapped, req.Text, ok
}

func (a *adapter) shapeFeatures(mapped font.Glyph, features []font.Feature) (font.Glyph, bool) {
	if len(features) == 0 {
		return mapped, true
	}
	ids, ok := a.fonts.Shape(mapped.Face, []font.GlyphID{mapped.ID}, features...)
	if !ok || len(ids) != 1 {
		return font.Glyph{}, false
	}
	mapped.ID = ids[0]
	return mapped, true
}

func (a *adapter) glyphFromFont(mapped font.Glyph, _ layout.Style, mathText bool, text string) (layout.Glyph, error) {
	info, ok := a.fonts.Face(mapped.Face)
	if !ok {
		return layout.Glyph{}, fmt.Errorf("%w: face %d", layout.ErrMissingGlyph, mapped.Face)
	}
	metrics, ok := a.fonts.GlyphMetrics(mapped)
	if !ok {
		return layout.Glyph{}, fmt.Errorf("%w: metrics for %q", layout.ErrMissingGlyph, text)
	}
	outline, ok := a.fonts.Outline(mapped)
	if !ok {
		return layout.Glyph{}, fmt.Errorf("%w: outline for %q", layout.ErrMissingGlyph, text)
	}
	if len(outline.Segments) > a.limits.MaxPathSegments {
		return layout.Glyph{}, fmt.Errorf("%w: path segment count %d", layout.ErrLimit, len(outline.Segments))
	}
	advance, err := a.emFromUnits(int64(metrics.Advance), info.UnitsPerEm)
	if err != nil {
		return layout.Glyph{}, err
	}
	ascent, descent := geometry.Em(0), geometry.Em(0)
	if bounds := outline.Bounds; !bounds.Empty() {
		if bounds.YMin < 0 {
			ascent, err = a.emFromUnits(-int64(bounds.YMin), info.UnitsPerEm)
			if err != nil {
				return layout.Glyph{}, err
			}
		}
		if bounds.YMax > 0 {
			descent, err = a.emFromUnits(int64(bounds.YMax), info.UnitsPerEm)
			if err != nil {
				return layout.Glyph{}, err
			}
		}
	}
	var italic geometry.Em
	if mathText {
		if value, ok := a.fonts.ItalicCorrection(mapped); ok {
			italic, err = a.emFromUnits(int64(value), info.UnitsPerEm)
			if err != nil {
				return layout.Glyph{}, err
			}
		}
	}
	extended := false
	if mathText {
		if value, ok := a.fonts.IsExtendedShape(mapped); ok {
			extended = value
		}
	}
	advanceIncludesItalic := mathText && !extended
	if advanceIncludesItalic {
		advance, err = advance.Add(italic)
		if err != nil {
			return layout.Glyph{}, err
		}
	}
	path, err := a.designPath(outline.Segments, info.UnitsPerEm)
	if err != nil {
		return layout.Glyph{}, err
	}
	result := layout.Glyph{
		ID:                    uint16(mapped.ID),
		FontID:                info.ID,
		Text:                  text,
		Advance:               advance,
		Ascent:                ascent,
		Descent:               descent,
		Italic:                italic,
		AdvanceIncludesItalic: advanceIncludesItalic,
		TextLike:              mathText && isTextLike(text),
		Outline:               path,
	}
	if mathText {
		result.Kerns = a.loadKerns(mapped)
		if value, ok := a.fonts.TopAccentAttachment(mapped); ok {
			result.TopAccent, err = a.emFromUnits(int64(value), info.UnitsPerEm)
			if err != nil {
				return layout.Glyph{}, err
			}
			result.HasTopAccent = true
		}
	}
	return result, nil
}

func (a *adapter) loadKerns(glyph font.Glyph) layout.MathKernInfo {
	load := func(corner font.MathKernCorner) *layout.MathKern {
		curve, ok := a.fonts.MathKernCurve(glyph, corner)
		if !ok {
			return nil
		}
		info, ok := a.fonts.Face(glyph.Face)
		if !ok {
			return nil
		}
		heights := make([]geometry.Em, len(curve.Heights))
		kerns := make([]geometry.Em, len(curve.Kerns))
		for i, value := range curve.Heights {
			em, err := a.emFromUnits(int64(value), info.UnitsPerEm)
			if err != nil {
				return nil
			}
			heights[i] = em
		}
		for i, value := range curve.Kerns {
			em, err := a.emFromUnits(int64(value), info.UnitsPerEm)
			if err != nil {
				return nil
			}
			kerns[i] = em
		}
		return &layout.MathKern{Heights: heights, Kerns: kerns}
	}
	return layout.MathKernInfo{
		TopLeft:     load(font.MathKernTopLeft),
		TopRight:    load(font.MathKernTopRight),
		BottomRight: load(font.MathKernBottomRight),
		BottomLeft:  load(font.MathKernBottomLeft),
	}
}
