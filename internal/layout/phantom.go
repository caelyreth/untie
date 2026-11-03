package layout

import (
	"github.com/caelyreth/untie/geometry"
	"github.com/caelyreth/untie/ir"
)

func (s *layoutState) layoutPhantom(node *ir.Phantom, env environment, depth int) (*Fragment, error) {
	body, err := s.layoutNode(node.Body, env, depth)
	if err != nil {
		return nil, err
	}
	var width, ascent, descent geometry.Em
	if node.Width {
		width = body.Width
	}
	if node.Height {
		ascent = body.Ascent
	}
	if node.Depth {
		descent = body.Descent
	}
	fragment, err := s.newFragment(Metrics{Width: width, Ascent: ascent, Descent: descent}, ir.ClassNormal, env.Style.Size, s.scale(env.Style))
	if err != nil {
		return nil, err
	}
	fragment.BaseAscent = ascent
	fragment.BaseDescent = descent
	fragment.AccentTop = body.AccentTop
	fragment.AccentBottom = body.AccentBottom
	fragment.Italic = body.Italic
	fragment.TextLike = body.TextLike
	return withClass(fragment, node.Class), nil
}
