package layout

import (
	"fmt"

	"github.com/caelyreth/untie/geometry"
	"github.com/caelyreth/untie/ir"
)

const defaultBoxBorderWidth geometry.Em = 3_277 // 0.05 em

func (s *layoutState) layoutBox(node *ir.Box, env environment, depth int) (*Fragment, error) {
	body, err := s.layoutNode(node.Body, env, depth)
	if err != nil {
		return nil, err
	}
	metrics := s.fontMetrics(env.Style, env.Variant)
	padding := make([]geometry.Em, len(node.Padding))
	for i, value := range node.Padding {
		padding[i], err = s.length(value, env.Style, metrics)
		if err != nil {
			return nil, fmt.Errorf("%w: box padding %d: %w", ErrInvalidInput, i, err)
		}
	}
	width, err := sum(body.Width, padding[1], padding[3])
	if err != nil {
		return nil, err
	}
	ascent, err := add(body.Ascent, padding[0])
	if err != nil {
		return nil, err
	}
	descent, err := add(body.Descent, padding[2])
	if err != nil {
		return nil, err
	}
	height, err := add(ascent, descent)
	if err != nil {
		return nil, err
	}
	radius, err := s.length(node.Radius, env.Style, metrics)
	if err != nil {
		return nil, fmt.Errorf("%w: box radius: %w", ErrInvalidInput, err)
	}

	fragment, err := s.newFragment(Metrics{Width: width, Ascent: ascent, Descent: descent}, ir.ClassNormal, env.Style.Size, s.scale(env.Style))
	if err != nil {
		return nil, err
	}
	drawCount := 1
	if node.HasFill {
		drawCount++
	}
	if node.HasBorder {
		drawCount++
	}
	fragment.reserveDraws(drawCount)
	fragment.BaseAscent = body.BaseAscent
	fragment.BaseDescent = body.BaseDescent
	fragment.AccentTop = body.AccentTop
	fragment.AccentBottom = body.AccentBottom
	fragment.Italic = body.Italic
	fragment.TextLike = body.TextLike

	top := -ascent
	if node.HasFill {
		if radius == 0 {
			if err := fragment.pushRule(0, top, &ruleDraw{
				width:     width,
				thickness: height,
				paint:     node.Fill,
			}); err != nil {
				return nil, err
			}
		} else {
			path, err := roundedRectPath(0, top, width, height, radius, false)
			if err != nil {
				return nil, err
			}
			if err := fragment.pushPath(0, 0, &pathDraw{path: path, paint: node.Fill}); err != nil {
				return nil, err
			}
		}
	}
	if node.HasBorder {
		borderWidth := node.BorderWidth
		if borderWidth.IsZero() {
			var err error
			borderWidth, err = ir.EmLength(float64(defaultBoxBorderWidth) / float64(geometry.EmScale))
			if err != nil {
				return nil, fmt.Errorf("%w: default box border width: %w", ErrInvalidInput, err)
			}
		}
		thickness, err := s.length(borderWidth, env.Style, metrics)
		if err != nil {
			return nil, fmt.Errorf("%w: box border width: %w", ErrInvalidInput, err)
		}
		if thickness > 0 {
			path, err := roundedRectRingPath(0, top, width, height, radius, thickness)
			if err != nil {
				return nil, err
			}
			if err := fragment.pushPath(0, 0, &pathDraw{path: path, paint: node.Border}); err != nil {
				return nil, err
			}
		}
	}
	bodyY, err := baselineAtTop(padding[0], body.Ascent, ascent)
	if err != nil {
		return nil, err
	}
	if err := fragment.pushGroup(padding[3], bodyY, body); err != nil {
		return nil, err
	}
	return withClass(fragment, node.Class), nil
}

func roundedRectRingPath(x, y, width, height, radius, thickness geometry.Em) (Path, error) {
	outer, err := roundedRectPath(x, y, width, height, radius, false)
	if err != nil {
		return nil, err
	}
	maxInset := minEm(width, height) / 2
	if thickness >= maxInset {
		return outer, nil
	}
	inner, err := roundedRectPath(
		x+thickness,
		y+thickness,
		width-2*thickness,
		height-2*thickness,
		max(radius-thickness, 0),
		true,
	)
	if err != nil {
		return nil, err
	}
	return append(outer, inner...), nil
}
