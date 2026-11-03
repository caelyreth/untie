package engine

import (
	"context"
	"errors"
	"fmt"
	"unicode/utf8"

	"github.com/caelyreth/untie/font"
	"github.com/caelyreth/untie/geometry"
	"github.com/caelyreth/untie/internal/layout"
	"github.com/caelyreth/untie/ir"
	"github.com/caelyreth/untie/source"
)

func (a *adapter) ShapeText(req layout.TextRequest) (layout.TextRun, error) {
	return a.shapeTextRun(
		req.Context,
		req.Text,
		req.Clusters,
		req.Style,
		req.Variant,
		req.Font,
		req.Features,
		req.Tracking,
	)
}

func (a *adapter) ShapeOperator(req layout.OperatorRequest) (layout.TextRun, error) {
	return a.shapeOperatorRun(req.Context, req.Text, req.Style, req.Features)
}

func (a *adapter) shapeTextRun(
	ctx context.Context,
	text string,
	sourceClusters []ir.TextClusterSource,
	style layout.Style,
	variant ir.Variant,
	role ir.FontRole,
	features []string,
	tracking geometry.Em,
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
	faces := a.faceOrder(role)
	styled, styledSource, badRune, ok := styledTextWithMap(text, variant, role == ir.FontMath)
	if !ok {
		return layout.TextRun{}, fmt.Errorf("%w: variant %d for %q", layout.ErrMissingGlyph, variant, string(badRune))
	}
	shaped, err := a.fonts.ShapeText(font.TextShapeRequest{
		Context:   ctx,
		Text:      styled,
		Faces:     faces,
		Features:  requestFeatures,
		MaxGlyphs: a.limits.MaxOutputItems,
	})
	if err != nil {
		switch {
		case errors.Is(err, font.ErrTextLimit):
			return layout.TextRun{}, fmt.Errorf("%w: %v", layout.ErrLimit, err)
		case errors.Is(err, font.ErrInvalidText):
			return layout.TextRun{}, fmt.Errorf("%w: %w", layout.ErrInvalidInput, err)
		default:
			return layout.TextRun{}, err
		}
	}

	sourceRunes := []rune(text)
	sourceOffsets := textByteOffsets(sourceRunes)
	run := layout.TextRun{Clusters: make([]layout.TextCluster, len(shaped.Clusters))}
	for i, cluster := range shaped.Clusters {
		start, end, ok := sourceClusterRange(styledSource, cluster.RuneStart, cluster.RuneEnd, sourceRunes)
		if !ok {
			return layout.TextRun{}, fmt.Errorf("%w: invalid styled text cluster", layout.ErrInvalidInput)
		}
		sourceRange := source.NewRange(sourceOffsets[start], sourceOffsets[end])
		if len(sourceClusters) > 0 {
			sourceRange, ok = mappedTextSourceRange(
				sourceClusters,
				sourceOffsets[start],
				sourceOffsets[end],
			)
			if !ok {
				return layout.TextRun{}, fmt.Errorf("%w: text cluster has no source mapping", layout.ErrInvalidInput)
			}
		}
		run.Clusters[i] = layout.TextCluster{
			RuneStart: start,
			RuneEnd:   end,
			Source:    sourceRange,
			Missing:   cluster.Missing,
		}
	}

	missingAdvances := make(map[int]geometry.Em)
	for _, positioned := range shaped.Glyphs {
		if positioned.Cluster < 0 || positioned.Cluster >= len(run.Clusters) ||
			!run.Clusters[positioned.Cluster].Missing {
			continue
		}
		info, ok := a.fonts.Face(positioned.Glyph.Face)
		if !ok {
			return layout.TextRun{}, fmt.Errorf("%w: face %d", layout.ErrMissingGlyph, positioned.Glyph.Face)
		}
		advance, err := a.emFromUnits(int64(positioned.Advance), info.UnitsPerEm)
		if err != nil {
			return layout.TextRun{}, err
		}
		total, err := missingAdvances[positioned.Cluster].Add(advance)
		if err != nil {
			return layout.TextRun{}, err
		}
		missingAdvances[positioned.Cluster] = total
	}

	var pen geometry.Em
	lastCluster := -1
	pathSegments := 0
	renderedMissing := make(map[int]bool)
	for i, positioned := range shaped.Glyphs {
		if err := checkEvery(ctx, i, 16); err != nil {
			return layout.TextRun{}, err
		}
		if positioned.Cluster < 0 || positioned.Cluster >= len(run.Clusters) {
			return layout.TextRun{}, fmt.Errorf("%w: invalid text cluster index %d", layout.ErrInvalidInput, positioned.Cluster)
		}
		if positioned.Cluster != lastCluster {
			if lastCluster >= 0 {
				pen, err = pen.Add(tracking)
				if err != nil {
					return layout.TextRun{}, err
				}
			}
			lastCluster = positioned.Cluster
			run.Clusters[positioned.Cluster].X = pen
		}
		if run.Clusters[positioned.Cluster].Missing {
			if renderedMissing[positioned.Cluster] {
				continue
			}
			renderedMissing[positioned.Cluster] = true
			advance := missingAdvances[positioned.Cluster]
			if advance <= 0 {
				advance = geometry.Em(geometry.EmScale / 2)
			}
			ascent := geometry.Em(geometry.EmScale * 3 / 4)
			descent := geometry.Em(geometry.EmScale / 8)
			glyph := layout.Glyph{
				Advance:               advance,
				Ascent:                ascent,
				Descent:               descent,
				AdvanceIncludesItalic: true,
				TextLike:              true,
			}
			if len(run.Glyphs) >= a.limits.MaxOutputItems {
				return layout.TextRun{}, fmt.Errorf("%w: output item count", layout.ErrLimit)
			}
			run.Clusters[positioned.Cluster].Advance, err =
				run.Clusters[positioned.Cluster].Advance.Add(advance)
			if err != nil {
				return layout.TextRun{}, err
			}
			run.Glyphs = append(run.Glyphs, layout.PositionedGlyph{
				Glyph:   glyph,
				At:      layout.Point{X: pen},
				Cluster: positioned.Cluster,
				Missing: true,
			})
			pen, err = pen.Add(advance)
			if err != nil {
				return layout.TextRun{}, err
			}
			run.Ascent = maxEm(run.Ascent, ascent)
			run.Descent = maxEm(run.Descent, descent)
			continue
		}
		info, ok := a.fonts.Face(positioned.Glyph.Face)
		if !ok {
			return layout.TextRun{}, fmt.Errorf("%w: face %d", layout.ErrMissingGlyph, positioned.Glyph.Face)
		}
		advance, err := a.emFromUnits(int64(positioned.Advance), info.UnitsPerEm)
		if err != nil {
			return layout.TextRun{}, err
		}
		if advance < 0 {
			return layout.TextRun{}, fmt.Errorf("%w: negative shaped advance", layout.ErrInvalidInput)
		}
		if positioned.Invisible {
			run.Clusters[positioned.Cluster].Advance, err =
				run.Clusters[positioned.Cluster].Advance.Add(advance)
			if err != nil {
				return layout.TextRun{}, err
			}
			pen, err = pen.Add(advance)
			if err != nil {
				return layout.TextRun{}, err
			}
			continue
		}
		xOffset, err := a.emFromUnits(int64(positioned.XOffset), info.UnitsPerEm)
		if err != nil {
			return layout.TextRun{}, err
		}
		yOffset, err := a.emFromUnits(-int64(positioned.YOffset), info.UnitsPerEm)
		if err != nil {
			return layout.TextRun{}, err
		}
		at, err := pen.Add(xOffset)
		if err != nil {
			return layout.TextRun{}, err
		}
		cluster := run.Clusters[positioned.Cluster]
		clusterText := string(sourceRunes[cluster.RuneStart:cluster.RuneEnd])
		glyph, err := a.glyphFromFont(positioned.Glyph, style, role == ir.FontMath, clusterText)
		if err != nil {
			return layout.TextRun{}, err
		}
		// The shaped advance includes kerning and GPOS adjustments, unlike the
		// immutable per-glyph design advance in the template cache.
		glyph.Advance = advance
		glyph.AdvanceIncludesItalic = false
		if len(run.Glyphs) >= a.limits.MaxOutputItems {
			return layout.TextRun{}, fmt.Errorf("%w: output item count", layout.ErrLimit)
		}
		run.Clusters[positioned.Cluster].Advance, err =
			run.Clusters[positioned.Cluster].Advance.Add(advance)
		if err != nil {
			return layout.TextRun{}, err
		}
		if err := reserveSegments(&pathSegments, len(glyph.Outline), a.limits.MaxPathSegments); err != nil {
			return layout.TextRun{}, err
		}
		run.Glyphs = append(run.Glyphs, layout.PositionedGlyph{
			Glyph:   glyph,
			At:      layout.Point{X: at, Y: yOffset},
			Cluster: positioned.Cluster,
		})
		pen, err = pen.Add(advance)
		if err != nil {
			return layout.TextRun{}, err
		}
		ascent, err := glyph.Ascent.Add(-yOffset)
		if err != nil {
			return layout.TextRun{}, err
		}
		descent, err := glyph.Descent.Add(yOffset)
		if err != nil {
			return layout.TextRun{}, err
		}
		run.Ascent = maxEm(run.Ascent, maxEm(0, ascent))
		run.Descent = maxEm(run.Descent, maxEm(0, descent))
		run.Italic = glyph.Italic
	}
	if lastCluster >= 0 {
		pen, err = pen.Add(tracking)
		if err != nil {
			return layout.TextRun{}, err
		}
	}
	if pen < 0 {
		return layout.TextRun{}, fmt.Errorf("%w: negative text advance", layout.ErrInvalidInput)
	}
	if len(run.Glyphs) == 0 {
		return layout.TextRun{}, fmt.Errorf("%w: empty shaped run", layout.ErrMissingGlyph)
	}
	for i, cluster := range run.Clusters {
		if cluster.Missing && !renderedMissing[i] {
			return layout.TextRun{}, fmt.Errorf("%w: missing cluster has no placeholder", layout.ErrMissingGlyph)
		}
	}
	run.Width = pen
	return run, nil
}

func mappedTextSourceRange(clusters []ir.TextClusterSource, start, end int) (source.Range, bool) {
	if start < 0 || end <= start {
		return source.Range{}, false
	}
	var merged source.Range
	found := false
	for _, cluster := range clusters {
		if cluster.TextEnd <= start {
			continue
		}
		if cluster.TextStart >= end {
			break
		}
		if !found {
			merged = cluster.Source
			found = true
		} else {
			merged = source.Merge(merged, cluster.Source)
		}
	}
	return merged, found
}

func sourceClusterRange(styledToSource []int, start, end int, sourceRunes []rune) (int, int, bool) {
	if start < 0 || end <= start || end > len(styledToSource) {
		return 0, 0, false
	}
	sourceStart := styledToSource[start]
	sourceEnd := styledToSource[end-1] + 1
	if sourceStart < 0 || sourceEnd <= sourceStart || sourceEnd > len(sourceRunes) {
		return 0, 0, false
	}
	return sourceStart, sourceEnd, true
}

func textByteOffsets(runes []rune) []int {
	offsets := make([]int, len(runes)+1)
	offset := 0
	for i, r := range runes {
		offsets[i] = offset
		offset += utf8.RuneLen(r)
	}
	offsets[len(runes)] = offset
	return offsets
}

// shapeOperatorRun keeps named math operators on the MATH font's one-to-one
// GSUB path. General text kerning is correct for prose but changes established
// operator metrics and spacing, which belong to the formula layout contract.
func (a *adapter) shapeOperatorRun(
	ctx context.Context,
	text string,
	style layout.Style,
	features []string,
) (layout.TextRun, error) {
	if err := requestContext(ctx); err != nil {
		return layout.TextRun{}, err
	}
	if len(text) > a.limits.MaxTextBytes {
		return layout.TextRun{}, fmt.Errorf("%w: text byte count %d", layout.ErrLimit, len(text))
	}
	requestFeatures, err := mathFeatures(features, style)
	if err != nil {
		return layout.TextRun{}, err
	}
	faces := a.faceOrder(ir.FontMath)
	runes := []rune(text)
	run := layout.TextRun{}
	var pen geometry.Em
	pathSegments := 0
	for i := 0; i < len(runes); {
		if err := checkEvery(ctx, i, 16); err != nil {
			return layout.TextRun{}, err
		}
		styled, ok := styleRune(runes[i], ir.VariantNormal, true)
		if !ok {
			return layout.TextRun{}, fmt.Errorf("%w: operator %q", layout.ErrMissingGlyph, string(runes[i]))
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
		glyph, err := a.glyphFromFont(mapped, style, true, string(runes[i]))
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
	key := glyphTemplateKey{glyph: mapped, mathText: mathText}
	if cached, ok := a.glyphTemplates.Load(key); ok {
		return materializeGlyph(cached.(*glyphTemplate), text), nil
	}

	info, ok := a.fonts.Face(mapped.Face)
	if !ok {
		return layout.Glyph{}, fmt.Errorf("%w: face %d", layout.ErrMissingGlyph, mapped.Face)
	}
	metrics, ok := a.fonts.GlyphMetrics(mapped)
	if !ok {
		return layout.Glyph{}, fmt.Errorf("%w: metrics for %q", layout.ErrMissingGlyph, text)
	}
	advance, err := a.emFromUnits(int64(metrics.Advance), info.UnitsPerEm)
	if err != nil {
		return layout.Glyph{}, err
	}
	ascent, descent := geometry.Em(0), geometry.Em(0)
	if bounds := metrics.Bounds; !bounds.Empty() {
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
	stretchableHorizontal := false
	if mathText {
		if value, ok := a.fonts.IsExtendedShape(mapped); ok {
			extended = value
		}
		_, stretchableHorizontal = a.cachedMathConstruction(mapped, font.HorizontalAxis)
	}
	advanceIncludesItalic := mathText && !extended
	if advanceIncludesItalic {
		advance, err = advance.Add(italic)
		if err != nil {
			return layout.Glyph{}, err
		}
	}
	path, err := a.cachedDesignPath(mapped, info.UnitsPerEm)
	if err != nil {
		return layout.Glyph{}, err
	}
	template := &glyphTemplate{mathText: mathText, glyph: layout.Glyph{
		ID:                    uint16(mapped.ID),
		FontID:                info.ID,
		Advance:               advance,
		Ascent:                ascent,
		Descent:               descent,
		Italic:                italic,
		AdvanceIncludesItalic: advanceIncludesItalic,
		StretchableHorizontal: stretchableHorizontal,
		ExactGlyphRef:         true,
		Outline:               path,
	}}
	if mathText {
		template.glyph.Kerns = a.loadKerns(mapped)
		if value, ok := a.fonts.TopAccentAttachment(mapped); ok {
			template.glyph.TopAccent, err = a.emFromUnits(int64(value), info.UnitsPerEm)
			if err != nil {
				return layout.Glyph{}, err
			}
			template.glyph.HasTopAccent = true
		}
	}
	actual, _ := a.glyphTemplates.LoadOrStore(key, template)
	return materializeGlyph(actual.(*glyphTemplate), text), nil
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
