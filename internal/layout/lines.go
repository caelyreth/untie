package layout

import (
	"fmt"
	"math"

	"github.com/caelyreth/untie/display"
	"github.com/caelyreth/untie/geometry"
)

type lineBoundary struct {
	itemIndex int
	x         geometry.Em
}

type linePlan struct {
	start int
	end   int
}

func (s *layoutState) flattenDisplayLines(root *Fragment) (*display.Box, error) {
	if err := s.ctx.Err(); err != nil {
		return nil, err
	}
	width, err := s.host(root.Width)
	if err != nil {
		return nil, err
	}
	if width <= s.request.MaxWidth || len(root.breaks) == 0 {
		offsetX, err := s.displayLineOffset(width)
		if err != nil {
			return nil, err
		}
		return s.flattenSingleDisplayLine(root, offsetX)
	}

	boundaries, ok, err := s.displayLineBoundaries(root)
	if err != nil {
		return nil, err
	}
	if !ok {
		offsetX, err := s.displayLineOffset(width)
		if err != nil {
			return nil, err
		}
		return s.flattenSingleDisplayLine(root, offsetX)
	}
	plans, err := s.greedyLinePlans(boundaries)
	if err != nil {
		return nil, err
	}
	if len(plans) == 1 {
		offsetX, err := s.displayLineOffset(width)
		if err != nil {
			return nil, err
		}
		return s.flattenSingleDisplayLine(root, offsetX)
	}
	return s.flattenPlannedDisplayLines(root, boundaries, plans)
}

func (s *layoutState) flattenSingleDisplayLine(root *Fragment, offsetX float32) (*display.Box, error) {
	parent, err := s.newConstrainedDisplayBox()
	if err != nil {
		return nil, err
	}
	width, ascent, descent, italic, err := s.fragmentHostMetrics(root)
	if err != nil {
		return nil, err
	}
	line := display.LineBox{
		Baseline: 0,
		OffsetX:  offsetX,
		Width:    width,
		Ascent:   ascent,
		Descent:  descent,
		Items:    display.ItemRange{Start: 0},
		Hits:     display.HitRange{Start: 0},
	}
	flatten := newFlattenState(s, parent, offsetX, 0)
	if err := flatten.visit(root, 0, 0); err != nil {
		return nil, err
	}
	line.Items.Count = len(parent.Items) - line.Items.Start
	line.Hits.Count = len(parent.Hits) - line.Hits.Start
	line.Overflow, err = s.localRangeOverflow(parent.Items, line, parent.Width)
	if err != nil {
		return nil, err
	}
	parent.Lines = []display.LineBox{line}
	parent.Ascent = ascent
	parent.Descent = descent
	parent.Italic = italic
	if err := s.finishConstrainedDisplayBox(parent); err != nil {
		return nil, err
	}
	return parent, nil
}

func (s *layoutState) flattenPlannedDisplayLines(
	root *Fragment,
	boundaries []lineBoundary,
	plans []linePlan,
) (*display.Box, error) {
	parent, err := s.newConstrainedDisplayBox()
	if err != nil {
		return nil, err
	}
	parent.Lines = make([]display.LineBox, 0, len(plans))
	var baseline float32
	for i, plan := range plans {
		if i%64 == 0 {
			if err := s.ctx.Err(); err != nil {
				return nil, err
			}
		}
		start, end := boundaries[plan.start], boundaries[plan.end]
		segment, err := s.lineFragment(root, start, end)
		if err != nil {
			return nil, err
		}
		width, ascent, descent, italic, err := s.fragmentHostMetrics(segment)
		if err != nil {
			return nil, err
		}
		if i > 0 {
			previous := parent.Lines[i-1]
			step, err := s.addHost(previous.Descent, ascent)
			if err != nil {
				return nil, err
			}
			baseline, err = s.addHost(previous.Baseline, step)
			if err != nil {
				return nil, err
			}
		}
		offsetX, err := s.displayLineOffset(width)
		if err != nil {
			return nil, err
		}
		line := display.LineBox{
			Baseline: baseline,
			OffsetX:  offsetX,
			Width:    width,
			Ascent:   ascent,
			Descent:  descent,
			Items:    display.ItemRange{Start: len(parent.Items)},
			Hits:     display.HitRange{Start: len(parent.Hits)},
		}
		flatten := newFlattenState(s, parent, offsetX, baseline)
		if err := flatten.visit(segment, 0, 0); err != nil {
			return nil, err
		}
		if err := s.appendLineOwnHit(parent, segment, line); err != nil {
			return nil, err
		}
		line.Items.Count = len(parent.Items) - line.Items.Start
		line.Hits.Count = len(parent.Hits) - line.Hits.Start
		line.Overflow, err = s.localRangeOverflow(parent.Items, line, parent.Width)
		if err != nil {
			return nil, err
		}
		parent.Lines = append(parent.Lines, line)
		parent.Italic = italic

		top, err := s.subHost(baseline, ascent)
		if err != nil {
			return nil, err
		}
		bottom, err := s.addHost(baseline, descent)
		if err != nil {
			return nil, err
		}
		if top < 0 {
			parent.Ascent = maxFloat(parent.Ascent, -top)
		}
		parent.Descent = maxFloat(parent.Descent, bottom)
	}
	if err := s.finishConstrainedDisplayBox(parent); err != nil {
		return nil, err
	}
	return parent, nil
}

func (s *layoutState) newConstrainedDisplayBox() (*display.Box, error) {
	if _, err := s.addHost(s.request.MaxWidth, 0); err != nil {
		return nil, err
	}
	axis, err := s.metricAt(s.metrics.AxisHeight, s.rootStyle)
	if err != nil {
		return nil, err
	}
	axisHeight, err := s.host(axis)
	if err != nil {
		return nil, err
	}
	return &display.Box{
		Width:      s.request.MaxWidth,
		AxisHeight: axisHeight,
	}, nil
}

func (s *layoutState) fragmentHostMetrics(fragment *Fragment) (float32, float32, float32, float32, error) {
	if fragment == nil {
		return 0, 0, 0, 0, fmt.Errorf("%w: nil line fragment", ErrInvalidInput)
	}
	width, err := s.host(fragment.Width)
	if err != nil {
		return 0, 0, 0, 0, err
	}
	ascent, err := s.host(fragment.Ascent)
	if err != nil {
		return 0, 0, 0, 0, err
	}
	descent, err := s.host(fragment.Descent)
	if err != nil {
		return 0, 0, 0, 0, err
	}
	italic, err := s.host(fragment.Italic)
	if err != nil {
		return 0, 0, 0, 0, err
	}
	return width, ascent, descent, italic, nil
}

func (s *layoutState) appendLineOwnHit(parent *display.Box, root *Fragment, line display.LineBox) error {
	if root.ownHit < 0 || root.ownHit >= len(root.hits) {
		return nil
	}
	hit := root.hits[root.ownHit]
	height, err := s.addHost(line.Ascent, line.Descent)
	if err != nil {
		return err
	}
	y, err := s.subHost(line.Baseline, line.Ascent)
	if err != nil {
		return err
	}
	parent.Hits = append(parent.Hits, display.Hit{
		Source: hit.source,
		Area: geometry.Rect{
			X:      line.OffsetX,
			Y:      y,
			Width:  line.Width,
			Height: height,
		},
		Depth: hit.depth,
		Kind:  hit.kind,
	})
	if len(parent.Hits) > s.limits.MaxNodes {
		return fmt.Errorf("%w: parent source hit count %d", ErrLimit, len(parent.Hits))
	}
	return nil
}

func (s *layoutState) localRangeOverflow(
	items []display.Item,
	line display.LineBox,
	parentWidth float32,
) (float32, error) {
	var overflow float32
	for i := line.Items.Start; i < line.Items.End(); i++ {
		if (i-line.Items.Start)%256 == 0 {
			if err := s.ctx.Err(); err != nil {
				return 0, err
			}
		}
		bound := items[i].Bound
		x, err := s.subHost(bound.X, line.OffsetX)
		if err != nil {
			return 0, err
		}
		y, err := s.subHost(bound.Y, line.Baseline)
		if err != nil {
			return 0, err
		}
		overflow = maxFloat(overflow, maxFloat(0, -x))
		overflow = maxFloat(overflow, maxFloat(0, x+bound.Width-line.Width))
		overflow = maxFloat(overflow, maxFloat(0, -y-line.Ascent))
		overflow = maxFloat(overflow, maxFloat(0, y+bound.Height-line.Descent))
	}
	return maxFloat(overflow, maxFloat(0, line.Width-parentWidth)), nil
}

func (s *layoutState) finishConstrainedDisplayBox(parent *display.Box) error {
	overflow, err := s.displayItemsOverflow(parent.Items, parent.Width, parent.Ascent, parent.Descent)
	if err != nil {
		return err
	}
	for i, line := range parent.Lines {
		if i%64 == 0 {
			if err := s.ctx.Err(); err != nil {
				return err
			}
		}
		overflow = maxFloat(overflow, maxFloat(0, line.Width-parent.Width))
	}
	parent.Overflow = overflow
	return nil
}

func (s *layoutState) displayLineBoundaries(root *Fragment) ([]lineBoundary, bool, error) {
	if root == nil {
		return nil, false, fmt.Errorf("%w: nil line root", ErrInvalidInput)
	}
	boundaries := []lineBoundary{{itemIndex: 0, x: 0}}
	previousCut := 0
	for i, candidate := range root.breaks {
		if i%64 == 0 {
			if err := s.ctx.Err(); err != nil {
				return nil, false, err
			}
		}
		if candidate.cut <= previousCut || candidate.cut > len(root.items) {
			return nil, false, nil
		}
		previousCut = candidate.cut
		if candidate.cut == len(root.items) || candidate.x <= 0 || candidate.x >= root.Width {
			continue
		}
		if candidate.x < boundaries[len(boundaries)-1].x {
			// Negative spacing can make the public X positions non-monotonic.
			// The paint stream no longer proves a safe horizontal partition.
			return nil, false, nil
		}
		boundaries = append(boundaries, lineBoundary{
			itemIndex: candidate.cut,
			x:         candidate.x,
		})
	}
	if root.Width < boundaries[len(boundaries)-1].x {
		return nil, false, nil
	}
	boundaries = append(boundaries, lineBoundary{itemIndex: len(root.items), x: root.Width})
	return boundaries, true, nil
}

func (s *layoutState) greedyLinePlans(boundaries []lineBoundary) ([]linePlan, error) {
	if len(boundaries) < 2 {
		return nil, fmt.Errorf("%w: missing line boundary", ErrInvalidInput)
	}
	plans := make([]linePlan, 0, len(boundaries)-1)
	candidates := 0
	for start := 0; start < len(boundaries)-1; {
		if len(plans)%64 == 0 {
			if err := s.ctx.Err(); err != nil {
				return nil, err
			}
		}
		best := -1
		var bestWidth float32
		for end := start + 1; end < len(boundaries); end++ {
			candidates++
			if candidates%64 == 0 {
				if err := s.ctx.Err(); err != nil {
					return nil, err
				}
			}
			width, err := sub(boundaries[end].x, boundaries[start].x)
			if err != nil {
				return nil, err
			}
			hostWidth, err := s.host(width)
			if err != nil {
				return nil, err
			}
			if hostWidth <= s.request.MaxWidth {
				// Boundaries are scanned in source order. Keeping the first
				// equal-width candidate makes the lowest logical cut the stable
				// tie-break without inventing a public cost.
				if best < 0 || hostWidth > bestWidth {
					best = end
					bestWidth = hostWidth
				}
			}
			if hostWidth > s.request.MaxWidth {
				break
			}
			if end == len(boundaries)-1 {
				best = end
				bestWidth = hostWidth
				break
			}
		}
		if best < 0 {
			// No legal piece fits. Keep the longest non-empty legal piece and
			// report this line as overflow.
			best = start + 1
		}
		plans = append(plans, linePlan{start: start, end: best})
		start = best
	}
	return plans, nil
}

func (s *layoutState) lineFragment(root *Fragment, start, end lineBoundary) (*Fragment, error) {
	if root == nil || start.itemIndex < 0 || end.itemIndex < start.itemIndex ||
		end.itemIndex > len(root.items) {
		return nil, fmt.Errorf("%w: invalid line fragment bounds", ErrInvalidInput)
	}
	if end.x < start.x {
		return nil, fmt.Errorf("%w: reversed line fragment bounds", ErrInvalidInput)
	}
	width, err := sub(end.x, start.x)
	if err != nil {
		return nil, err
	}
	line := emptyFragment(root.Size, root.Scale)
	line.Width = width
	line.Class = root.Class
	line.TextLike = true

	minTop, maxBottom := geometry.Em(0), geometry.Em(0)
	haveExtent := false
	for i := start.itemIndex; i < end.itemIndex; i++ {
		if (i-start.itemIndex)%64 == 0 {
			if err := s.ctx.Err(); err != nil {
				return nil, err
			}
		}
		item := root.items[i]
		x, err := sub(item.x, start.x)
		if err != nil {
			return nil, err
		}
		item.x = x
		line.items = append(line.items, item)
		top, bottom, italic, err := s.placedExtents(item)
		if err != nil {
			return nil, err
		}
		if !haveExtent || top < minTop {
			minTop = top
		}
		if !haveExtent || bottom > maxBottom {
			maxBottom = bottom
		}
		haveExtent = true
		if italic != 0 {
			line.Italic = italic
		}
	}
	if haveExtent {
		line.Ascent = max(0, -minTop)
		line.Descent = max(0, maxBottom)
	}
	line.BaseAscent = line.Ascent
	line.BaseDescent = line.Descent
	line.AccentTop = line.Width / 2
	line.AccentBottom = line.Width / 2
	return line, nil
}

func (s *layoutState) placedExtents(item placedDraw) (geometry.Em, geometry.Em, geometry.Em, error) {
	switch item.kind {
	case drawGroup:
		if item.group == nil {
			return 0, 0, 0, fmt.Errorf("%w: nil line group", ErrInvalidInput)
		}
		top, err := sub(item.y, item.group.Ascent)
		if err != nil {
			return 0, 0, 0, err
		}
		bottom, err := add(item.y, item.group.Descent)
		return top, bottom, item.group.Italic, err
	case drawGlyph:
		if item.glyph == nil {
			return 0, 0, 0, fmt.Errorf("%w: nil line glyph", ErrInvalidInput)
		}
		top, err := sub(item.y, item.glyph.Glyph.Ascent)
		if err != nil {
			return 0, 0, 0, err
		}
		bottom, err := add(item.y, item.glyph.Glyph.Descent)
		return top, bottom, item.glyph.Glyph.Italic, err
	case drawRule:
		if item.rule == nil {
			return 0, 0, 0, fmt.Errorf("%w: nil line rule", ErrInvalidInput)
		}
		extent := item.rule.thickness
		if item.rule.vertical {
			extent = item.rule.width
		}
		bottom, err := add(item.y, extent)
		return item.y, bottom, 0, err
	case drawPath:
		if item.path == nil || len(item.path.path) == 0 {
			return 0, 0, 0, fmt.Errorf("%w: empty line path", ErrInvalidInput)
		}
		minY, maxY := geometry.Em(0), geometry.Em(0)
		have := false
		for segmentIndex, segment := range item.path.path {
			if segmentIndex%256 == 0 {
				if err := s.ctx.Err(); err != nil {
					return 0, 0, 0, err
				}
			}
			points := segmentPointCountForBounds(segment.Op)
			for i := 0; i < points; i++ {
				y, err := add(item.y, segment.Points[i].Y)
				if err != nil {
					return 0, 0, 0, err
				}
				if !have || y < minY {
					minY = y
				}
				if !have || y > maxY {
					maxY = y
				}
				have = true
			}
		}
		return minY, maxY, 0, nil
	default:
		return 0, 0, 0, fmt.Errorf("%w: unknown line draw kind %d", ErrInvalidInput, item.kind)
	}
}

func segmentPointCountForBounds(op geometry.SegOp) int {
	points, _ := segmentPointCount(op)
	return points
}

func (s *layoutState) displayItemsOverflow(items []display.Item, width, ascent, descent float32) (float32, error) {
	var overflow float32
	for i, item := range items {
		if i%256 == 0 {
			if err := s.ctx.Err(); err != nil {
				return 0, err
			}
		}
		bound := item.Bound
		left := maxFloat(0, -bound.X)
		right := maxFloat(0, bound.X+bound.Width-width)
		top := maxFloat(0, -bound.Y-ascent)
		bottom := maxFloat(0, bound.Y+bound.Height-descent)
		overflow = maxFloat(overflow, maxFloat(maxFloat(left, right), maxFloat(top, bottom)))
	}
	return overflow, nil
}

func (s *layoutState) displayLineOffset(lineWidth float32) (float32, error) {
	return s.displayLineOffsetWithin(lineWidth, s.request.MaxWidth)
}

func (s *layoutState) displayLineOffsetWithin(lineWidth, frameWidth float32) (float32, error) {
	if lineWidth >= frameWidth {
		return 0, nil
	}
	remaining, err := s.subHost(frameWidth, lineWidth)
	if err != nil {
		return 0, err
	}
	switch s.request.Align {
	case display.AlignStart:
		return 0, nil
	case display.AlignCenter:
		return remaining / 2, nil
	case display.AlignEnd:
		return remaining, nil
	default:
		return 0, fmt.Errorf("%w: invalid alignment", ErrInvalidInput)
	}
}

func (s *layoutState) addHost(a, b float32) (float32, error) {
	sum := float64(a) + float64(b)
	if math.IsNaN(sum) || math.IsInf(sum, 0) || math.Abs(sum) > float64(math.MaxFloat32) {
		return 0, fmt.Errorf("%w: line host coordinate overflow", ErrLimit)
	}
	if float64(s.limits.MaxDimension) > 0 && math.Abs(sum) > float64(s.limits.MaxDimension) {
		return 0, fmt.Errorf("%w: line host coordinate %v", ErrLimit, sum)
	}
	return float32(sum), nil
}

func (s *layoutState) subHost(a, b float32) (float32, error) {
	return s.addHost(a, -b)
}
