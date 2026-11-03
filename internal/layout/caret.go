package layout

import (
	"fmt"

	"github.com/caelyreth/untie/display"
	"github.com/caelyreth/untie/geometry"
	"github.com/caelyreth/untie/identity"
)

type sourceCaret struct {
	source int
	x      geometry.Em
	y      geometry.Em
	height geometry.Em
	nodeID identity.NodeID
}

func (f *Fragment) appendCaret(caret sourceCaret) {
	if f.caretCount == 0 {
		f.firstCaret = caret
	} else {
		f.moreCarets = append(f.moreCarets, caret)
	}
	f.caretCount++
	f.flatCarets = saturatedAdd(f.flatCarets, 1)
}

func (f *Fragment) appendCaretBoundaries(
	sourceStart, sourceEnd int,
	left, right, top, bottom geometry.Em,
	rtl bool,
	nodeID identity.NodeID,
) {
	if f == nil || sourceStart < 0 || sourceEnd <= sourceStart || right < left || bottom < top {
		return
	}
	startX, endX := left, right
	if rtl {
		startX, endX = right, left
	}
	height, err := sub(bottom, top)
	if err != nil {
		return
	}
	f.appendCaret(sourceCaret{
		source: sourceStart,
		x:      startX,
		y:      top,
		height: height,
		nodeID: nodeID,
	})
	f.appendCaret(sourceCaret{
		source: sourceEnd,
		x:      endX,
		y:      top,
		height: height,
		nodeID: nodeID,
	})
}

func (f *Fragment) caretAt(index int) (*sourceCaret, bool) {
	if f == nil || index < 0 || index >= f.caretCount {
		return nil, false
	}
	if index == 0 {
		return &f.firstCaret, true
	}
	return &f.moreCarets[index-1], true
}

func caretBearingHit(kind display.HitKind) bool {
	switch kind {
	case display.HitAtom, display.HitRadical, display.HitFence:
		return true
	default:
		return false
	}
}

func (s *layoutState) appendFragmentCarets(
	f *flattenState,
	fragment *Fragment,
	x, y geometry.Em,
	fallbackID identity.NodeID,
) error {
	for i := 0; i < fragment.caretCount; i++ {
		if err := f.tick(); err != nil {
			return err
		}
		caret, ok := fragment.caretAt(i)
		if !ok {
			return fmt.Errorf("%w: invalid fragment caret index %d", ErrInvalidInput, i)
		}
		caretX, err := add(x, caret.x)
		if err != nil {
			return err
		}
		caretY, err := add(y, caret.y)
		if err != nil {
			return err
		}
		hostX, err := s.host(caretX)
		if err != nil {
			return err
		}
		hostY, err := s.host(caretY)
		if err != nil {
			return err
		}
		height, err := s.host(caret.height)
		if err != nil {
			return err
		}
		nodeID := caret.nodeID
		if !nodeID.Assigned() {
			nodeID = fallbackID
		}
		if err := f.appendCaret(display.CaretStop{
			Source: caret.source,
			X:      hostX,
			Top:    hostY,
			Bottom: hostY + height,
			NodeID: nodeID,
		}); err != nil {
			return err
		}
	}
	return nil
}

func (f *flattenState) appendCaret(caret display.CaretStop) error {
	if f.offsetX != 0 {
		var err error
		caret.X, err = f.layout.addHost(caret.X, f.offsetX)
		if err != nil {
			return err
		}
	}
	if f.offsetY != 0 {
		var err error
		caret.Top, err = f.layout.addHost(caret.Top, f.offsetY)
		if err != nil {
			return err
		}
		caret.Bottom, err = f.layout.addHost(caret.Bottom, f.offsetY)
		if err != nil {
			return err
		}
	}
	f.box.CaretStops = append(f.box.CaretStops, caret)
	caretLimit := maxInt(f.layout.limits.MaxOutputItems, f.layout.limits.MaxNodes)
	if len(f.box.CaretStops) > caretLimit {
		return fmt.Errorf(
			"%w: caret stop count %d",
			ErrLimit,
			len(f.box.CaretStops),
		)
	}
	return nil
}
