package layout

import (
	"fmt"

	"github.com/caelyreth/untie/geometry"
	"github.com/caelyreth/untie/ir"
)

func (s *layoutState) layoutOverlay(node *ir.Overlay, env environment, depth int) (*Fragment, error) {
	body, err := s.layoutNode(node.Body, env, depth)
	if err != nil {
		return nil, err
	}
	fragment, err := s.newFragment(
		Metrics{Ascent: body.Ascent, Descent: body.Descent},
		ir.ClassNormal,
		env.Style.Size,
		s.scale(env.Style),
	)
	if err != nil {
		return nil, err
	}
	var x geometry.Em
	switch node.Alignment {
	case ir.OverlayStart:
		x = 0
	case ir.OverlayCenter:
		x = -body.Width / 2
	case ir.OverlayEnd:
		x = -body.Width
	default:
		return nil, fmt.Errorf("%w: invalid overlay alignment", ErrInvalidInput)
	}
	fragment.BaseAscent = body.BaseAscent
	fragment.BaseDescent = body.BaseDescent
	fragment.AccentTop, err = add(body.AccentTop, x)
	if err != nil {
		return nil, err
	}
	fragment.AccentBottom, err = add(body.AccentBottom, x)
	if err != nil {
		return nil, err
	}
	fragment.Italic, err = add(body.Italic, x)
	if err != nil {
		return nil, err
	}
	fragment.InkBottom = body.InkBottom
	fragment.HasInkBottom = body.HasInkBottom
	fragment.TextLike = body.TextLike
	if err := fragment.pushGroup(x, 0, body); err != nil {
		return nil, err
	}
	return withClass(fragment, node.Class), nil
}
