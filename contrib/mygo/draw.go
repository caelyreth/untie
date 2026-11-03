package untiemygo

import (
	"fmt"

	"github.com/caelyreth/untie"
	"github.com/caelyreth/untie/render"
	"github.com/egoist/mygo/ui"
)

// ScaleMode selects how DrawBox fits a display box into its viewport.
type ScaleMode uint8

const (
	// ScaleFit preserves the compact preview behavior: a box is uniformly
	// scaled down to fit the padded viewport, and scaled up only when
	// DrawOptions.AllowUpscale is true.
	ScaleFit ScaleMode = iota
	// ScaleNatural draws at scale 1, preserving line wrapping and glyph
	// metrics. Ink outside the viewport is clipped.
	ScaleNatural
)

// PaintResolver maps an Untie paint to a MyGo color at draw time.
//
// paint retains both semantic role and explicit-color information, allowing a
// host policy to preserve formula colors, map roles to its own palette, or
// intentionally adapt explicit colors. Resolvers are called for every visible
// command and must only read the painter and paint.
type PaintResolver func(painter *ui.Painter, paint untie.Paint) ui.Color

// ThemePaintResolver resolves explicit colors verbatim and semantic roles
// through the active MyGo theme.
func ThemePaintResolver(painter *ui.Painter, paint untie.Paint) ui.Color {
	var theme *ui.Theme
	if painter != nil {
		theme = painter.Theme()
	}
	return PaintColor(theme, paint)
}

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

// DrawBox draws an existing Untie box into viewport using options.
//
// It does not lay out, cache, or mutate the box. ScaleFit uniformly fits the
// complete Box.Bounds, while ScaleNatural preserves the box's native scale.
// A nil Paint resolver uses ThemePaintResolver. The viewport is the clip
// boundary.
func DrawBox(
	painter *ui.Painter,
	box *untie.Box,
	viewport ui.Rect,
	options DrawOptions,
) error {
	if painter == nil {
		return ErrInvalidPainter
	}
	if box == nil {
		return ErrInvalidBox
	}
	if err := validateDrawOptions(options); err != nil {
		return err
	}
	transform, err := drawTransform(box, viewport, options)
	if err != nil {
		return err
	}
	return drawBoxCommands(painter, box, viewport, transform, options.Paint)
}

// Draw draws box into viewport using this adapter's configured DrawOptions.
func (r *Renderer) Draw(painter *ui.Painter, box *untie.Box, viewport ui.Rect) error {
	if r == nil {
		return ErrInvalidOptions
	}
	return DrawBox(painter, box, viewport, r.draw)
}

func (r *Renderer) drawInline(
	painter *ui.Painter,
	box *untie.Box,
	rect ui.Rect,
	metrics InlineMetricsMode,
) {
	if r == nil || painter == nil || box == nil {
		return
	}
	transform, clip := inlinePlacement(box, rect, metrics)
	_ = drawBoxCommands(painter, box, clip, transform, r.draw.Paint)
}

func drawTransform(box *untie.Box, viewport ui.Rect, options DrawOptions) (render.Transform, error) {
	if options.Scale == ScaleNatural {
		if err := validateNaturalViewport(viewport, options.Padding); err != nil {
			return render.Transform{}, err
		}
		return boundsTransform(box, viewport, 1, options), nil
	}
	transform, err := render.Fit(box, untie.Rect{
		X:      viewport.X,
		Y:      viewport.Y,
		Width:  viewport.W,
		Height: viewport.H,
	}, render.FitOptions{
		Padding: options.Padding,
		Align:   options.Align,
	})
	if err != nil || options.AllowUpscale || transform.Scale <= 1 {
		return transform, err
	}
	return boundsTransform(box, viewport, 1, options), nil
}

func validateNaturalViewport(viewport ui.Rect, padding float32) error {
	if !finite(viewport.X) ||
		!finite(viewport.Y) ||
		!finite(viewport.W) ||
		!finite(viewport.H) ||
		viewport.W <= 0 ||
		viewport.H <= 0 {
		return fmt.Errorf("%w: invalid viewport", render.ErrInvalidViewport)
	}
	if padding >= viewport.W/2 || padding >= viewport.H/2 {
		return fmt.Errorf("%w: padding %v does not leave a positive content area",
			render.ErrInvalidFitOptions, padding)
	}
	return nil
}

func boundsTransform(
	box *untie.Box,
	viewport ui.Rect,
	scale float32,
	options DrawOptions,
) render.Transform {
	bounds := box.Bounds()
	padding := options.Padding
	var offsetX float32
	switch options.Align {
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
	}
}

func drawBoxCommands(
	painter *ui.Painter,
	box *untie.Box,
	clip ui.Rect,
	transform render.Transform,
	resolver PaintResolver,
) error {
	if resolver == nil {
		resolver = ThemePaintResolver
	}
	var path ui.Path
	var walkErr error
	painter.Clip(clip, 0, func() {
		walkErr = render.Walk(box, func(command render.Command) error {
			color := resolver(painter, command.Paint)
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
						return fmt.Errorf("untiemygo: unsupported path segment %d", segment.Op)
					}
				}
				painter.FillPath(&path, color)
			default:
				return fmt.Errorf("untiemygo: unsupported render command %d", command.Kind)
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

func validateDrawOptions(options DrawOptions) error {
	if !finite(options.Padding) || options.Padding < 0 {
		return fmt.Errorf("%w: padding %v is invalid", ErrInvalidOptions, options.Padding)
	}
	if options.Align > render.AlignEnd {
		return fmt.Errorf("%w: unknown alignment %d", ErrInvalidOptions, options.Align)
	}
	if options.Scale > ScaleNatural {
		return fmt.Errorf("%w: unknown scale mode %d", ErrInvalidOptions, options.Scale)
	}
	return nil
}
