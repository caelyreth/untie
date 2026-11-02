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
		fragment := emptyFragment(env.Style.Size, s.scale(env.Style))
		fragment.TextLike = false
		return withClass(fragment, node.Class), nil
	}
	// KaTeX renders every explicit delimiter tier in text style. The size tier
	// is therefore stable inside scripts rather than inheriting their scale.
	env.Style = Style{Size: Text}
	target, err := sizedDelimiterHeight(node.Size)
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

func sizedDelimiterHeight(size ir.DelimiterSize) (geometry.Em, error) {
	switch size {
	case ir.DelimiterSize1:
		return geometry.EmFromParts(6, 5)
	case ir.DelimiterSize2:
		return geometry.EmFromParts(9, 5)
	case ir.DelimiterSize3:
		return geometry.EmFromParts(12, 5)
	case ir.DelimiterSize4:
		return geometry.EmFromParts(3, 1)
	default:
		return 0, fmt.Errorf("%w: invalid delimiter size %d", ErrInvalidInput, size)
	}
}
