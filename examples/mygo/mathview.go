package main

import (
	"github.com/caelyreth/untie"
	"github.com/egoist/mygo/ui"
)

func drawMath(p *ui.Painter, r ui.Rect, box *untie.Box) {
	padding := min(float32(24), max(float32(2), min(r.W, r.H)*0.08))
	content := ui.Rect{
		X: r.X + padding,
		Y: r.Y + padding,
		W: max(0, r.W-2*padding),
		H: max(0, r.H-2*padding),
	}
	placement := fitMath(content, box)

	theme := p.Theme()
	p.Clip(content, 0, func() {
		for _, item := range box.Items {
			color := itemColor(theme, item.Paint)
			switch item.Kind {
			case untie.ItemGlyphPath:
				path := mathPath(item.Path, placement.originX, placement.originY, placement.scale)
				p.FillPath(&path, color)
			case untie.ItemRule:
				p.Fill(ui.Rect{
					X: placement.originX + item.Rect.X*placement.scale,
					Y: placement.originY + item.Rect.Y*placement.scale,
					W: item.Rect.Width * placement.scale,
					H: item.Rect.Height * placement.scale,
				}, color, 0)
			case untie.ItemGlyphRef:
				// The example paints paths directly; a production host may
				// resolve these records through its own glyph cache instead.
			}
		}
	})
}

type mathPlacement struct {
	scale   float32
	originX float32
	originY float32
}

func fitMath(content ui.Rect, box *untie.Box) mathPlacement {
	bounds := box.Bounds()
	scale := float32(1)
	if bounds.Width > content.W && bounds.Width > 0 {
		scale = min(scale, content.W/bounds.Width)
	}
	if bounds.Height > content.H && bounds.Height > 0 {
		scale = min(scale, content.H/bounds.Height)
	}
	scale = max(0, scale)
	scaledWidth := bounds.Width * scale
	scaledHeight := bounds.Height * scale
	originX := content.X + (content.W-scaledWidth)/2
	originX -= bounds.X * scale
	originY := content.Y + (content.H-scaledHeight)/2
	originY -= bounds.Y * scale
	return mathPlacement{scale: scale, originX: originX, originY: originY}
}

func mathPath(path untie.Path, originX, originY, scale float32) ui.Path {
	var out ui.Path
	for _, segment := range path {
		switch segment.Op {
		case untie.SegMoveTo:
			out.MoveTo(
				originX+segment.Points[0].X*scale,
				originY+segment.Points[0].Y*scale,
			)
		case untie.SegLineTo:
			out.LineTo(
				originX+segment.Points[0].X*scale,
				originY+segment.Points[0].Y*scale,
			)
		case untie.SegQuadTo:
			out.QuadTo(
				originX+segment.Points[0].X*scale, originY+segment.Points[0].Y*scale,
				originX+segment.Points[1].X*scale, originY+segment.Points[1].Y*scale,
			)
		case untie.SegCubeTo:
			out.CubeTo(
				originX+segment.Points[0].X*scale, originY+segment.Points[0].Y*scale,
				originX+segment.Points[1].X*scale, originY+segment.Points[1].Y*scale,
				originX+segment.Points[2].X*scale, originY+segment.Points[2].Y*scale,
			)
		case untie.SegClose:
			out.Close()
		}
	}
	return out
}

func itemColor(theme *ui.Theme, paint untie.Paint) ui.Color {
	if paint.HasExplicit {
		return ui.RGBA(
			paint.Explicit.R,
			paint.Explicit.G,
			paint.Explicit.B,
			float32(paint.Explicit.A)/255,
		)
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

func drawMessage(p *ui.Painter, r ui.Rect, message string, color ui.Color) {
	if message == "" {
		message = "Enter a formula."
	}
	padding := min(float32(32), max(float32(4), min(r.W, r.H)*0.1))
	width := max(0, r.W-2*padding)
	span := ui.Span{Text: message, Color: color, Size: 14}
	_, height := p.MeasureText(width, span)
	p.RichText(r.X+padding, max(r.Y+padding, r.Y+(r.H-height)/2), width, span)
}
