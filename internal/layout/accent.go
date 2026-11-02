package layout

import (
	"fmt"
	"strings"

	"github.com/caelyreth/untie/geometry"
	"github.com/caelyreth/untie/ir"
)

const accentShortFall = geometry.Em(geometry.EmScale / 2)

func (s *layoutState) layoutAccent(node *ir.Accent, env environment, depth int) (*Fragment, error) {
	top := node.Position == ir.PositionAbove
	bodyEnv := env
	if top {
		bodyEnv.Style.Cramped = true
	}
	if top && node.Dotless {
		bodyEnv.Features = addFeature(bodyEnv.Features, "dtls")
	}
	body, err := s.layoutNode(node.Body, bodyEnv, depth)
	if err != nil {
		return nil, err
	}

	accentEnv := env
	metrics := s.fontMetrics(env.Style, env.Variant)
	flattenedHeight, err := s.metricAt(metrics.FlattenedAccentBaseHeight, env.Style)
	if err != nil {
		return nil, err
	}
	if top && body.Ascent > flattenedHeight {
		accentEnv.Features = addFeature(accentEnv.Features, "flac")
	}

	var stretch *stretchSpec
	if node.Stretchy {
		shortFall, err := s.metricAt(accentShortFall, env.Style)
		if err != nil {
			return nil, err
		}
		stretch = &stretchSpec{axis: Horizontal, target: body.Width, shortFall: shortFall}
	}
	accent, err := s.layoutAccentMark(node.Accent, accentEnv, depth, stretch)
	if err != nil {
		return nil, err
	}

	bodyAttach := body.AccentTop
	if !top {
		bodyAttach = body.AccentBottom
	}
	accentAttach := accent.AccentTop
	var width, baseX, accentX geometry.Em
	if !node.ExactFrameWidth {
		width = body.Width
		accentX, err = sub(bodyAttach, accentAttach)
		if err != nil {
			return nil, err
		}
	} else {
		pre, err := sub(accentAttach, bodyAttach)
		if err != nil {
			return nil, err
		}
		bodyRemain, err := sub(body.Width, bodyAttach)
		if err != nil {
			return nil, err
		}
		post, err := sub(accent.Width, accentAttach, bodyRemain)
		if err != nil {
			return nil, err
		}
		width, err = sum(max(pre, 0), body.Width, max(post, 0))
		if err != nil {
			return nil, err
		}
		if pre < 0 {
			accentX = -pre
		} else {
			baseX = pre
		}
	}

	var gap, accentTop, bodyTop geometry.Em
	accentHeight, err := accent.Height()
	if err != nil {
		return nil, err
	}
	if top && accent.HasInkBottom && accent.InkBottom < 0 {
		// A MATH accent mark can have its entire ink box above the font
		// baseline. Its signed lower ink edge is then the useful frame bottom.
		accentHeight, err = add(accent.Ascent, accent.InkBottom)
		if err != nil {
			return nil, err
		}
		if accentHeight < 0 {
			return nil, fmt.Errorf("%w: accent has negative frame height", ErrInvalidInput)
		}
	}
	bodyHeight, err := body.Height()
	if err != nil {
		return nil, err
	}
	if top {
		baseHeight, err := s.metricAt(metrics.AccentBaseHeight, env.Style)
		if err != nil {
			return nil, err
		}
		gap, err = sub(-accent.inkBottom(), minEm(body.Ascent, baseHeight))
		if err != nil {
			return nil, err
		}
		accentTop = 0
		bodyTop, err = sum(accentHeight, gap)
		if err != nil {
			return nil, err
		}
	} else {
		gap = -accent.Ascent
		bodyTop = 0
		accentTop, err = sum(bodyHeight, gap)
		if err != nil {
			return nil, err
		}
	}
	height, err := sum(accentHeight, gap, bodyHeight)
	if err != nil {
		return nil, err
	}
	baseline, err := add(bodyTop, body.Ascent)
	if err != nil {
		return nil, err
	}
	ascent, descent := baseline, geometry.Em(0)
	if height >= baseline {
		descent = height - baseline
	} else {
		return nil, fmt.Errorf("%w: accent baseline outside frame", ErrInvalidInput)
	}

	fragment, err := newFragment(Metrics{Width: width, Ascent: ascent, Descent: descent}, ir.ClassNormal, env.Style.Size, s.scale(env.Style))
	if err != nil {
		return nil, err
	}
	fragment.BaseAscent = body.BaseAscent
	fragment.BaseDescent = body.BaseDescent
	fragment.AccentTop = body.AccentTop
	fragment.AccentBottom = body.AccentBottom
	fragment.Italic = body.Italic
	fragment.TextLike = !node.ExactFrameWidth && body.TextLike
	baseY, err := baselineAtTop(bodyTop, body.Ascent, baseline)
	if err != nil {
		return nil, err
	}
	accentY, err := baselineAtTop(accentTop, accent.Ascent, baseline)
	if err != nil {
		return nil, err
	}
	if err := fragment.pushGroup(baseX, baseY, body); err != nil {
		return nil, err
	}
	if err := fragment.pushGroup(accentX, accentY, accent); err != nil {
		return nil, err
	}
	return withClass(fragment, node.Class), nil
}

func (s *layoutState) layoutAccentMark(node ir.Node, env environment, depth int, stretch *stretchSpec) (*Fragment, error) {
	if node == nil {
		return nil, fmt.Errorf("%w: missing accent mark", ErrInvalidInput)
	}
	nodeEnv := s.nodeEnvironment(node, env)
	switch item := node.(type) {
	case *ir.Glyph:
		if err := s.check(depth); err != nil {
			return nil, err
		}
		if err := s.checkTextLimit(node); err != nil {
			return nil, err
		}
		fragment, err := s.layoutGlyphText(item.Text, nodeEnv, stretch)
		if err != nil {
			return nil, err
		}
		return withClass(fragment, item.Class), nil
	case *ir.Operator:
		if err := s.check(depth); err != nil {
			return nil, err
		}
		if err := s.checkTextLimit(node); err != nil {
			return nil, err
		}
		fragment, err := s.layoutGlyphText(item.Text, nodeEnv, stretch)
		if err != nil {
			return nil, err
		}
		return withClass(fragment, item.Class), nil
	default:
		return s.layoutNode(node, env, depth)
	}
}

func (f *Fragment) inkBottom() geometry.Em {
	if f == nil {
		return 0
	}
	return f.InkBottom
}

func addFeature(features []string, feature string) []string {
	for _, existing := range features {
		if strings.EqualFold(existing, feature) {
			return features
		}
	}
	out := make([]string, len(features), len(features)+1)
	copy(out, features)
	return append(out, feature)
}
