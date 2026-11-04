package stream

import (
	"errors"
	"fmt"
	"math"

	"github.com/caelyreth/untie"
	"github.com/caelyreth/untie/display"
	"github.com/caelyreth/untie/geometry"
)

var (
	// ErrInvalidOptions reports unusable stream configuration.
	ErrInvalidOptions = errors.New("stream: invalid options")
	// ErrInvalidTransition reports a nil or malformed display box.
	ErrInvalidTransition = errors.New("stream: invalid transition")
	// ErrInvalidVisitor reports a nil command visitor.
	ErrInvalidVisitor = errors.New("stream: invalid visitor")
	// ErrInvalidProgress reports non-finite or out-of-range sampling input.
	ErrInvalidProgress = errors.New("stream: invalid progress")
)

// Kind identifies one sampled drawing command.
type Kind uint8

const (
	// Path fills a shared path template placed by At and Em.
	Path Kind = iota
	// Rule fills Rect.
	Rule
)

// Command is one streamed drawing command in box coordinates.
//
// Path aliases immutable storage in the current Box. Callers must not mutate
// it or retain it after the Walk callback returns. Opacity ranges from zero to
// one and multiplies the host-resolved paint color.
type Command struct {
	Kind    Kind
	Paint   display.Paint
	Opacity float32
	Bound   geometry.Rect

	Path display.PathTemplate
	At   geometry.Point
	Em   float32

	Rect geometry.Rect
	Snap display.SnapAxis
}
type shape struct {
	key    string
	kind   Kind
	paint  display.Paint
	bound  geometry.Rect
	path   display.PathTemplate
	at     geometry.Point
	em     float32
	rect   geometry.Rect
	snap   display.SnapAxis
	fontID string
	glyph  uint16
}

type actionKind uint8

const (
	actionEnter actionKind = iota
	actionExit
	actionMovePath
	actionMoveRule
	actionReplace
)

type action struct {
	kind     actionKind
	previous int
	current  int
}

// Transition samples the change from one immutable formula layout to another.
type Transition struct {
	previous []shape
	current  []shape
	actions  []action
}

// NewTransition compares previous and current and prepares a reusable display
// transition. A nil previous makes every current command enter.
func NewTransition(previous, current *untie.Box) (*Transition, error) {
	if current == nil {
		return nil, fmt.Errorf("%w: nil current box", ErrInvalidTransition)
	}
	before, err := extractShapes(previous)
	if err != nil {
		return nil, err
	}
	after, err := extractShapes(current)
	if err != nil {
		return nil, err
	}
	t := &Transition{
		previous: before,
		current:  after,
	}
	matches := matchShapes(before, after)
	for i := range after {
		if matches[i] < 0 {
			t.actions = append(t.actions, action{kind: actionEnter, current: i})
			continue
		}
		previousIndex := matches[i]
		switch {
		case before[previousIndex].kind == Path &&
			after[i].kind == Path &&
			sameGlyph(before[previousIndex], after[i]) &&
			samePathTemplate(before[previousIndex].path, after[i].path):
			t.actions = append(t.actions, action{
				kind: actionMovePath, previous: previousIndex, current: i,
			})
		case before[previousIndex].kind == Rule && after[i].kind == Rule:
			t.actions = append(t.actions, action{
				kind: actionMoveRule, previous: previousIndex, current: i,
			})
		default:
			t.actions = append(t.actions, action{
				kind: actionReplace, previous: previousIndex, current: i,
			})
		}
	}
	used := make([]bool, len(before))
	for _, match := range matches {
		if match >= 0 {
			used[match] = true
		}
	}
	for i := range before {
		if !used[i] {
			t.actions = append(t.actions, action{kind: actionExit, previous: i})
		}
	}
	return t, nil
}

// Walk samples commands at normalized progress. appearanceStartScale controls
// the scale of entering and leaving commands. A progress of one walks exactly
// the visible commands of the current box in their original order.
func (t *Transition) Walk(
	progress, appearanceStartScale float32,
	visit func(Command) error,
) error {
	if visit == nil {
		return ErrInvalidVisitor
	}
	if t == nil {
		return nil
	}
	if !finite(progress) || progress < 0 || progress > 1 {
		return fmt.Errorf("%w: progress %v is outside [0,1]",
			ErrInvalidProgress, progress)
	}
	if !finite(appearanceStartScale) ||
		appearanceStartScale < 0 ||
		appearanceStartScale > 1 {
		return fmt.Errorf("%w: appearance start scale %v is outside [0,1]",
			ErrInvalidProgress, appearanceStartScale)
	}
	if progress >= 1 {
		for i := range t.current {
			if err := emit(t.current[i], 1, 1, visit); err != nil {
				return err
			}
		}
		return nil
	}
	for _, a := range t.actions {
		switch a.kind {
		case actionEnter:
			if err := emit(
				t.current[a.current],
				progress,
				lerp(appearanceStartScale, 1, progress),
				visit,
			); err != nil {
				return err
			}
		case actionExit:
			if err := emit(
				t.previous[a.previous],
				1-progress,
				lerp(1, appearanceStartScale, progress),
				visit,
			); err != nil {
				return err
			}
		case actionMovePath:
			before, after := t.previous[a.previous], t.current[a.current]
			command := pathCommand(after, 1)
			command.At = lerpPoint(before.at, after.at, progress)
			command.Em = lerp(before.em, after.em, progress)
			if err := visit(command); err != nil {
				return err
			}
		case actionMoveRule:
			command := ruleCommand(t.current[a.current], 1)
			command.Rect = lerpRect(t.previous[a.previous].rect, command.Rect, progress)
			if err := visit(command); err != nil {
				return err
			}
		case actionReplace:
			if err := emit(
				t.previous[a.previous],
				1-progress,
				lerp(1, appearanceStartScale, progress),
				visit,
			); err != nil {
				return err
			}
			if err := emit(
				t.current[a.current],
				progress,
				lerp(appearanceStartScale, 1, progress),
				visit,
			); err != nil {
				return err
			}
		}
	}
	return nil
}

func extractShapes(box *untie.Box) ([]shape, error) {
	if box == nil {
		return nil, nil
	}
	shapes := make([]shape, 0, len(box.Items))
	ordinals := make(map[string]int, len(box.Items))
	for i, item := range box.Items {
		identity := shapeIdentity(box, item)
		ordinal := ordinals[identity]
		ordinals[identity] = ordinal + 1
		key := fmt.Sprintf("%s:%d:%d", identity, item.Kind, ordinal)
		common := shape{
			key:    key,
			paint:  item.Paint,
			bound:  item.Bound,
			snap:   item.Snap,
			fontID: item.FontID,
			glyph:  item.Glyph,
		}
		switch item.Kind {
		case display.ItemGlyphPath:
			if item.PathIndex < 0 || item.PathIndex >= len(box.Paths) {
				return nil, fmt.Errorf(
					"%w: item %d path index %d is outside [0,%d)",
					ErrInvalidTransition, i, item.PathIndex, len(box.Paths),
				)
			}
			if item.Em <= 0 || !finite(item.Em) ||
				!finite(item.At.X) || !finite(item.At.Y) {
				return nil, fmt.Errorf("%w: item %d has invalid path placement",
					ErrInvalidTransition, i)
			}
			common.kind = Path
			common.path = box.Paths[item.PathIndex]
			common.at = item.At
			common.em = item.Em
		case display.ItemRule:
			common.kind = Rule
			common.rect = item.Rect
		case display.ItemGlyphRef:
			continue
		default:
			return nil, fmt.Errorf("%w: item %d has unknown kind %d",
				ErrInvalidTransition, i, item.Kind)
		}
		shapes = append(shapes, common)
	}
	return shapes, nil
}

func shapeIdentity(box *untie.Box, item display.Item) string {
	if item.NodeID.Assigned() {
		return fmt.Sprintf("node:%d", item.NodeID)
	}
	center := geometry.Point{
		X: item.Bound.X + item.Bound.Width/2,
		Y: item.Bound.Y + item.Bound.Height/2,
	}
	if hit, ok := box.At(center); ok && hit.Source.IsValid() {
		return fmt.Sprintf("source:%d:%d", hit.Source.Start, hit.Source.End)
	}
	if item.Kind == display.ItemGlyphPath || item.Kind == display.ItemGlyphRef {
		return fmt.Sprintf("glyph:%s:%d", item.FontID, item.Glyph)
	}
	return fmt.Sprintf("rule:%.2f:%.2f", item.Rect.Width, item.Rect.Height)
}

func matchShapes(previous, current []shape) []int {
	matches := make([]int, len(current))
	for i := range matches {
		matches[i] = -1
	}
	byKey := make(map[string]int, len(previous))
	for i := range previous {
		byKey[previous[i].key] = i
	}
	used := make([]bool, len(previous))
	for i := range current {
		if index, ok := byKey[current[i].key]; ok {
			matches[i] = index
			used[index] = true
		}
	}
	for i := range current {
		if matches[i] >= 0 {
			continue
		}
		best := -1
		var bestDistance float32
		for j := range previous {
			if used[j] || !fallbackCompatible(previous[j], current[i]) {
				continue
			}
			distance := rectCenterDistance(previous[j].bound, current[i].bound)
			if best < 0 || distance < bestDistance {
				best, bestDistance = j, distance
			}
		}
		if best >= 0 {
			matches[i] = best
			used[best] = true
		}
	}
	return matches
}

func fallbackCompatible(previous, current shape) bool {
	if previous.kind != current.kind {
		return false
	}
	if current.kind == Rule {
		return true
	}
	return sameGlyph(previous, current) &&
		samePathTemplate(previous.path, current.path)
}

func sameGlyph(a, b shape) bool {
	return a.fontID == b.fontID && a.glyph == b.glyph
}

func samePathTemplate(a, b display.PathTemplate) bool {
	if len(a.Segments) != len(b.Segments) {
		return false
	}
	for i := range a.Segments {
		left, right := a.Segments[i], b.Segments[i]
		if left.Op != right.Op {
			return false
		}
		for j := range left.Points {
			if left.Points[j] != right.Points[j] {
				return false
			}
		}
	}
	return true
}

func rectCenterDistance(a, b geometry.Rect) float32 {
	ax, ay := a.X+a.Width/2, a.Y+a.Height/2
	bx, by := b.X+b.Width/2, b.Y+b.Height/2
	dx, dy := ax-bx, ay-by
	return dx*dx + dy*dy
}

func emit(shape shape, opacity, scale float32, visit func(Command) error) error {
	if opacity <= 0 || scale <= 0 {
		return nil
	}
	var command Command
	switch shape.kind {
	case Path:
		command = pathCommand(shape, opacity)
		command.At, command.Em = scalePath(shape, scale)
	case Rule:
		command = ruleCommand(shape, opacity)
		command.Rect = scaleRect(shape.rect, scale)
	default:
		return fmt.Errorf("%w: unknown shape kind %d", ErrInvalidTransition, shape.kind)
	}
	return visit(command)
}

func pathCommand(shape shape, opacity float32) Command {
	return Command{
		Kind:    Path,
		Paint:   shape.paint,
		Opacity: opacity,
		Bound:   shape.bound,
		Path:    shape.path,
		At:      shape.at,
		Em:      shape.em,
	}
}

func ruleCommand(shape shape, opacity float32) Command {
	return Command{
		Kind:    Rule,
		Paint:   shape.paint,
		Opacity: opacity,
		Bound:   shape.bound,
		Rect:    shape.rect,
		Snap:    shape.snap,
	}
}

func scalePath(shape shape, scale float32) (geometry.Point, float32) {
	center := geometry.Point{
		X: shape.bound.X + shape.bound.Width/2,
		Y: shape.bound.Y + shape.bound.Height/2,
	}
	return geometry.Point{
		X: center.X + (shape.at.X-center.X)*scale,
		Y: center.Y + (shape.at.Y-center.Y)*scale,
	}, shape.em * scale
}

func scaleRect(rect geometry.Rect, scale float32) geometry.Rect {
	centerX := rect.X + rect.Width/2
	centerY := rect.Y + rect.Height/2
	width := rect.Width * scale
	height := rect.Height * scale
	return geometry.Rect{
		X: centerX - width/2, Y: centerY - height/2,
		Width: width, Height: height,
	}
}

func lerpRect(a, b geometry.Rect, t float32) geometry.Rect {
	return geometry.Rect{
		X:      lerp(a.X, b.X, t),
		Y:      lerp(a.Y, b.Y, t),
		Width:  lerp(a.Width, b.Width, t),
		Height: lerp(a.Height, b.Height, t),
	}
}

func lerpPoint(a, b geometry.Point, t float32) geometry.Point {
	return geometry.Point{X: lerp(a.X, b.X, t), Y: lerp(a.Y, b.Y, t)}
}

func lerp(a, b, t float32) float32 {
	return a + (b-a)*t
}

func finite(value float32) bool {
	return !float32IsNaN(value) && !float32IsInf(value)
}

func float32IsNaN(value float32) bool {
	return value != value
}

func float32IsInf(value float32) bool {
	return value > math.MaxFloat32 || value < -math.MaxFloat32
}
