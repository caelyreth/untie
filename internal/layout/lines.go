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
		return s.constrainedFallback(root)
	}

	boundaries, ok, err := s.displayLineBoundaries(root)
	if err != nil {
		return nil, err
	}
	if !ok {
		return s.constrainedFallback(root)
	}
	plans, err := s.greedyLinePlans(boundaries)
	if err != nil {
		return nil, err
	}
	if len(plans) == 1 {
		return s.constrainedFallback(root)
	}

	lines := make([]display.LineBox, 0, len(plans))
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
		line, err := s.flattenFragment(segment)
		if err != nil {
			return nil, err
		}
		line.Alt = display.Alt{}
		line.Overflow = maxFloat(line.Overflow, line.Width-s.request.MaxWidth)
		line.Overflow = maxFloat(0, line.Overflow)
		if err := s.appendLineOwnHit(root, line); err != nil {
			return nil, err
		}
		if len(line.Hits) > s.limits.MaxNodes {
			return nil, fmt.Errorf("%w: line source hit count %d", ErrLimit, len(line.Hits))
		}
		lines = append(lines, display.LineBox{Box: line})
	}
	return s.assembleDisplayLines(lines)
}

func (s *layoutState) constrainedFallback(root *Fragment) (*display.Box, error) {
	base, err := s.flattenFragment(root)
	if err != nil {
		return nil, err
	}
	return s.constrainedFallbackBox(base)
}

func (s *layoutState) constrainedFallbackBox(base *display.Box) (*display.Box, error) {
	line, err := s.singleDisplayLine(base, s.request.MaxWidth)
	if err != nil {
		return nil, err
	}
	return s.assembleDisplayLines([]display.LineBox{{Box: line}})
}

func (s *layoutState) singleDisplayLine(base *display.Box, maxWidth float32) (*display.Box, error) {
	if base == nil {
		return nil, fmt.Errorf("%w: nil fallback box", ErrInvalidInput)
	}
	if err := s.ctx.Err(); err != nil {
		return nil, err
	}
	line := *base
	line.Breaks = nil
	line.Alt = display.Alt{}
	line.Lines = nil
	if len(base.Breaks) > 0 {
		line.Breaks = make([]display.Break, len(base.Breaks))
		for i := range base.Breaks {
			if i%64 == 0 {
				if err := s.ctx.Err(); err != nil {
					return nil, err
				}
			}
			line.Breaks[i] = base.Breaks[i]
		}
	}
	line.Overflow = maxFloat(line.Overflow, line.Width-maxWidth)
	line.Overflow = maxFloat(0, line.Overflow)
	return &line, nil
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

func (s *layoutState) appendLineOwnHit(root *Fragment, line *display.Box) error {
	if root.ownHit < 0 || root.ownHit >= len(root.hits) {
		return nil
	}
	hit := root.hits[root.ownHit]
	height, err := s.addHost(line.Ascent, line.Descent)
	if err != nil {
		return err
	}
	line.Hits = append(line.Hits, display.Hit{
		Source: hit.source,
		Area: geometry.Rect{
			Y:      -line.Ascent,
			Width:  line.Width,
			Height: height,
		},
		Depth: hit.depth,
		Kind:  hit.kind,
	})
	return nil
}

func (s *layoutState) assembleDisplayLines(lines []display.LineBox) (*display.Box, error) {
	if len(lines) == 0 {
		return nil, fmt.Errorf("%w: display lines are empty", ErrInvalidInput)
	}
	if _, err := s.addHost(s.request.MaxWidth, 0); err != nil {
		return nil, err
	}
	if lines[0].Box == nil {
		return nil, fmt.Errorf("%w: nil first display line", ErrInvalidInput)
	}
	maxOutput := int64(s.limits.MaxOutputItems)
	physicalItems := int64(len(lines))
	physicalPathSegments := int64(0)
	parent := &display.Box{
		Width:      s.request.MaxWidth,
		AxisHeight: lines[0].Box.AxisHeight,
		Lines:      make([]display.LineBox, len(lines)),
	}
	var baseline float32
	for i := range lines {
		if i%64 == 0 {
			if err := s.ctx.Err(); err != nil {
				return nil, err
			}
		}
		if lines[i].Box == nil {
			return nil, fmt.Errorf("%w: nil display line", ErrInvalidInput)
		}
		if i > 0 {
			previous := &lines[i-1]
			step, err := s.addHost(previous.Box.Descent, lines[i].Box.Ascent)
			if err != nil {
				return nil, err
			}
			baseline, err = s.addHost(previous.Baseline, step)
			if err != nil {
				return nil, err
			}
		}
		offsetX, err := s.displayLineOffset(lines[i].Box.Width)
		if err != nil {
			return nil, err
		}
		lines[i].Baseline = baseline
		lines[i].OffsetX = offsetX
		parent.Lines[i] = lines[i]
		// Italic correction belongs to the final horizontal endpoint, which is
		// the end of the last line for a vertically assembled display box.
		parent.Italic = lines[i].Box.Italic

		top, err := s.subHost(baseline, lines[i].Box.Ascent)
		if err != nil {
			return nil, err
		}
		bottom, err := s.addHost(baseline, lines[i].Box.Descent)
		if err != nil {
			return nil, err
		}
		if top < 0 {
			parent.Ascent = maxFloat(parent.Ascent, -top)
		}
		parent.Descent = maxFloat(parent.Descent, bottom)

		for itemIndex, item := range lines[i].Box.Items {
			if itemIndex%64 == 0 {
				if err := s.ctx.Err(); err != nil {
					return nil, err
				}
			}
			translated, err := s.translateItem(item, offsetX, baseline)
			if err != nil {
				return nil, err
			}
			parent.Items = append(parent.Items, translated)
			physicalItems += 2
			physicalPathSegments += 2 * int64(len(item.Path))
			if physicalItems > maxOutput {
				return nil, fmt.Errorf("%w: display output item count", ErrLimit)
			}
			if physicalPathSegments > int64(s.limits.MaxPathSegments) {
				return nil, fmt.Errorf("%w: display path segment count", ErrLimit)
			}
		}
		for hitIndex, hit := range lines[i].Box.Hits {
			if hitIndex%64 == 0 {
				if err := s.ctx.Err(); err != nil {
					return nil, err
				}
			}
			translated := hit
			translated.Area.X, err = s.addHost(hit.Area.X, offsetX)
			if err != nil {
				return nil, err
			}
			translated.Area.Y, err = s.addHost(hit.Area.Y, baseline)
			if err != nil {
				return nil, err
			}
			parent.Hits = append(parent.Hits, translated)
			if len(parent.Hits) > s.limits.MaxNodes {
				return nil, fmt.Errorf("%w: parent source hit count %d", ErrLimit, len(parent.Hits))
			}
		}
	}
	overflow, err := s.displayItemsOverflow(parent.Items, parent.Width, parent.Ascent, parent.Descent)
	if err != nil {
		return nil, err
	}
	for i, line := range parent.Lines {
		if i%64 == 0 {
			if err := s.ctx.Err(); err != nil {
				return nil, err
			}
		}
		overflow = maxFloat(overflow, maxFloat(0, line.Box.Width-parent.Width))
	}
	parent.Overflow = overflow
	return parent, nil
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

func (s *layoutState) translateItem(item display.Item, dx, dy float32) (display.Item, error) {
	var err error
	item.Bound.X, err = s.addHost(item.Bound.X, dx)
	if err != nil {
		return display.Item{}, err
	}
	item.Bound.Y, err = s.addHost(item.Bound.Y, dy)
	if err != nil {
		return display.Item{}, err
	}
	item.Rect.X, err = s.addHost(item.Rect.X, dx)
	if err != nil {
		return display.Item{}, err
	}
	item.Rect.Y, err = s.addHost(item.Rect.Y, dy)
	if err != nil {
		return display.Item{}, err
	}
	item.At.X, err = s.addHost(item.At.X, dx)
	if err != nil {
		return display.Item{}, err
	}
	item.At.Y, err = s.addHost(item.At.Y, dy)
	if err != nil {
		return display.Item{}, err
	}
	if len(item.Path) > 0 {
		item.Path = append([]geometry.Segment(nil), item.Path...)
		for i := range item.Path {
			if i%256 == 0 {
				if err := s.ctx.Err(); err != nil {
					return display.Item{}, err
				}
			}
			for j := range item.Path[i].Points {
				item.Path[i].Points[j].X, err = s.addHost(item.Path[i].Points[j].X, dx)
				if err != nil {
					return display.Item{}, err
				}
				item.Path[i].Points[j].Y, err = s.addHost(item.Path[i].Points[j].Y, dy)
				if err != nil {
					return display.Item{}, err
				}
			}
		}
	}
	return item, nil
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
