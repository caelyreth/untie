package engine

import (
	"context"
	"errors"
	"fmt"
	"math"

	"github.com/caelyreth/untie/font"
	"github.com/caelyreth/untie/geometry"
	"github.com/caelyreth/untie/internal/layout"
	"github.com/caelyreth/untie/ir"
	"github.com/caelyreth/untie/source"
)

type adapter struct {
	fonts     *font.FontSet
	limits    source.Limits
	metrics   layout.MathMetrics
	mathFaces []font.FaceRef
	textFaces []font.FaceRef
}

func newAdapter(fonts *font.FontSet, limits source.Limits) (*adapter, error) {
	a := &adapter{fonts: fonts, limits: limits.WithDefaults()}
	metrics, err := a.extractMathMetrics()
	if err != nil {
		return nil, err
	}
	a.metrics = metrics
	a.mathFaces = a.buildFaceOrder(ir.FontMath)
	a.textFaces = a.buildFaceOrder(ir.FontText)
	return a, nil
}

func (a *adapter) MathMetrics(_ layout.Style, _ ir.Variant) layout.MathMetrics {
	return a.metrics
}

func (a *adapter) Glyph(req layout.GlyphRequest) (layout.Glyph, error) {
	if err := requestContext(req.Context); err != nil {
		return layout.Glyph{}, err
	}
	mapped, text, ok := a.shapedGlyph(req)
	if !ok {
		return layout.Glyph{}, fmt.Errorf("%w: %q", layout.ErrMissingGlyph, req.Text)
	}
	return a.glyphFromFont(mapped, req.Style, req.Font == ir.FontMath, text)
}

func (a *adapter) Stretch(req layout.StretchRequest) (layout.Glyph, error) {
	if err := requestContext(req.Context); err != nil {
		return layout.Glyph{}, err
	}
	mapped, text, ok := a.shapedGlyph(req.GlyphRequest)
	if !ok {
		return layout.Glyph{}, fmt.Errorf("%w: %q", layout.ErrMissingGlyph, req.Text)
	}
	base, err := a.glyphFromFont(mapped, req.Style, req.Font == ir.FontMath, text)
	if err != nil {
		return layout.Glyph{}, err
	}
	required, err := subNonNegative(req.Target, req.ShortFall)
	if err != nil {
		return layout.Glyph{}, err
	}
	extent, err := glyphExtent(base, req.Axis)
	if err != nil {
		return layout.Glyph{}, err
	}
	if extent >= required {
		return base, nil
	}

	info, ok := a.fonts.Face(mapped.Face)
	if !ok {
		return layout.Glyph{}, fmt.Errorf("%w: missing stretched face", layout.ErrMissingGlyph)
	}
	axis := font.VerticalAxis
	if req.Axis == layout.Horizontal {
		axis = font.HorizontalAxis
	}
	construction, ok := a.fonts.MathConstruction(mapped, axis)
	if !ok {
		return layout.Glyph{}, fmt.Errorf("%w: no %s construction", layout.ErrInsufficientStretch, axisName(req.Axis))
	}
	target, err := a.toDesignUnits(req.Target, info.UnitsPerEm)
	if err != nil {
		return layout.Glyph{}, err
	}
	shortTarget, err := subNonNegative(req.Target, req.ShortFall)
	if err != nil {
		return layout.Glyph{}, err
	}
	requiredUnits, err := a.toDesignUnits(shortTarget, info.UnitsPerEm)
	if err != nil {
		return layout.Glyph{}, err
	}

	var best layout.Glyph
	var bestExtent geometry.Em
	for _, variant := range construction.Variants {
		if err := req.Context.Err(); err != nil {
			return layout.Glyph{}, err
		}
		if int64(variant.Advance) < int64(requiredUnits) {
			continue
		}
		candidate := font.Glyph{Face: mapped.Face, ID: variant.Glyph}
		glyph, err := a.glyphFromFont(candidate, req.Style, true, text)
		if err != nil {
			if errors.Is(err, layout.ErrLimit) {
				return layout.Glyph{}, err
			}
			continue
		}
		extent, err := glyphExtent(glyph, req.Axis)
		if err != nil {
			return layout.Glyph{}, err
		}
		if extent >= required {
			if req.Axis == layout.Vertical {
				glyph, err = a.centerOnAxis(glyph)
				if err != nil {
					return layout.Glyph{}, err
				}
			}
			return glyph, nil
		}
		if bestExtent == 0 || extent > bestExtent {
			best, bestExtent = glyph, extent
		}
	}
	if construction.Assembly != nil {
		assembled, err := a.assembleGlyph(req.Context, base, mapped, construction, target, req.Target, axis, text)
		if err != nil {
			return layout.Glyph{}, err
		}
		extent, err := glyphExtent(assembled, req.Axis)
		if err != nil {
			return layout.Glyph{}, err
		}
		if extent >= required {
			if req.Axis == layout.Vertical {
				assembled, err = a.centerOnAxis(assembled)
				if err != nil {
					return layout.Glyph{}, err
				}
			}
			return assembled, nil
		}
	}
	if bestExtent >= required {
		return best, nil
	}
	return layout.Glyph{}, fmt.Errorf(
		"%w: %s construction did not reach %d (target %d)",
		layout.ErrInsufficientStretch,
		axisName(req.Axis),
		required,
		req.Target,
	)
}

func requestContext(ctx context.Context) error {
	if ctx == nil {
		return fmt.Errorf("%w: nil context", layout.ErrInvalidInput)
	}
	return ctx.Err()
}

func checkEvery(ctx context.Context, index, interval int) error {
	if index%interval == 0 {
		return ctx.Err()
	}
	return nil
}

func reserveSegments(used *int, count, limit int) error {
	if count < 0 || count > limit || *used > limit-count {
		return fmt.Errorf("%w: path segment count would exceed %d", layout.ErrLimit, limit)
	}
	*used += count
	return nil
}

func subNonNegative(a, b geometry.Em) (geometry.Em, error) {
	if b < 0 {
		return 0, fmt.Errorf("%w: negative shortfall", layout.ErrInvalidInput)
	}
	out, err := a.Add(-b)
	if err != nil {
		return 0, err
	}
	if out < 0 {
		return 0, fmt.Errorf("%w: target smaller than shortfall", layout.ErrInvalidInput)
	}
	return out, nil
}

func glyphExtent(g layout.Glyph, axis layout.Axis) (geometry.Em, error) {
	if axis == layout.Horizontal {
		extent := g.Advance
		if g.AdvanceIncludesItalic {
			if g.Italic == geometry.Em(math.MinInt64) {
				return 0, geometry.ErrOverflow
			}
			var err error
			extent, err = extent.Add(-g.Italic)
			if err != nil {
				return 0, err
			}
		}
		return extent, nil
	}
	return g.Ascent.Add(g.Descent)
}

func axisName(axis layout.Axis) string {
	if axis == layout.Horizontal {
		return "horizontal"
	}
	return "vertical"
}
