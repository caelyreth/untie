package layout

import (
	"fmt"

	"github.com/caelyreth/untie/geometry"
	"github.com/caelyreth/untie/ir"
)

type stretchSpec struct {
	axis      Axis
	target    geometry.Em
	shortFall geometry.Em
}

func (s *layoutState) layoutGlyphText(text string, env environment, stretch *stretchSpec) (*Fragment, error) {
	if text == "" {
		return nil, fmt.Errorf("%w: empty glyph text", ErrInvalidInput)
	}
	req := GlyphRequest{
		Text:     text,
		Style:    env.Style,
		Variant:  env.Variant,
		Font:     env.Font,
		Features: s.features,
	}

	glyph, ok := s.glyph(req, stretch)
	if !ok {
		return nil, fmt.Errorf("%w: %q in style %s", ErrMissingGlyph, text, env.Style)
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
	fragment, err := newFragment(metrics, ir.ClassNormal, env.Style.Size, scale)
	if err != nil {
		return nil, err
	}
	fragment.Italic = scaled.Glyph.Italic
	fragment.BaseAscent = scaled.Glyph.Ascent
	fragment.BaseDescent = scaled.Glyph.Descent
	fragment.TextLike = scaled.Glyph.TextLike
	if err := fragment.pushGlyph(0, 0, scaled); err != nil {
		return nil, err
	}
	return fragment, nil
}

func (s *layoutState) glyph(req GlyphRequest, stretch *stretchSpec) (Glyph, bool) {
	if stretch == nil || s.renderer.fonts.Stretch == nil {
		return s.renderer.fonts.Glyphs.Glyph(req)
	}
	target, err := s.localLength(stretch.target, req.Style)
	if err != nil {
		return Glyph{}, false
	}
	shortFall, err := s.localLength(stretch.shortFall, req.Style)
	if err != nil {
		return Glyph{}, false
	}
	if glyph, ok := s.renderer.fonts.Stretch.Stretch(StretchRequest{
		GlyphRequest: req,
		Axis:         stretch.axis,
		Target:       target,
		ShortFall:    shortFall,
	}); ok {
		return glyph, true
	}
	return s.renderer.fonts.Glyphs.Glyph(req)
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
	if scaled.TopAccent, err = scale(glyph.TopAccent, factor); err != nil {
		return nil, err
	}
	scaled.Outline = make(Path, len(glyph.Outline))
	for i, segment := range glyph.Outline {
		scaled.Outline[i].Op = segment.Op
		for j, point := range segment.Points {
			if scaled.Outline[i].Points[j].X, err = scale(point.X, factor); err != nil {
				return nil, err
			}
			if scaled.Outline[i].Points[j].Y, err = scale(point.Y, factor); err != nil {
				return nil, err
			}
		}
	}
	return &GlyphFragment{Glyph: scaled, Scale: factor, Paint: paint}, nil
}

func (s *layoutState) layoutSpace(space *ir.Space, env environment) (*Fragment, error) {
	fragment := emptyFragment(env.Style.Size, s.scale(env.Style))
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
		width, err = s.length(space.Amount, env.Style, metrics)
	}
	if err != nil {
		return nil, err
	}
	fragment.Width = width
	return fragment, nil
}

func (s *layoutState) layoutPlaceholder(node *ir.Placeholder, env environment) (*Fragment, error) {
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
	fragment, err := newFragment(Metrics{Width: width, Ascent: ascent, Descent: descent}, ir.ClassNormal, env.Style.Size, s.scale(env.Style))
	if err != nil {
		return nil, err
	}
	paint := ir.Paint{Role: ir.PaintPlaceholder}
	// Four thin rules form a deterministic missing-content box.
	rules := []struct {
		x, y geometry.Em
		rule ruleDraw
	}{
		{0, -descent, ruleDraw{width: width, thickness: thickness, paint: paint}},
		{0, ascent - thickness, ruleDraw{width: width, thickness: thickness, paint: paint}},
		{0, -descent, ruleDraw{width: ascent + descent, thickness: thickness, paint: paint, vertical: true}},
		{width - thickness, -descent, ruleDraw{width: ascent + descent, thickness: thickness, paint: paint, vertical: true}},
	}
	for _, at := range rules {
		rule := at.rule
		if err := fragment.pushRule(at.x, at.y, &rule); err != nil {
			return nil, err
		}
	}
	fragment.BaseAscent = ascent
	fragment.BaseDescent = descent
	return fragment, nil
}
