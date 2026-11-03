package layout

import (
	"fmt"

	"github.com/caelyreth/untie/geometry"
	"github.com/caelyreth/untie/ir"
)

func (s *layoutState) layoutSizedDelimiter(node *ir.SizedDelimiter, env environment) (*Fragment, error) {
	if node.Text == "" {
		return nil, fmt.Errorf("%w: empty sized delimiter", ErrInvalidInput)
	}
	if node.Text == "." {
		fragment := s.emptyFragment(env.Style.Size, s.scale(env.Style))
		fragment.TextLike = false
		return withClass(fragment, node.Class), nil
	}
	unitTarget, err := s.metricAt(geometry.Em(geometry.EmScale), env.Style)
	if err != nil {
		return nil, err
	}
	target, err := s.delimiterSizingTarget(node.Sizing, unitTarget, env)
	if err != nil {
		return nil, err
	}
	fragment, err := s.layoutGlyphText(node.Text, env, &stretchSpec{
		axis:   Vertical,
		target: target,
	})
	if err != nil {
		return nil, err
	}
	fragment.TextLike = false
	return withClass(fragment, node.Class), nil
}
