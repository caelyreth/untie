package untiemygo

import (
	"fmt"

	"github.com/caelyreth/untie"
	"github.com/egoist/mygo/ui"
)

// HitPhase identifies the pointer interaction reported by HitEvent.
type HitPhase uint8

const (
	// HitHover reports pointer movement over the formula.
	HitHover HitPhase = iota
	// HitPress reports the primary pointer press that starts a click.
	HitPress
	// HitClick reports a completed primary click. It uses the pointer
	// position recorded by the preceding HitPress.
	HitClick
)

// String returns a stable lowercase phase name for logs and diagnostics.
func (p HitPhase) String() string {
	switch p {
	case HitHover:
		return "hover"
	case HitPress:
		return "press"
	case HitClick:
		return "click"
	default:
		return fmt.Sprintf("hit-phase-%d", p)
	}
}

// HitEvent describes one pointer interaction with a formula.
//
// Hit and Found come from Box.At after the viewport transform is inverted.
// Point is always the box-local pointer position when the event contains a
// valid pointer location.
type HitEvent struct {
	Source string
	Mode   untie.Mode
	Phase  HitPhase
	Hit    untie.Hit
	Found  bool
	Point  untie.Point
	Caret  untie.CaretStop
	// HasCaret reports whether Caret contains the nearest proven source
	// boundary to Point.
	HasCaret bool
	X        float32
	Y        float32
	Clicks   int
	Mods     ui.Modifiers
}

// HitAt resolves a pointer at x,y relative to viewport to the innermost
// source hit in box.
//
// options must match the options used to draw box. ScaleFit and ScaleNatural
// therefore produce the same hit geometry as their rendering paths.
func HitAt(
	box *untie.Box,
	viewport ui.Rect,
	options DrawOptions,
	x float32,
	y float32,
) (untie.Hit, untie.Point, bool, error) {
	if box == nil {
		return untie.Hit{}, untie.Point{}, false, ErrInvalidBox
	}
	if !finite(x) || !finite(y) || !finite(viewport.X) || !finite(viewport.Y) {
		return untie.Hit{}, untie.Point{}, false, fmt.Errorf(
			"%w: pointer coordinates are not finite", ErrInvalidOptions,
		)
	}
	transform, err := drawTransform(box, viewport, options)
	if err != nil {
		return untie.Hit{}, untie.Point{}, false, err
	}
	if !finite(transform.Scale) || transform.Scale <= 0 {
		return untie.Hit{}, untie.Point{}, false, fmt.Errorf(
			"%w: non-positive draw scale", ErrInvalidOptions,
		)
	}
	windowX := viewport.X + x
	windowY := viewport.Y + y
	point := untie.Point{
		X: (windowX - transform.Offset.X) / transform.Scale,
		Y: (windowY - transform.Offset.Y) / transform.Scale,
	}
	if !finite(point.X) || !finite(point.Y) {
		return untie.Hit{}, untie.Point{}, false, fmt.Errorf(
			"%w: box-local pointer is not finite", ErrInvalidOptions,
		)
	}
	hit, ok := box.At(point)
	return hit, point, ok, nil
}

type pointerHitState struct {
	x      float32
	y      float32
	clicks int
	mods   ui.Modifiers
	valid  bool
}

type pointerHitStateKey struct{}

func wirePointerHits(
	element ui.Element,
	source string,
	mode untie.Mode,
	box *untie.Box,
	width float32,
	height float32,
	options DrawOptions,
	onHit func(HitEvent),
) ui.Element {
	if onHit == nil || !element.Valid() {
		return element
	}
	state := ui.Local(element, pointerHitStateKey{}, func() pointerHitState {
		return pointerHitState{}
	})
	if state == nil {
		return element
	}
	viewport := ui.Rect{W: width, H: height}
	emit := func(phase HitPhase, x, y float32, clicks int, mods ui.Modifiers) {
		hit, point, found, err := HitAt(box, viewport, options, x, y)
		if err != nil {
			return
		}
		caret, hasCaret := box.NearestCaret(point)
		onHit(HitEvent{
			Source:   source,
			Mode:     mode,
			Phase:    phase,
			Hit:      hit,
			Found:    found,
			Point:    point,
			Caret:    caret,
			HasCaret: hasCaret,
			X:        x,
			Y:        y,
			Clicks:   clicks,
			Mods:     mods,
		})
	}
	element = element.HandleInput(func(event ui.InputEvent) bool {
		switch event.Kind {
		case ui.InputPointerMove:
			state.x, state.y = event.X, event.Y
			state.mods, state.valid = event.Mods, true
			emit(HitHover, event.X, event.Y, event.Clicks, event.Mods)
		case ui.InputPointerDown:
			state.x, state.y = event.X, event.Y
			state.clicks, state.mods, state.valid = event.Clicks, event.Mods, true
			emit(HitPress, event.X, event.Y, event.Clicks, event.Mods)
		}
		// Returning false keeps MyGo's click, selection, focus, and context
		// menu behavior intact.
		return false
	})
	return element.OnClick(func() {
		if !state.valid {
			onHit(HitEvent{
				Source: source,
				Mode:   mode,
				Phase:  HitClick,
			})
			return
		}
		emit(HitClick, state.x, state.y, state.clicks, state.mods)
	})
}
