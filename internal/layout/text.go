package layout

import (
	"fmt"

	"github.com/caelyreth/untie/display"
	"github.com/caelyreth/untie/geometry"
	"github.com/caelyreth/untie/ir"
	"github.com/caelyreth/untie/source"
)

func (s *layoutState) layoutText(node *ir.Text, env environment, depth int) (*Fragment, error) {
	if s.renderer.fonts.Text == nil {
		return nil, fmt.Errorf("%w: text source is not configured", ErrUnsupported)
	}
	metrics := s.fontMetrics(env.Style, env.Variant)
	tracking, err := s.baseLength(node.Tracking, env.Style, metrics)
	if err != nil {
		return nil, err
	}
	if absoluteTracking(node.Tracking.Unit) {
		tracking, err = unscale(tracking, s.scale(env.Style))
		if err != nil {
			return nil, err
		}
	}
	run, err := s.renderer.fonts.Text.ShapeText(TextRequest{
		Context:  s.ctx,
		Text:     node.Text,
		Clusters: node.Clusters,
		Style:    env.Style,
		Variant:  env.Variant,
		Font:     node.Font,
		Tracking: tracking,
		Features: env.Features,
	})
	if err != nil {
		return nil, err
	}
	runes := []rune(node.Text)
	for _, cluster := range run.Clusters {
		if !cluster.Missing {
			continue
		}
		if cluster.RuneStart < 0 || cluster.RuneEnd <= cluster.RuneStart || cluster.RuneEnd > len(runes) {
			return nil, fmt.Errorf("%w: invalid missing text cluster", ErrInvalidInput)
		}
		if !s.recoverVisible {
			return nil, fmt.Errorf(
				"%w: %q",
				ErrMissingGlyph,
				string(runes[cluster.RuneStart:cluster.RuneEnd]),
			)
		}
		span := node.Span
		if len(node.Clusters) > 0 && cluster.Source.IsValid() {
			span = cluster.Source
		}
		s.recordRecoveryDiagnosticText(
			span,
			string(runes[cluster.RuneStart:cluster.RuneEnd]),
			nil,
			source.CodeMissingGlyph,
		)
	}
	return s.layoutTextRun(run, env, true, node, depth)
}

func absoluteTracking(unit ir.LengthUnit) bool {
	return unit == ir.LengthDip || unit == ir.LengthPt
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
	fragment, err := s.layoutTextRun(run, env, len([]rune(node.Text)) > 1, nil, -1)
	if err != nil {
		return nil, err
	}
	return withClass(fragment, node.Class), nil
}

func (s *layoutState) layoutTextRun(
	run TextRun,
	env environment,
	textLike bool,
	textNode *ir.Text,
	depth int,
) (*Fragment, error) {
	if run.Width < 0 || run.Ascent < 0 || run.Descent < 0 {
		return nil, fmt.Errorf("%w: run has negative metrics", ErrInvalidInput)
	}
	if len(run.Clusters) > 0 {
		for i, cluster := range run.Clusters {
			if cluster.RuneStart < 0 || cluster.RuneEnd < cluster.RuneStart ||
				cluster.X < 0 || cluster.Advance < 0 || !cluster.Source.IsValid() {
				return nil, fmt.Errorf("%w: invalid text cluster %d", ErrInvalidInput, i)
			}
		}
		for i, positioned := range run.Glyphs {
			if positioned.Cluster < 0 || positioned.Cluster >= len(run.Clusters) {
				return nil, fmt.Errorf("%w: invalid text glyph cluster %d", ErrInvalidInput, i)
			}
		}
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
	fragment, err := s.newFragment(Metrics{Width: width, Ascent: ascent, Descent: descent}, ir.ClassNormal, env.Style.Size, factor)
	if err != nil {
		return nil, err
	}
	fragment.reserveDraws(len(run.Glyphs))
	fragment.Italic = italic
	fragment.BaseAscent = ascent
	fragment.BaseDescent = descent
	fragment.TextLike = textLike
	var clusterExtents []textClusterExtent
	if textNode != nil && len(textNode.Clusters) > 0 {
		clusterExtents = make([]textClusterExtent, len(run.Clusters))
		for i, cluster := range run.Clusters {
			left, err := scale(cluster.X, factor)
			if err != nil {
				return nil, err
			}
			clusterAdvance, err := scale(cluster.Advance, factor)
			if err != nil {
				return nil, err
			}
			right, err := add(left, clusterAdvance)
			if err != nil {
				return nil, err
			}
			clusterExtents[i] = textClusterExtent{
				source: cluster.Source,
				used:   true,
				left:   left,
				right:  right,
				top:    -ascent,
				bottom: descent,
			}
		}
	}
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
		if positioned.Missing {
			thickness, err := s.metricAt(geometry.Em(geometry.EmScale/64), env.Style)
			if err != nil {
				return nil, err
			}
			placeholder, err := s.placeholderFragment(
				Metrics{
					Width:   draw.Glyph.Advance,
					Ascent:  draw.Glyph.Ascent,
					Descent: draw.Glyph.Descent,
				},
				thickness,
				factor,
				env.Style.Size,
				ir.Paint{Role: ir.PaintError},
			)
			if err != nil {
				return nil, err
			}
			if err := fragment.pushGroup(x, y, placeholder); err != nil {
				return nil, err
			}
			if err := addTextClusterExtent(clusterExtents, positioned.Cluster, y, draw.Glyph); err != nil {
				return nil, err
			}
			continue
		}
		if err := fragment.pushGlyph(x, y, draw); err != nil {
			return nil, err
		}
		if err := addTextClusterExtent(clusterExtents, positioned.Cluster, y, draw.Glyph); err != nil {
			return nil, err
		}
	}
	if len(clusterExtents) > 0 {
		if err := appendTextClusterHits(fragment, clusterExtents, depth); err != nil {
			return nil, err
		}
	}
	return fragment, nil
}

type textClusterExtent struct {
	source      source.Range
	used        bool
	left, right geometry.Em
	top, bottom geometry.Em
}

func addTextClusterExtent(
	extents []textClusterExtent,
	cluster int,
	y geometry.Em,
	glyph Glyph,
) error {
	if len(extents) == 0 {
		return nil
	}
	if cluster < 0 || cluster >= len(extents) {
		return fmt.Errorf("%w: invalid text cluster extent %d", ErrInvalidInput, cluster)
	}
	originY, err := add(y, glyph.OriginY)
	if err != nil {
		return err
	}
	top, err := sub(originY, glyph.Ascent)
	if err != nil {
		return err
	}
	bottom, err := add(originY, glyph.Descent)
	if err != nil {
		return err
	}
	extent := &extents[cluster]
	if !extent.used {
		extent.used = true
		extent.top = top
		extent.bottom = bottom
		return nil
	}
	extent.top = min(extent.top, top)
	extent.bottom = max(extent.bottom, bottom)
	return nil
}

func appendTextClusterHits(fragment *Fragment, extents []textClusterExtent, depth int) error {
	if depth < 0 || uint64(depth) > uint64(^uint32(0)) {
		return fmt.Errorf("%w: text cluster hit depth %d", ErrInvalidInput, depth)
	}
	groups := make([]textClusterExtent, 0, len(extents))
	indexes := make(map[source.Range]int, len(extents))
	for _, extent := range extents {
		if !extent.used || extent.source.IsDetached() || !extent.source.IsValid() {
			continue
		}
		if index, exists := indexes[extent.source]; exists {
			group := &groups[index]
			group.left = min(group.left, extent.left)
			group.right = max(group.right, extent.right)
			group.top = min(group.top, extent.top)
			group.bottom = max(group.bottom, extent.bottom)
			continue
		}
		indexes[extent.source] = len(groups)
		groups = append(groups, extent)
	}
	for _, group := range groups {
		width, err := sub(group.right, group.left)
		if err != nil {
			return err
		}
		height, err := sub(group.bottom, group.top)
		if err != nil {
			return err
		}
		fragment.appendHit(sourceHit{
			source: group.source,
			x:      group.left,
			y:      group.top,
			width:  width,
			height: height,
			depth:  uint32(depth),
			kind:   display.HitAtom,
		})
	}
	return nil
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
	if node.SetFontSize {
		scale, err := fontScale(node.FontSize)
		if err != nil {
			return nil, err
		}
		child.Style.SizeScale = scale
	}
	fragment, err := s.layoutNode(node.Body, child, depth)
	if err != nil {
		return nil, err
	}
	return withClass(fragment, node.Class), nil
}

func fontScale(value ir.Length) (geometry.Em, error) {
	if value.Unit != ir.LengthPercent || value.Value <= 0 {
		return 0, fmt.Errorf("%w: invalid font size %+v", ErrInvalidInput, value)
	}
	const percentScale int64 = 100 * 1_000_000
	scale, err := geometry.EmFromParts(value.Value, percentScale)
	if err != nil {
		return 0, fmt.Errorf("%w: font size: %w", ErrInvalidInput, err)
	}
	if scale <= 0 {
		return 0, fmt.Errorf("%w: font size rounds to zero", ErrInvalidInput)
	}
	return scale, nil
}
