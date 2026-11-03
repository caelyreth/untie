package layout

import (
	"errors"
	"fmt"

	"github.com/caelyreth/untie/geometry"
	"github.com/caelyreth/untie/ir"
)

type stretchSpec struct {
	axis      Axis
	target    geometry.Em
	shortFall geometry.Em
	// bestEffort keeps an unstretched glyph when the font has no usable
	// construction. Explicit delimiter and accent requests remain strict.
	bestEffort bool
}

func (s *layoutState) automaticDisplayStretch(class ir.MathClass, env environment) *stretchSpec {
	if env.Style.Size != Display || class != ir.ClassLarge {
		return nil
	}
	target := s.fontMetrics(env.Style, env.Variant).DisplayOperatorMinHeight
	if target <= 0 {
		return nil
	}
	return &stretchSpec{axis: Vertical, target: target, bestEffort: true}
}

func (s *layoutState) layoutClassGlyphText(
	text string,
	class ir.MathClass,
	env environment,
	stretch *stretchSpec,
) (*Fragment, error) {
	if stretch == nil {
		stretch = s.automaticDisplayStretch(class, env)
	}
	return s.layoutGlyphText(text, env, stretch)
}

func (s *layoutState) layoutGlyphText(text string, env environment, stretch *stretchSpec) (*Fragment, error) {
	if text == "" {
		return nil, fmt.Errorf("%w: empty glyph text", ErrInvalidInput)
	}
	req := GlyphRequest{
		Context:  s.ctx,
		Text:     text,
		Style:    env.Style,
		Variant:  env.Variant,
		Font:     env.Font,
		Features: env.Features,
	}

	glyph, err := s.glyph(req, stretch)
	if err != nil {
		return nil, err
	}
	scaled, err := s.scaleGlyph(glyph, env.Style, env.Paint)
	if err != nil {
		return nil, err
	}
	scale := s.scale(env.Style)
	metrics := Metrics{
		Width:   scaled.Glyph.Advance,
		Ascent:  scaled.Glyph.Ascent,
		Descent: scaled.Glyph.Descent,
	}
	fragment, err := s.newFragment(metrics, ir.ClassNormal, env.Style.Size, scale)
	if err != nil {
		return nil, err
	}
	fragment.reserveDraws(1)
	fragment.Italic = scaled.Glyph.Italic
	fragment.TextLike = scaled.Glyph.TextLike
	fragment.StretchableHorizontal = scaled.Glyph.StretchableHorizontal
	fragment.BaseAscent = scaled.Glyph.Ascent
	fragment.BaseDescent = scaled.Glyph.Descent
	fragment.AccentTop = (fragment.Width + fragment.Italic) / 2
	if scaled.Glyph.HasTopAccent {
		fragment.AccentTop = scaled.Glyph.TopAccent
	}
	if scaled.Glyph.HasBottomAccent {
		fragment.AccentBottom = scaled.Glyph.BottomAccent
	} else {
		fragment.AccentBottom = (fragment.Width - fragment.Italic) / 2
	}
	fragment.InkBottom = scaled.Glyph.InkBottom
	fragment.HasInkBottom = scaled.Glyph.HasInkBottom
	if !fragment.HasInkBottom {
		if bottom, ok := pathInkBottom(scaled.Glyph.Outline, scaled.Scale); ok {
			fragment.InkBottom, err = bottom.Add(scaled.Glyph.OriginY)
			if err != nil {
				return nil, err
			}
			fragment.HasInkBottom = true
		}
	}
	fragment.kerns = &scaled.Glyph.Kerns
	if err := fragment.pushGlyph(0, 0, scaled); err != nil {
		return nil, err
	}
	return fragment, nil
}

func (s *layoutState) glyph(req GlyphRequest, stretch *stretchSpec) (Glyph, error) {
	glyph, err := s.renderer.fonts.Glyphs.Glyph(req)
	if err != nil {
		return Glyph{}, err
	}
	if stretch == nil {
		return glyph, nil
	}
	target, err := s.localLength(stretch.target, req.Style)
	if err != nil {
		return Glyph{}, err
	}
	shortFall, err := s.localLength(stretch.shortFall, req.Style)
	if err != nil {
		return Glyph{}, err
	}
	if target < 0 || shortFall < 0 {
		return Glyph{}, fmt.Errorf("%w: negative stretch target", ErrInvalidInput)
	}
	if s.renderer.fonts.Stretch != nil {
		stretched, err := s.renderer.fonts.Stretch.Stretch(StretchRequest{
			GlyphRequest: req,
			Axis:         stretch.axis,
			Target:       target,
			ShortFall:    shortFall,
		})
		if err != nil {
			if stretch.bestEffort && errors.Is(err, ErrInsufficientStretch) {
				return glyph, nil
			}
			return Glyph{}, err
		}
		glyph = stretched
	}
	extent, err := glyphExtent(glyph, stretch.axis)
	if err != nil {
		return Glyph{}, err
	}
	required, err := sub(target, shortFall)
	if err != nil {
		return Glyph{}, err
	}
	if extent < required {
		if stretch.bestEffort {
			return glyph, nil
		}
		axis := "horizontal"
		if stretch.axis == Vertical {
			axis = "vertical"
		}
		return Glyph{}, fmt.Errorf(
			"%w: %w: %s glyph extent %d is less than required %d",
			ErrUnsupported, ErrInsufficientStretch, axis, extent, required,
		)
	}
	return glyph, nil
}

func glyphExtent(glyph Glyph, axis Axis) (geometry.Em, error) {
	if axis == Horizontal {
		extent := glyph.Advance
		if glyph.AdvanceIncludesItalic {
			return sub(extent, glyph.Italic)
		}
		return extent, nil
	}
	return add(glyph.Ascent, glyph.Descent)
}

func (s *layoutState) localLength(value geometry.Em, style Style) (geometry.Em, error) {
	return unscale(value, s.scale(style))
}

func (s *layoutState) scaleGlyph(glyph Glyph, style Style, paint ir.Paint) (*GlyphFragment, error) {
	if glyph.Advance < 0 || glyph.Ascent < 0 || glyph.Descent < 0 {
		return nil, fmt.Errorf("%w: glyph %q has negative metrics", ErrInvalidInput, glyph.Text)
	}
	factor := s.scale(style)
	scaled := glyph
	var err error
	if scaled.Advance, err = scale(glyph.Advance, factor); err != nil {
		return nil, err
	}
	if scaled.Ascent, err = scale(glyph.Ascent, factor); err != nil {
		return nil, err
	}
	if scaled.Descent, err = scale(glyph.Descent, factor); err != nil {
		return nil, err
	}
	if scaled.Italic, err = scale(glyph.Italic, factor); err != nil {
		return nil, err
	}
	if scaled.OriginY, err = scale(glyph.OriginY, factor); err != nil {
		return nil, err
	}
	if scaled.TopAccent, err = scale(glyph.TopAccent, factor); err != nil {
		return nil, err
	}
	if scaled.BottomAccent, err = scale(glyph.BottomAccent, factor); err != nil {
		return nil, err
	}
	if scaled.InkBottom, err = scale(glyph.InkBottom, factor); err != nil {
		return nil, err
	}
	if err := validateKernInfo(glyph.Kerns); err != nil {
		return nil, err
	}
	if len(glyph.Parts) > 0 {
		scaled.Parts = make([]PositionedGlyph, len(glyph.Parts))
		for i, part := range glyph.Parts {
			if err := s.ctx.Err(); err != nil {
				return nil, err
			}
			scaledPart, err := s.scaleGlyph(part.Glyph, style, paint)
			if err != nil {
				return nil, err
			}
			atX, err := scale(part.At.X, factor)
			if err != nil {
				return nil, err
			}
			atY, err := scale(part.At.Y, factor)
			if err != nil {
				return nil, err
			}
			scaled.Parts[i] = PositionedGlyph{
				Glyph: scaledPart.Glyph,
				At:    Point{X: atX, Y: atY},
			}
		}
	}
	if err := s.reservePathSegments(len(glyph.Outline)); err != nil {
		return nil, err
	}
	scaled.Outline = glyph.Outline
	return s.newGlyphFragment(scaled, factor, paint), nil
}

func validateKernInfo(info MathKernInfo) error {
	for _, kern := range [...]*MathKern{
		info.TopLeft,
		info.TopRight,
		info.BottomRight,
		info.BottomLeft,
	} {
		if err := validateKern(kern); err != nil {
			return err
		}
	}
	return nil
}

func validateKern(kern *MathKern) error {
	if kern == nil {
		return nil
	}
	if len(kern.Kerns) != len(kern.Heights)+1 {
		return fmt.Errorf("%w: malformed math kern table", ErrInvalidInput)
	}
	for i := 1; i < len(kern.Heights); i++ {
		if kern.Heights[i] <= kern.Heights[i-1] {
			return fmt.Errorf("%w: math kern heights are not increasing", ErrInvalidInput)
		}
	}
	return nil
}

func (s *layoutState) layoutSpace(space *ir.Space, env environment) (*Fragment, error) {
	fragment := s.emptyFragment(env.Style.Size, s.scale(env.Style))
	fragment.Class = ir.ClassSpace
	if space.Type == ir.SpaceIgnored {
		return fragment, nil
	}
	metrics := s.fontMetrics(env.Style, env.Variant)
	var width geometry.Em
	var err error
	if space.Type == ir.SpaceInterword {
		width, err = s.metricAt(metrics.SpaceWidth, env.Style)
	} else {
		width, err = s.signedLength(space.Amount, env.Style, metrics)
	}
	if err != nil {
		return nil, err
	}
	if width < 0 {
		return nil, fmt.Errorf("%w: negative standalone space", ErrInvalidInput)
	}
	fragment.Width = width
	return fragment, nil
}

func (s *layoutState) layoutPlaceholder(node *ir.Placeholder, env environment) (*Fragment, error) {
	return s.layoutMissingFragment(env, ir.Paint{Role: ir.PaintPlaceholder})
}

func (s *layoutState) layoutMissingFragment(env environment, paint ir.Paint) (*Fragment, error) {
	width, err := s.metricAt(geometry.Em(geometry.EmScale/2), env.Style)
	if err != nil {
		return nil, err
	}
	ascent, err := s.metricAt(geometry.Em(geometry.EmScale*3/4), env.Style)
	if err != nil {
		return nil, err
	}
	descent, err := s.metricAt(geometry.Em(geometry.EmScale/8), env.Style)
	if err != nil {
		return nil, err
	}
	thickness, err := s.metricAt(geometry.Em(geometry.EmScale/64), env.Style)
	if err != nil {
		return nil, err
	}
	return s.placeholderFragment(
		Metrics{Width: width, Ascent: ascent, Descent: descent},
		thickness,
		s.scale(env.Style),
		env.Style.Size,
		paint,
	)
}

func (s *layoutState) placeholderFragment(
	metrics Metrics,
	thickness geometry.Em,
	scale geometry.Em,
	size MathSize,
	paint ir.Paint,
) (*Fragment, error) {
	fragment, err := s.newFragment(metrics, ir.ClassNormal, size, scale)
	if err != nil {
		return nil, err
	}
	fragment.reserveDraws(4)
	// Four thin rules form a deterministic missing-content box.
	rules := []struct {
		x, y geometry.Em
		rule ruleDraw
	}{
		{0, -metrics.Descent, ruleDraw{width: metrics.Width, thickness: thickness, paint: paint}},
		{0, metrics.Ascent - thickness, ruleDraw{width: metrics.Width, thickness: thickness, paint: paint}},
		{0, -metrics.Descent, ruleDraw{width: metrics.Ascent + metrics.Descent, thickness: thickness, paint: paint, vertical: true}},
		{metrics.Width - thickness, -metrics.Descent, ruleDraw{width: metrics.Ascent + metrics.Descent, thickness: thickness, paint: paint, vertical: true}},
	}
	for _, at := range rules {
		rule := at.rule
		if err := fragment.pushRule(at.x, at.y, &rule); err != nil {
			return nil, err
		}
	}
	fragment.BaseAscent = metrics.Ascent
	fragment.BaseDescent = metrics.Descent
	return fragment, nil
}
