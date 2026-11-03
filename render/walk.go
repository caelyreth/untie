package render

import (
	"errors"
	"fmt"
	"math"

	"github.com/caelyreth/untie/display"
	"github.com/caelyreth/untie/geometry"
)

var (
	// ErrInvalidBox reports malformed input needed by the drawing protocol.
	ErrInvalidBox = errors.New("render: invalid box")
	// ErrInvalidCallback reports a nil Walk callback.
	ErrInvalidCallback = errors.New("render: invalid callback")
	// ErrInvalidCommand reports a command that cannot be resolved as a path.
	ErrInvalidCommand = errors.New("render: invalid command")
)

// CommandKind identifies one visible drawing operation.
type CommandKind uint8

const (
	// CommandPath fills a shared path template placed by At and Em.
	CommandPath CommandKind = iota
	// CommandRule fills Rect.
	CommandRule
)

// Command is one visible drawing operation in box coordinates.
//
// Path aliases Box.Paths storage. Callers must not mutate Path or retain it
// after the Walk callback returns. Rule commands use Rect and leave Path empty.
type Command struct {
	Kind  CommandKind
	Paint display.Paint
	Bound geometry.Rect

	Path display.PathTemplate
	At   geometry.Point
	Em   float32

	Rect geometry.Rect
}

// Walk visits visible path and rule items in box item order.
//
// ItemGlyphRef records are cache metadata and are skipped. Walk does not
// validate the complete box, materialize path geometry, or allocate command
// storage. A callback error is returned unchanged and stops the walk.
func Walk(box *display.Box, visit func(Command) error) error {
	if visit == nil {
		return ErrInvalidCallback
	}
	if box == nil {
		return fmt.Errorf("%w: nil box", ErrInvalidBox)
	}
	for i, item := range box.Items {
		switch item.Kind {
		case display.ItemGlyphPath:
			if item.PathIndex < 0 || item.PathIndex >= len(box.Paths) {
				return fmt.Errorf("%w: item %d path index %d is outside [0,%d)",
					ErrInvalidBox, i, item.PathIndex, len(box.Paths))
			}
			command := Command{
				Kind:  CommandPath,
				Paint: item.Paint,
				Bound: item.Bound,
				Path:  box.Paths[item.PathIndex],
				At:    item.At,
				Em:    item.Em,
			}
			if err := visit(command); err != nil {
				return err
			}
		case display.ItemRule:
			command := Command{
				Kind:  CommandRule,
				Paint: item.Paint,
				Bound: item.Bound,
				Rect:  item.Rect,
			}
			if err := visit(command); err != nil {
				return err
			}
		case display.ItemGlyphRef:
			continue
		default:
			return fmt.Errorf("%w: item %d has unknown kind %d", ErrInvalidBox, i, item.Kind)
		}
	}
	return nil
}

// ResolvePath materializes one path command in box coordinates.
//
// ResolvePath allocates a new geometry.Path. Hosts that can draw directly from
// segments should use Walk and transform points to box coordinates themselves.
// ResolvePath does not mutate the borrowed template.
func ResolvePath(command Command) (geometry.Path, error) {
	if command.Kind != CommandPath {
		return nil, fmt.Errorf("%w: kind %d is not a path", ErrInvalidCommand, command.Kind)
	}
	if len(command.Path.Segments) == 0 {
		return nil, fmt.Errorf("%w: empty path", ErrInvalidCommand)
	}
	if !finite(command.At.X) || !finite(command.At.Y) || !finite(command.Em) || command.Em <= 0 {
		return nil, fmt.Errorf("%w: invalid path placement", ErrInvalidCommand)
	}

	out := make(geometry.Path, len(command.Path.Segments))
	for i, segment := range command.Path.Segments {
		if segment.Op > geometry.SegClose {
			return nil, fmt.Errorf("%w: segment %d has unknown op %d", ErrInvalidCommand, i, segment.Op)
		}
		out[i].Op = segment.Op
		pointCount := segmentPointCount(segment.Op)
		for j := 0; j < pointCount; j++ {
			point := segment.Points[j]
			if !finite(point.X) || !finite(point.Y) {
				return nil, fmt.Errorf("%w: segment %d point %d is non-finite", ErrInvalidCommand, i, j)
			}
			placed := geometry.Point{
				X: command.At.X + point.X*command.Em,
				Y: command.At.Y + point.Y*command.Em,
			}
			if !finite(placed.X) || !finite(placed.Y) {
				return nil, fmt.Errorf("%w: segment %d point %d placement overflows", ErrInvalidCommand, i, j)
			}
			out[i].Points[j] = placed
		}
	}
	return out, nil
}

func segmentPointCount(op geometry.SegOp) int {
	return [...]int{1, 1, 2, 3, 0}[op]
}

func finite(value float32) bool {
	number := float64(value)
	return !math.IsNaN(number) && !math.IsInf(number, 0)
}
