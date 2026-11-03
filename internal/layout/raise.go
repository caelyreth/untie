package layout

import "github.com/caelyreth/untie/ir"

func (s *layoutState) layoutRaise(node *ir.Raise, env environment, depth int) (*Fragment, error) {
	body, err := s.layoutNode(node.Body, env, depth)
	if err != nil {
		return nil, err
	}
	metrics := s.fontMetrics(env.Style, env.Variant)
	amount, err := s.signedLength(node.Amount, env.Style, metrics)
	if err != nil {
		return nil, err
	}
	top, err := add(body.Ascent, amount)
	if err != nil {
		return nil, err
	}
	bottom, err := sub(body.Descent, amount)
	if err != nil {
		return nil, err
	}
	baseTop, err := add(body.BaseAscent, amount)
	if err != nil {
		return nil, err
	}
	baseBottom, err := sub(body.BaseDescent, amount)
	if err != nil {
		return nil, err
	}
	ascent := max(0, top)
	descent := max(0, bottom)
	fragment, err := s.newFragment(
		Metrics{Width: body.Width, Ascent: ascent, Descent: descent},
		ir.ClassNormal,
		env.Style.Size,
		s.scale(env.Style),
	)
	if err != nil {
		return nil, err
	}
	fragment.BaseAscent = max(0, baseTop)
	fragment.BaseDescent = max(0, baseBottom)
	fragment.AccentTop = body.AccentTop
	fragment.AccentBottom = body.AccentBottom
	fragment.Italic = body.Italic
	fragment.TextLike = body.TextLike
	if body.HasInkBottom {
		fragment.InkBottom, err = sub(body.InkBottom, amount)
		if err != nil {
			return nil, err
		}
		fragment.HasInkBottom = true
	}
	y, err := sub(0, amount)
	if err != nil {
		return nil, err
	}
	if err := fragment.pushGroup(0, y, body); err != nil {
		return nil, err
	}
	return withClass(fragment, node.Class), nil
}
