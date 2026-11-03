package display

import (
	"errors"
	"fmt"

	"github.com/caelyreth/untie/geometry"
	"github.com/caelyreth/untie/source"
)

// ErrCannotSplit reports a box that cannot be divided into independent inline
// geometry.
var ErrCannotSplit = errors.New("display: cannot split box")

// Split divides an inline box at one of its legal Break opportunities.
//
// The returned boxes have independent item, path, hit, caret, and break
// storage. Left keeps parent coordinates and uses at.X as its width. Right is
// translated so its origin is zero. Hit rectangles are clipped to each side;
// caret stops on the boundary are retained by both sides. Alt and FontUsage are
// cleared because neither whole-formula metadata can be divided soundly.
//
// Constrained display boxes, tagged boxes, unknown break values, and endpoint
// breaks cannot be split. Passing a Break from Box.Breaks is required.
func (b *Box) Split(at Break) (left, right *Box, err error) {
	if b == nil {
		return nil, nil, fmt.Errorf("%w: nil box", ErrCannotSplit)
	}
	if len(b.Lines) != 0 || b.Tag != nil {
		return nil, nil, fmt.Errorf(
			"%w: split supports inline boxes without lines or tags",
			ErrCannotSplit,
		)
	}
	if err := b.Validate(); err != nil {
		return nil, nil, fmt.Errorf("%w: %v", ErrCannotSplit, err)
	}
	if !finite(at.X) || at.X <= 0 || at.X >= b.Width {
		return nil, nil, fmt.Errorf("%w: break x %v is not interior", ErrCannotSplit, at.X)
	}
	if !containsBreak(b.Breaks, at) {
		return nil, nil, fmt.Errorf("%w: break %+v is not owned by the box", ErrCannotSplit, at)
	}

	leftItems, leftPaths := splitItems(b.Items, b.Paths, func(item Item) bool {
		return itemOriginX(item) < at.X
	}, 0)
	rightItems, rightPaths := splitItems(b.Items, b.Paths, func(item Item) bool {
		return itemOriginX(item) >= at.X
	}, -at.X)

	leftHits := splitHits(b.Hits, 0, at.X, 0)
	rightHits := splitHits(b.Hits, at.X, b.Width, -at.X)
	leftCarets := splitCarets(b.CaretStops, 0, at.X, true, 0)
	rightCarets := splitCarets(b.CaretStops, at.X, b.Width, false, -at.X)
	leftBreaks := splitBreaks(b.Breaks, at, true)
	rightBreaks := splitBreaks(b.Breaks, at, false)

	left = &Box{
		Width:       at.X,
		AxisHeight:  b.AxisHeight,
		Items:       leftItems,
		Paths:       leftPaths,
		Hits:        leftHits,
		Breaks:      leftBreaks,
		CaretStops:  leftCarets,
		Diagnostics: cloneDiagnostics(b.Diagnostics),
	}
	right = &Box{
		Width:       b.Width - at.X,
		AxisHeight:  b.AxisHeight,
		Italic:      b.Italic,
		Items:       rightItems,
		Paths:       rightPaths,
		Hits:        rightHits,
		Breaks:      rightBreaks,
		CaretStops:  rightCarets,
		Diagnostics: cloneDiagnostics(b.Diagnostics),
	}
	left.Ascent, left.Descent = splitExtent(left.Items, left.Hits)
	right.Ascent, right.Descent = splitExtent(right.Items, right.Hits)
	left.Overflow = splitOverflow(left)
	right.Overflow = splitOverflow(right)
	left.CaretStops = left.Carets()
	right.CaretStops = right.Carets()
	if err := left.Validate(); err != nil {
		return nil, nil, fmt.Errorf("%w: left: %v", ErrCannotSplit, err)
	}
	if err := right.Validate(); err != nil {
		return nil, nil, fmt.Errorf("%w: right: %v", ErrCannotSplit, err)
	}
	return left, right, nil
}

func containsBreak(breaks []Break, at Break) bool {
	for _, candidate := range breaks {
		if candidate.X == at.X && candidate.Kind == at.Kind {
			return true
		}
	}
	return false
}

func itemOriginX(item Item) float32 {
	if item.Kind == ItemRule {
		return item.Rect.X
	}
	return item.At.X
}

func splitItems(
	items []Item,
	paths []PathTemplate,
	keep func(Item) bool,
	dx float32,
) ([]Item, []PathTemplate) {
	var outItems []Item
	var outPaths []PathTemplate
	pathMap := make(map[int]int)
	for _, item := range items {
		if !keep(item) {
			continue
		}
		item.Bound.X += dx
		if item.Kind == ItemRule {
			item.Rect.X += dx
		} else {
			item.At.X += dx
		}
		if item.PathIndex >= 0 && item.PathIndex < len(paths) {
			mapped, ok := pathMap[item.PathIndex]
			if !ok {
				template := paths[item.PathIndex]
				template.Segments = append([]geometry.Segment(nil), template.Segments...)
				mapped = len(outPaths)
				pathMap[item.PathIndex] = mapped
				outPaths = append(outPaths, template)
			}
			item.PathIndex = mapped
		}
		outItems = append(outItems, item)
	}
	return outItems, outPaths
}

func splitHits(hits []Hit, x0, x1, dx float32) []Hit {
	var out []Hit
	if x1 < x0 {
		return nil
	}
	for _, hit := range hits {
		left := maxFloat(hit.Area.X, x0)
		right := minFloat(hit.Area.X+hit.Area.Width, x1)
		if right < left {
			continue
		}
		out = append(out, Hit{
			Source: hit.Source,
			Area: geometry.Rect{
				X:      left + dx,
				Y:      hit.Area.Y,
				Width:  right - left,
				Height: hit.Area.Height,
			},
			Depth:  hit.Depth,
			Kind:   hit.Kind,
			NodeID: hit.NodeID,
		})
	}
	return out
}

func splitCarets(
	carets []CaretStop,
	x0, x1 float32,
	leftSide bool,
	dx float32,
) []CaretStop {
	var out []CaretStop
	for _, caret := range carets {
		var include bool
		if leftSide {
			include = caret.X <= x1
		} else {
			include = caret.X >= x0
		}
		if !include {
			continue
		}
		caret.X += dx
		out = append(out, caret)
	}
	return out
}

func splitBreaks(breaks []Break, at Break, leftSide bool) []Break {
	var out []Break
	for _, candidate := range breaks {
		if candidate == at {
			continue
		}
		if leftSide {
			if candidate.X < at.X {
				out = append(out, candidate)
			}
			continue
		}
		if candidate.X > at.X {
			candidate.X -= at.X
			out = append(out, candidate)
		}
	}
	return out
}

func splitExtent(items []Item, hits []Hit) (ascent, descent float32) {
	for _, item := range items {
		addSplitExtent(&ascent, &descent, item.Bound)
	}
	for _, hit := range hits {
		addSplitExtent(&ascent, &descent, hit.Area)
	}
	return ascent, descent
}

func addSplitExtent(ascent, descent *float32, rect geometry.Rect) {
	*ascent = maxFloat(*ascent, maxFloat(0, -rect.Y))
	*descent = maxFloat(*descent, maxFloat(0, rect.Y+rect.Height))
}

func splitOverflow(box *Box) float32 {
	extent := inkExtentForItems(box.Items)
	return extent.overflow(box.Width, box.Ascent, box.Descent)
}

func cloneDiagnostics(diagnostics []source.Diagnostic) []source.Diagnostic {
	if len(diagnostics) == 0 {
		return nil
	}
	out := make([]source.Diagnostic, len(diagnostics))
	for i, diagnostic := range diagnostics {
		out[i] = diagnostic.Clone()
	}
	return out
}
