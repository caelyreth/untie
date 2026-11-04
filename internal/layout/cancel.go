package layout

import (
	"github.com/caelyreth/untie/geometry"
	"github.com/caelyreth/untie/ir"
)

const defaultCancelThickness geometry.Em = 3_277 // 0.05 em

func (s *layoutState) layoutCancel(node *ir.Cancel, env environment, depth int) (*Fragment, error) {
	if node.Role == ir.CancelRoleSlashed {
		return s.layoutSlashed(node, env, depth)
	}
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

// layoutSlashed composes the math slash glyph over the body while preserving
// the body's advance when it is wider and reserving the slash width when the
// body is narrow. Both boxes share the same horizontal and vertical center.
func (s *layoutState) layoutSlashed(node *ir.Cancel, env environment, depth int) (*Fragment, error) {
	body, err := s.layoutNode(node.Body, env, depth)
	if err != nil {
		return nil, err
	}
	slash, err := s.layoutGlyphText("/", env, nil)
	if err != nil {
		return nil, err
	}

	width := max(body.Width, slash.Width)
	bodyX, err := sub(width, body.Width)
	if err != nil {
		return nil, err
	}
	bodyX /= 2
	slashX, err := sub(width, slash.Width)
	if err != nil {
		return nil, err
	}
	slashX /= 2
	bodyCenter, err := sub(body.Descent, body.Ascent)
	if err != nil {
		return nil, err
	}
	bodyCenter /= 2
	slashCenter, err := sub(slash.Descent, slash.Ascent)
	if err != nil {
		return nil, err
	}
	slashCenter /= 2
	slashY, err := sub(bodyCenter, slashCenter)
	if err != nil {
		return nil, err
	}
	bodyTop, err := sub(0, body.Ascent)
	if err != nil {
		return nil, err
	}
	slashTop, err := sub(slashY, slash.Ascent)
	if err != nil {
		return nil, err
	}
	slashBottom, err := add(slashY, slash.Descent)
	if err != nil {
		return nil, err
	}
	ascent, err := sub(0, min(bodyTop, slashTop))
	if err != nil {
		return nil, err
	}
	descent := max(body.Descent, slashBottom)

	fragment, err := s.newFragment(
		Metrics{Width: width, Ascent: ascent, Descent: descent},
		ir.ClassNormal,
		env.Style.Size,
		s.scale(env.Style),
	)
	if err != nil {
		return nil, err
	}
	fragment.reserveDraws(2)
	fragment.BaseAscent = body.BaseAscent
	fragment.BaseDescent = body.BaseDescent
	fragment.AccentTop = width / 2
	fragment.AccentBottom = width / 2
	fragment.Italic = body.Italic
	fragment.TextLike = body.TextLike
	if err := fragment.pushGroup(bodyX, 0, body); err != nil {
		return nil, err
	}
	if err := fragment.pushGroup(slashX, slashY, slash); err != nil {
		return nil, err
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
