package layout

import (
	"fmt"

	"github.com/caelyreth/untie/display"
	"github.com/caelyreth/untie/geometry"
	"github.com/caelyreth/untie/ir"
)

func (s *layoutState) attachNodeHit(fragment *Fragment, node ir.Node, depth int) error {
	if fragment == nil || node == nil || node.NodeBase() == nil {
		return nil
	}
	fragment.ownHit = -1
	if text, ok := node.(*ir.Text); ok && len(text.Clusters) > 0 && fragment.hitCount > 0 {
		// Cluster hits already describe the complete text run.
		return nil
	}
	kind, ok := nodeHitKind(node)
	if !ok {
		return nil
	}
	base := node.NodeBase()
	span := base.Origin
	if span.IsDetached() {
		span = base.Span
	}
	if span.IsDetached() {
		return nil
	}
	if !span.IsValid() {
		return fmt.Errorf("%w: invalid hit source range %+v", ErrInvalidInput, span)
	}
	if depth < 0 || uint64(depth) > uint64(^uint32(0)) {
		return fmt.Errorf("%w: source hit depth %d", ErrInvalidInput, depth)
	}
	height, err := fragment.Height()
	if err != nil {
		return err
	}
	index := fragment.appendHit(sourceHit{
		source: span,
		y:      -fragment.Ascent,
		width:  fragment.Width,
		height: height,
		depth:  uint32(depth),
		kind:   kind,
	})
	fragment.ownHit = index
	return nil
}

func (s *layoutState) attachNodeHitAs(fragment *Fragment, node ir.Node, depth int, kind display.HitKind) error {
	if err := s.attachNodeHit(fragment, node, depth); err != nil {
		return err
	}
	setOwnHitKind(fragment, kind)
	return nil
}

func setOwnHitKind(fragment *Fragment, kind display.HitKind) {
	if fragment == nil {
		return
	}
	hit, ok := fragment.hitAt(fragment.ownHit)
	if !ok {
		return
	}
	hit.kind = kind
}

func nodeHitKind(node ir.Node) (display.HitKind, bool) {
	switch node.(type) {
	case *ir.Glyph, *ir.Text, *ir.Operator, *ir.Space, *ir.ErrorNode, *ir.Placeholder:
		return display.HitAtom, true
	case *ir.Row:
		return display.HitRow, true
	case *ir.Multiline:
		return display.HitRow, true
	case *ir.Scripts:
		return display.HitScript, true
	case *ir.Fraction:
		return display.HitOther, true
	case *ir.Radical:
		return display.HitRadical, true
	case *ir.Fenced:
		return display.HitFence, true
	case *ir.SizedDelimiter:
		return display.HitFence, true
	case *ir.Style, *ir.Color, *ir.Accent, *ir.Line, *ir.Cancel, *ir.Box, *ir.Phantom:
		return display.HitGroup, true
	case *ir.Table:
		return display.HitOther, true
	default:
		return 0, false
	}
}

func (s *layoutState) appendFragmentHits(f *flattenState, fragment *Fragment, x, y geometry.Em) error {
	for i := 0; i < fragment.hitCount; i++ {
		if err := f.tick(); err != nil {
			return err
		}
		hit, ok := fragment.hitAt(i)
		if !ok {
			return fmt.Errorf("%w: invalid fragment hit index %d", ErrInvalidInput, i)
		}
		hitX, err := add(x, hit.x)
		if err != nil {
			return err
		}
		hitY, err := add(y, hit.y)
		if err != nil {
			return err
		}
		hostX, err := s.host(hitX)
		if err != nil {
			return err
		}
		hostY, err := s.host(hitY)
		if err != nil {
			return err
		}
		width, err := s.host(hit.width)
		if err != nil {
			return err
		}
		height, err := s.host(hit.height)
		if err != nil {
			return err
		}
		if err := f.appendHit(display.Hit{
			Source: hit.source,
			Area: geometry.Rect{
				X:      hostX,
				Y:      hostY,
				Width:  width,
				Height: height,
			},
			Depth: int(hit.depth),
			Kind:  hit.kind,
		}); err != nil {
			return err
		}
	}
	return nil
}

func (f *Fragment) appendHit(hit sourceHit) int {
	if f.hitCount == 0 {
		f.firstHit = hit
	} else {
		f.moreHits = append(f.moreHits, hit)
	}
	f.hitCount++
	f.flatHits = saturatedAdd(f.flatHits, 1)
	return f.hitCount - 1
}

func (f *Fragment) hitAt(index int) (*sourceHit, bool) {
	if f == nil || index < 0 || index >= f.hitCount {
		return nil, false
	}
	if index == 0 {
		return &f.firstHit, true
	}
	return &f.moreHits[index-1], true
}
