package mygo

import (
	"fmt"

	"github.com/caelyreth/untie"
	"github.com/caelyreth/untie/render"
	"github.com/egoist/mygo/ui"
)

// PaintColor resolves an explicit paint or one of Untie's semantic roles
// through theme. A nil theme uses MyGo's light theme.
func PaintColor(theme *ui.Theme, paint untie.Paint) ui.Color {
	if paint.HasExplicit {
		return ui.RGBA(
			paint.Explicit.R,
			paint.Explicit.G,
			paint.Explicit.B,
			float32(paint.Explicit.A)/255,
		)
	}
	if theme == nil {
		theme = ui.LightTheme()
	}
	switch paint.Role {
	case untie.RoleError:
		return theme.Danger
	case untie.RolePlaceholder:
		return theme.TextMuted
	default:
		return theme.Text
	}
}

// Draw fits box.Bounds into viewport using this adapter's DrawOptions and
// paints every visible render command with theme-resolved colors.
func (r *Renderer) Draw(painter *ui.Painter, box *untie.Box, viewport ui.Rect) error {
	if r == nil {
		return ErrInvalidOptions
	}
	if painter == nil {
		return ErrInvalidPainter
	}
	if box == nil {
		return ErrInvalidBox
	}
	transform, err := r.fit(box, viewport)
	if err != nil {
		return err
	}
	return r.drawWithTransform(painter, box, viewport, transform)
}

func (r *Renderer) drawInline(painter *ui.Painter, box *untie.Box, rect ui.Rect) {
	if r == nil || painter == nil || box == nil {
		return
	}
	bounds := box.Bounds()
	height, _ := inlineExtents(bounds)
	transform := render.Transform{
		Scale: 1,
		Offset: untie.Point{
			X: rect.X - bounds.X,
			Y: rect.Y + height,
		},
	}
	_ = r.drawWithTransform(painter, box, rect, transform)
}

func (r *Renderer) fit(box *untie.Box, viewport ui.Rect) (render.Transform, error) {
	options := render.FitOptions{Padding: r.draw.Padding, Align: r.draw.Align}
	transform, err := render.Fit(box, untie.Rect{
		X:      viewport.X,
		Y:      viewport.Y,
		Width:  viewport.W,
		Height: viewport.H,
	}, options)
	if err != nil || r.draw.AllowUpscale || transform.Scale <= 1 {
		return transform, err
	}
	bounds := box.Bounds()
	padding := r.draw.Padding
	scale := float32(1)
	var offsetX float32
	switch r.draw.Align {
	case render.AlignStart:
		offsetX = viewport.X + padding - bounds.X
	case render.AlignEnd:
		offsetX = viewport.X + viewport.W - padding - (bounds.X + bounds.Width)
	default:
		offsetX = viewport.X + viewport.W/2 - (bounds.X + bounds.Width/2)
	}
	offsetY := viewport.Y + viewport.H/2 - (bounds.Y + bounds.Height/2)
	return render.Transform{
		Scale: scale,
		Offset: untie.Point{
			X: offsetX,
			Y: offsetY,
		},
	}, nil
}

func (r *Renderer) drawWithTransform(
	painter *ui.Painter,
	box *untie.Box,
	clip ui.Rect,
	transform render.Transform,
) error {
	theme := painter.Theme()
	var path ui.Path
	var walkErr error
	painter.Clip(clip, 0, func() {
		walkErr = render.Walk(box, func(command render.Command) error {
			color := PaintColor(theme, command.Paint)
			switch command.Kind {
			case render.CommandRule:
				rect := transform.ApplyRect(command.Rect)
				painter.Fill(ui.Rect{
					X: rect.X,
					Y: rect.Y,
					W: rect.Width,
					H: rect.Height,
				}, color, 0)
			case render.CommandPath:
				path.Reset()
				for _, segment := range command.Path.Segments {
					switch segment.Op {
					case untie.SegMoveTo:
						point := placePathPoint(command, segment.Points[0], transform)
						path.MoveTo(point.X, point.Y)
					case untie.SegLineTo:
						point := placePathPoint(command, segment.Points[0], transform)
						path.LineTo(point.X, point.Y)
					case untie.SegQuadTo:
						control := placePathPoint(command, segment.Points[0], transform)
						point := placePathPoint(command, segment.Points[1], transform)
						path.QuadTo(control.X, control.Y, point.X, point.Y)
					case untie.SegCubeTo:
						control1 := placePathPoint(command, segment.Points[0], transform)
						control2 := placePathPoint(command, segment.Points[1], transform)
						point := placePathPoint(command, segment.Points[2], transform)
						path.CubeTo(
							control1.X, control1.Y,
							control2.X, control2.Y,
							point.X, point.Y,
						)
					case untie.SegClose:
						path.Close()
					default:
						return fmt.Errorf("mygo: unsupported path segment %d", segment.Op)
					}
				}
				painter.FillPath(&path, color)
			default:
				return fmt.Errorf("mygo: unsupported render command %d", command.Kind)
			}
			return nil
		})
	})
	return walkErr
}

func placePathPoint(
	command render.Command,
	point untie.Point,
	transform render.Transform,
) untie.Point {
	return transform.Apply(untie.Point{
		X: command.At.X + point.X*command.Em,
		Y: command.At.Y + point.Y*command.Em,
	})
}
