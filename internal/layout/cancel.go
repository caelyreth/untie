package layout

import (
	"github.com/caelyreth/untie/geometry"
	"github.com/caelyreth/untie/ir"
)

const defaultCancelThickness geometry.Em = 3_277 // 0.05 em

func (s *layoutState) layoutCancel(node *ir.Cancel, env environment, depth int) (*Fragment, error) {
	body, err := s.layoutNode(node.Body, env, depth)
	if err != nil {
		return nil, err
	}
	metrics := s.fontMetrics(env.Style, env.Variant)
	thickness, err := s.orMetric(node.Thickness, defaultCancelThickness, env.Style, metrics)
	if err != nil {
		return nil, err
	}

	fragment, err := s.newFragment(body.Metrics, ir.ClassNormal, env.Style.Size, s.scale(env.Style))
	if err != nil {
		return nil, err
	}
	drawCount := 1
	if node.Cross {
		drawCount++
	}
	fragment.reserveDraws(drawCount)
	fragment.BaseAscent = body.BaseAscent
	fragment.BaseDescent = body.BaseDescent
	fragment.AccentTop = body.AccentTop
	fragment.AccentBottom = body.AccentBottom
	fragment.Italic = body.Italic
	fragment.TextLike = body.TextLike
	if err := fragment.pushGroup(0, 0, body); err != nil {
		return nil, err
	}

	if err := s.addCancelLine(fragment, body, thickness, node.Inverted, env.Paint); err != nil {
		return nil, err
	}
	if node.Cross {
		if err := s.addCancelLine(fragment, body, thickness, !node.Inverted, env.Paint); err != nil {
			return nil, err
		}
	}
	return withClass(fragment, node.Class), nil
}

func (s *layoutState) addCancelLine(fragment, body *Fragment, thickness geometry.Em, inverted bool, paint ir.Paint) error {
	height, err := body.Height()
	if err != nil {
		return err
	}
	halfHeight := height / 2
	centerY, err := sub(body.Descent, body.Ascent)
	if err != nil {
		return err
	}
	centerY /= 2
	bottom, err := add(centerY, halfHeight)
	if err != nil {
		return err
	}
	top, err := sub(centerY, halfHeight)
	if err != nil {
		return err
	}
	start := Point{X: 0, Y: bottom}
	end := Point{X: body.Width, Y: top}
	if inverted {
		start.Y, end.Y = end.Y, start.Y
	}
	path, err := lineQuad(start, end, thickness)
	if err != nil {
		return err
	}
	if len(path) == 0 {
		return nil
	}
	return fragment.pushPath(0, 0, &pathDraw{path: path, paint: paint})
}
