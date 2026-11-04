package untiemygo

import (
	"context"
	"fmt"
	"time"

	"github.com/caelyreth/untie"
	"github.com/caelyreth/untie/render"
	"github.com/caelyreth/untie/stream"
	"github.com/egoist/mygo/ui"
)

const (
	// DefaultStreamAnimationDuration is the default display transition time.
	DefaultStreamAnimationDuration = 180 * time.Millisecond
	// DefaultStreamAppearanceScale is the default scale of entering commands.
	DefaultStreamAppearanceScale float32 = 0.72
)

// StreamTiming selects the host-side interpolation curve for streamed math.
type StreamTiming uint8

const (
	// StreamEaseOutCubic starts quickly and settles gently.
	StreamEaseOutCubic StreamTiming = iota
	// StreamEaseInOutCubic accelerates from rest and settles gently.
	StreamEaseInOutCubic
)

func (t StreamTiming) apply(progress float32) float32 {
	switch t {
	case StreamEaseInOutCubic:
		if progress < 0.5 {
			return 4 * progress * progress * progress
		}
		left := -2*progress + 2
		return 1 - left*left*left/2
	default:
		left := 1 - progress
		return 1 - left*left*left
	}
}

// StreamingMathOptions configure one display formula stream.
type StreamingMathOptions struct {
	// TargetSource, when non-empty, reserves the completed formula's natural
	// layout while partial source changes inside it.
	TargetSource string
	// CommitInterval bounds how often source snapshots are laid out. Zero uses
	// stream.DefaultCommitInterval.
	CommitInterval time.Duration
	// AnimationDuration zero uses DefaultStreamAnimationDuration. Negative
	// values are invalid.
	AnimationDuration time.Duration
	// Timing selects the animation curve.
	Timing StreamTiming
	// AppearanceScale zero uses DefaultStreamAppearanceScale. One keeps
	// entering commands at their final scale while retaining the fade.
	AppearanceScale float32
	// HideErrors omits error and placeholder ink from streamed transitions.
	HideErrors bool
}

// StreamingMath adapts one stream.Session to a MyGo display-math element.
//
// A StreamingMath is caller-owned and not concurrency-safe. It naturally sizes
// to the stable completed stage and draws at natural scale; hosts should put it
// in a clipping or horizontal scroll container when the formula is wider than
// the available area.
type StreamingMath struct {
	renderer     *Renderer
	session      *stream.Session
	draw         DrawOptions
	duration     time.Duration
	timing       StreamTiming
	appearance   float32
	hideErrors   bool
	frame        *stream.Frame
	transition   *stream.Transition
	transitionAt time.Time
}

// NewStreamingMath creates a streamed display formula using this adapter's
// dialect, style, parse options, and paint resolver.
func (r *Renderer) NewStreamingMath(
	ctx context.Context,
	options StreamingMathOptions,
) (*StreamingMath, error) {
	if r == nil || r.renderer == nil {
		return nil, ErrInvalidOptions
	}
	if options.AnimationDuration < 0 {
		return nil, fmt.Errorf("%w: negative stream animation duration %v",
			ErrInvalidOptions, options.AnimationDuration)
	}
	if options.AnimationDuration == 0 {
		options.AnimationDuration = DefaultStreamAnimationDuration
	}
	if options.Timing > StreamEaseInOutCubic {
		return nil, fmt.Errorf("%w: unknown stream timing %d",
			ErrInvalidOptions, options.Timing)
	}
	if !finite(options.AppearanceScale) ||
		options.AppearanceScale < 0 ||
		options.AppearanceScale > 1 {
		return nil, fmt.Errorf(
			"%w: stream appearance scale %v is outside [0,1]",
			ErrInvalidOptions, options.AppearanceScale,
		)
	}
	if options.AppearanceScale == 0 {
		options.AppearanceScale = DefaultStreamAppearanceScale
	}
	session, err := stream.NewSession(ctx, r.renderer, stream.SessionOptions{
		Request: untie.RenderRequest{
			Dialect: r.dialect,
			Mode:    untie.Display,
			Style:   r.elementStyle(r.displaySize),
			Parse:   r.parse,
		},
		TargetSource:   options.TargetSource,
		CommitInterval: options.CommitInterval,
	})
	if err != nil {
		return nil, fmt.Errorf("untiemygo: streaming math: %w", err)
	}
	draw := r.draw
	draw.Scale = ScaleNatural
	return &StreamingMath{
		renderer:   r,
		session:    session,
		draw:       draw,
		duration:   options.AnimationDuration,
		timing:     options.Timing,
		appearance: options.AppearanceScale,
		hideErrors: options.HideErrors,
	}, nil
}

// Update offers the latest partial source and returns the committed frame.
func (m *StreamingMath) Update(
	ctx context.Context,
	source string,
	now time.Time,
) (*stream.Frame, bool, error) {
	if m == nil || m.session == nil {
		return nil, false, ErrInvalidOptions
	}
	return m.session.Update(ctx, source, now)
}

// Frame returns the latest committed frame without rendering a new one.
func (m *StreamingMath) Frame() *stream.Frame {
	if m == nil || m.session == nil {
		return nil
	}
	return m.session.Frame()
}

// Element builds a drawable display-math element for the latest source using
// the host frame clock.
func (m *StreamingMath) Element(c *ui.Context, source string) ui.Element {
	if c == nil || !c.Valid() {
		return ui.Element{}
	}
	return m.element(c, source, c.Now())
}

func (m *StreamingMath) element(c *ui.Context, source string, now time.Time) ui.Element {
	if c == nil || !c.Valid() {
		return ui.Element{}
	}
	if m == nil || m.session == nil {
		return fallbackElement(
			c,
			source,
			DefaultDisplaySize,
			untie.RoleError,
			"streaming math renderer unavailable",
		)
	}
	frame, _, err := m.session.Update(context.Background(), source, now)
	if frame == nil {
		return fallbackElement(
			c,
			source,
			m.renderer.displaySize,
			untie.RoleError,
			renderFailureText(nil, err),
		)
	}
	if m.frame != frame || m.transition == nil {
		transition, transitionErr := stream.NewTransition(frame.Previous(), frame.Box())
		if transitionErr != nil {
			return fallbackElement(
				c,
				source,
				m.renderer.displaySize,
				untie.RoleError,
				transitionErr.Error(),
			)
		}
		m.frame = frame
		m.transition = transition
		m.transitionAt = now
	}
	transition := m.transition
	transitionAt := m.transitionAt
	stage := frame.Stage()
	bounds := stage.Bounds()
	padding := m.draw.Padding
	width := max(1, bounds.Width+2*padding)
	height := max(1, bounds.Height+2*padding)
	result := frame.Result()
	semantics := DescribeFormula(source, result.Alt)
	label := accessibleLabel(semantics, m.renderer.localize)
	element := ui.Box(c).
		Width(width).
		Height(height).
		Role(ui.RoleImage).
		Label(label).
		Value(semantics.Value).
		Draw(func(painter *ui.Painter, rect ui.Rect) {
			m.drawFrame(
				painter,
				transition,
				stage,
				transitionAt,
				rect,
				painter.Now(),
			)
		})
	if delay, ok := m.session.NextCommitIn(now); ok {
		// Painter.AnimationFrame repaints this committed layout, but only a
		// view rebuild can call Session.Update and commit newer source.
		c.After(delay)
	}
	return element
}

func (m *StreamingMath) drawFrame(
	painter *ui.Painter,
	transition *stream.Transition,
	stage *untie.Box,
	started time.Time,
	rect ui.Rect,
	now time.Time,
) {
	if painter == nil || transition == nil {
		return
	}
	transform, err := drawTransform(stage, rect, m.draw)
	if err != nil {
		return
	}
	progress := float32(1)
	if m.duration > 0 {
		if !now.After(started) {
			progress = 0
		} else if !now.Before(started.Add(m.duration)) {
			progress = 1
		} else {
			progress = float32(now.Sub(started)) / float32(m.duration)
		}
		progress = m.timing.apply(progress)
	}
	var path ui.Path
	var walkErr error
	painter.Clip(rect, 0, func() {
		walkErr = transition.Walk(progress, m.appearance, func(command stream.Command) error {
			if m.hideErrors &&
				(command.Paint.Role == untie.RoleError ||
					command.Paint.Role == untie.RolePlaceholder) {
				return nil
			}
			color := ThemePaintResolver(painter, command.Paint).Alpha(command.Opacity)
			switch command.Kind {
			case stream.Rule:
				rule := transform.ApplyRect(command.Rect)
				painter.Fill(ui.Rect{
					X: rule.X,
					Y: rule.Y,
					W: rule.Width,
					H: rule.Height,
				}, color, 0)
			case stream.Path:
				path.Reset()
				for _, segment := range command.Path.Segments {
					switch segment.Op {
					case untie.SegMoveTo:
						point := streamPathPoint(command, segment.Points[0], transform)
						path.MoveTo(point.X, point.Y)
					case untie.SegLineTo:
						point := streamPathPoint(command, segment.Points[0], transform)
						path.LineTo(point.X, point.Y)
					case untie.SegQuadTo:
						control := streamPathPoint(command, segment.Points[0], transform)
						point := streamPathPoint(command, segment.Points[1], transform)
						path.QuadTo(control.X, control.Y, point.X, point.Y)
					case untie.SegCubeTo:
						control1 := streamPathPoint(command, segment.Points[0], transform)
						control2 := streamPathPoint(command, segment.Points[1], transform)
						point := streamPathPoint(command, segment.Points[2], transform)
						path.CubeTo(
							control1.X, control1.Y,
							control2.X, control2.Y,
							point.X, point.Y,
						)
					case untie.SegClose:
						path.Close()
					default:
						return fmt.Errorf(
							"untiemygo: unsupported stream path segment %d",
							segment.Op,
						)
					}
				}
				painter.FillPath(&path, color)
			default:
				return fmt.Errorf("untiemygo: unknown stream command kind %d", command.Kind)
			}
			return nil
		})
	})
	if walkErr == nil && progress < 1 {
		painter.AnimationFrame()
	}
}

func streamPathPoint(
	command stream.Command,
	point untie.Point,
	transform render.Transform,
) untie.Point {
	return transform.Apply(untie.Point{
		X: command.At.X + point.X*command.Em,
		Y: command.At.Y + point.Y*command.Em,
	})
}
