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

// StreamingMathOptions configure one display formula stream.
type StreamingMathOptions struct {
	// TargetSource, when non-empty, reserves the completed formula's natural
	// layout while partial source changes inside it.
	TargetSource string
	// Transition controls duration, partial error visibility, and entering
	// command scaling.
	Transition stream.TransitionOptions
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
	renderer *Renderer
	session  *stream.Session
	draw     DrawOptions
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
	transition := options.Transition
	transition.HideErrors = transition.HideErrors || options.HideErrors
	session, err := stream.NewSession(ctx, r.renderer, stream.SessionOptions{
		Request: untie.RenderRequest{
			Dialect: r.dialect,
			Mode:    untie.Display,
			Style:   r.elementStyle(r.displaySize),
			Parse:   r.parse,
		},
		TargetSource: options.TargetSource,
		Transition:   transition,
	})
	if err != nil {
		return nil, fmt.Errorf("untiemygo: streaming math: %w", err)
	}
	draw := r.draw
	draw.Scale = ScaleNatural
	return &StreamingMath{
		renderer: r,
		session:  session,
		draw:     draw,
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
			// AnimationFrame repaints this callback without rebuilding the
			// element, so sampling the captured build time would freeze every
			// entrance at its first partially faded frame.
			m.drawFrame(painter, frame, rect, painter.Now())
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
	frame *stream.Frame,
	rect ui.Rect,
	now time.Time,
) {
	if painter == nil || frame == nil {
		return
	}
	transform, err := drawTransform(frame.Stage(), rect, m.draw)
	if err != nil {
		return
	}
	var path ui.Path
	var walkErr error
	painter.Clip(rect, 0, func() {
		walkErr = frame.Walk(now, func(command stream.Command) error {
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
	if walkErr == nil && frame.Animating(now) {
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
