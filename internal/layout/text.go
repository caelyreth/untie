package layout

import (
	"fmt"

	"github.com/caelyreth/untie/ir"
)

func (s *layoutState) layoutText(node *ir.Text, env environment) (*Fragment, error) {
	if s.renderer.fonts.Text == nil {
		return nil, fmt.Errorf("%w: text source is not configured", ErrUnsupported)
	}
	metrics := s.fontMetrics(env.Style, env.Variant)
	tracking, err := s.length(node.Tracking, env.Style, metrics)
	if err != nil {
		return nil, err
	}
	run, ok := s.renderer.fonts.Text.ShapeText(TextRequest{
		Text:     node.Text,
		Style:    env.Style,
		Variant:  env.Variant,
		Tracking: tracking,
		Features: s.features,
	})
	if !ok {
		return nil, fmt.Errorf("%w: text %q", ErrMissingGlyph, node.Text)
	}
	if run.Width < 0 || run.Ascent < 0 || run.Descent < 0 {
		return nil, fmt.Errorf("%w: text run has negative metrics", ErrInvalidInput)
	}
	factor := s.scale(env.Style)
	width, err := scale(run.Width, factor)
	if err != nil {
		return nil, err
	}
	ascent, err := scale(run.Ascent, factor)
	if err != nil {
		return nil, err
	}
	descent, err := scale(run.Descent, factor)
	if err != nil {
		return nil, err
	}
	italic, err := scale(run.Italic, factor)
	if err != nil {
		return nil, err
	}
	fragment, err := newFragment(Metrics{Width: width, Ascent: ascent, Descent: descent}, ir.ClassNormal, env.Style.Size, factor)
	if err != nil {
		return nil, err
	}
	fragment.Italic = italic
	fragment.BaseAscent = ascent
	fragment.BaseDescent = descent
	fragment.TextLike = true
	for _, positioned := range run.Glyphs {
		draw, err := s.scaleGlyph(positioned.Glyph, env.Style, env.Paint)
		if err != nil {
			return nil, err
		}
		x, err := scale(positioned.At.X, factor)
		if err != nil {
			return nil, err
		}
		y, err := scale(positioned.At.Y, factor)
		if err != nil {
			return nil, err
		}
		if err := fragment.pushGlyph(x, y, draw); err != nil {
			return nil, err
		}
	}
	return fragment, nil
}

func (s *layoutState) layoutStyle(node *ir.Style, env environment, depth int) (*Fragment, error) {
	child := env
	if node.SetSize {
		size, ok := styleFromIR(node.NewSize)
		if !ok {
			return nil, fmt.Errorf("%w: invalid explicit style", ErrInvalidInput)
		}
		child.Style.Size = size
	}
	if node.SetCramped {
		child.Style.Cramped = node.Cramped
	}
	fragment, err := s.layoutNode(node.Body, child, depth)
	if err != nil {
		return nil, err
	}
	return withClass(fragment, node.Class), nil
}
