package layout

import (
	"context"
	"fmt"
	"slices"

	"github.com/caelyreth/untie/display"
	"github.com/caelyreth/untie/ir"
)

type pendingBreak struct {
	kind  display.BreakKind
	valid bool
}

func (p *pendingBreak) offer(kind display.BreakKind) {
	if !p.valid || breakPriority(kind) > breakPriority(p.kind) {
		p.kind = kind
		p.valid = true
	}
}

func breakPriority(kind display.BreakKind) int {
	switch kind {
	case display.BreakExplicit:
		return 3
	case display.BreakRelation:
		return 2
	case display.BreakBinary:
		return 1
	default:
		return 0
	}
}

func neutralBreak(kind ir.BreakKind) display.BreakKind {
	switch kind {
	case ir.BreakBinary:
		return display.BreakBinary
	case ir.BreakRelation:
		return display.BreakRelation
	default:
		return display.BreakExplicit
	}
}

func automaticBreakKind(class ir.MathClass, next ir.MathClass, hasNext bool) (display.BreakKind, bool) {
	switch class {
	case ir.ClassBinary:
		// Typst excludes a break immediately before a closing atom. Class
		// normalization already demotes such a binary, but retain the guard so
		// callers of this helper cannot create a non-neutral opportunity.
		if hasNext && next == ir.ClassClosing {
			return 0, false
		}
		return display.BreakBinary, true
	case ir.ClassRelation:
		// KaTeX's outer-level rule and Typst's paragraph splitter agree on
		// ordinary relations. Typst additionally avoids dangling relation runs
		// and a relation immediately before a closing atom.
		if hasNext && (next == ir.ClassRelation || next == ir.ClassClosing) {
			return 0, false
		}
		return display.BreakRelation, true
	default:
		return 0, false
	}
}

func normalizeFragmentBreaks(breaks []fragmentBreak) []fragmentBreak {
	if len(breaks) < 2 {
		return breaks
	}
	slices.SortStableFunc(breaks, func(a, b fragmentBreak) int {
		if a.cut < b.cut {
			return -1
		}
		if a.cut > b.cut {
			return 1
		}
		return 0
	})
	out := breaks[:0]
	for _, candidate := range breaks {
		if len(out) > 0 && out[len(out)-1].cut == candidate.cut {
			last := &out[len(out)-1]
			if breakPriority(candidate.kind) > breakPriority(last.kind) {
				*last = candidate
			}
			continue
		}
		out = append(out, candidate)
	}
	return out
}

func (s *layoutState) displayBreaks(ctx context.Context, breaks []fragmentBreak) ([]display.Break, error) {
	if len(breaks) > s.limits.MaxOutputItems {
		return nil, fmt.Errorf("%w: break point count %d", ErrLimit, len(breaks))
	}
	ordered := slices.Clone(breaks)
	slices.SortStableFunc(ordered, func(a, b fragmentBreak) int {
		if a.x < b.x {
			return -1
		}
		if a.x > b.x {
			return 1
		}
		if a.cut < b.cut {
			return -1
		}
		if a.cut > b.cut {
			return 1
		}
		return 0
	})
	out := make([]display.Break, 0, len(ordered))
	for i, breakPoint := range ordered {
		if i%64 == 0 {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
		}
		x, err := s.host(breakPoint.x)
		if err != nil {
			return nil, err
		}
		out = append(out, display.Break{
			X:    x,
			Kind: breakPoint.kind,
		})
	}
	return out, nil
}
