package layout

import (
	"context"
	"fmt"
	"slices"

	"github.com/caelyreth/untie/display"
	"github.com/caelyreth/untie/ir"
)

// Untie does not inherit numeric penalties from TeX or Typst. Both reference
// implementations distinguish break eligibility by atom class but assign no
// relative cost to relation, binary, or explicit opportunities. Zero keeps
// every emitted opportunity available to a host's line breaker; Kind carries
// the semantic distinction.
const (
	breakPenaltyRelation = 0
	breakPenaltyBinary   = 0
	breakPenaltyExplicit = 0
)

type pendingBreak struct {
	kind    display.BreakKind
	penalty int
	valid   bool
}

func (p *pendingBreak) offer(kind display.BreakKind, penalty int) {
	if !p.valid || breakPriority(kind) > breakPriority(p.kind) {
		p.kind = kind
		p.penalty = penalty
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

func neutralBreak(kind ir.BreakKind, penalty int) (display.BreakKind, int) {
	switch kind {
	case ir.BreakBinary:
		return display.BreakBinary, penalty
	case ir.BreakRelation:
		return display.BreakRelation, penalty
	default:
		return display.BreakExplicit, penalty
	}
}

func automaticBreakKind(class ir.MathClass, next ir.MathClass, hasNext bool) (display.BreakKind, int, bool) {
	switch class {
	case ir.ClassBinary:
		// Typst excludes a break immediately before a closing atom. Class
		// normalization already demotes such a binary, but retain the guard so
		// callers of this helper cannot create a non-neutral opportunity.
		if hasNext && next == ir.ClassClosing {
			return 0, 0, false
		}
		return display.BreakBinary, breakPenaltyBinary, true
	case ir.ClassRelation:
		// KaTeX's outer-level rule and Typst's paragraph splitter agree on
		// ordinary relations. Typst additionally avoids dangling relation runs
		// and a relation immediately before a closing atom.
		if hasNext && (next == ir.ClassRelation || next == ir.ClassClosing) {
			return 0, 0, false
		}
		return display.BreakRelation, breakPenaltyRelation, true
	default:
		return 0, 0, false
	}
}

func normalizeFragmentBreaks(breaks []fragmentBreak) []fragmentBreak {
	if len(breaks) < 2 {
		return breaks
	}
	slices.SortStableFunc(breaks, func(a, b fragmentBreak) int {
		if a.x < b.x {
			return -1
		}
		if a.x > b.x {
			return 1
		}
		return 0
	})
	out := breaks[:0]
	for _, candidate := range breaks {
		if len(out) > 0 && out[len(out)-1].x == candidate.x {
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
	out := make([]display.Break, 0, len(breaks))
	for i, breakPoint := range breaks {
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
			X:       x,
			Penalty: breakPoint.penalty,
			Kind:    breakPoint.kind,
		})
	}
	return out, nil
}
