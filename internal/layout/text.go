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
	run, err := s.renderer.fonts.Text.ShapeText(TextRequest{
		Context:  s.ctx,
		Text:     node.Text,
		Style:    env.Style,
		Variant:  env.Variant,
		Font:     node.Font,
		Tracking: tracking,
		Features: env.Features,
	})
	if err != nil {
		return nil, err
	}
	return s.layoutTextRun(run, env, true)
}

func (s *layoutState) layoutOperator(node *ir.Operator, env environment) (*Fragment, error) {
	if len([]rune(node.Text)) == 1 {
		fragment, err := s.layoutClassGlyphText(node.Text, node.Class, env, nil)
		if err != nil {
			return nil, err
		}
		return withClass(fragment, node.Class), nil
	}
	if s.renderer.fonts.Operators == nil {
		return nil, fmt.Errorf("%w: multi-glyph operator %q", ErrUnsupported, node.Text)
	}
	run, err := s.renderer.fonts.Operators.ShapeOperator(OperatorRequest{
		Context:  s.ctx,
		Text:     node.Text,
		Style:    env.Style,
		Variant:  env.Variant,
		Features: env.Features,
	})
	if err != nil {
		return nil, err
	}
	fragment, err := s.layoutTextRun(run, env, len([]rune(node.Text)) > 1)
	if err != nil {
		return nil, err
	}
	return withClass(fragment, node.Class), nil
}

func (s *layoutState) layoutTextRun(run TextRun, env environment, textLike bool) (*Fragment, error) {
	if run.Width < 0 || run.Ascent < 0 || run.Descent < 0 {
		return nil, fmt.Errorf("%w: run has negative metrics", ErrInvalidInput)
	}
	if len(run.Glyphs) == 0 {
		return nil, fmt.Errorf("%w: empty shaped run", ErrMissingGlyph)
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
	fragment.TextLike = textLike
	for _, positioned := range run.Glyphs {
		if positioned.Glyph.Advance < 0 || positioned.Glyph.Ascent < 0 || positioned.Glyph.Descent < 0 {
			return nil, fmt.Errorf("%w: shaped run glyph has negative metrics", ErrInvalidInput)
		}
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
