// Package testrender converts display lists to SVG for tests and diagnostics.
//
// It is intentionally internal and must never become a runtime dependency of
// the math engine.
package testrender

import (
	"fmt"
	"html"
	"strconv"
	"strings"

	"github.com/caelyreth/untie/display"
	"github.com/caelyreth/untie/geometry"
)

// Options control test SVG output.
type Options struct {
	Padding    float32
	RoleColors map[display.Role]string
	Title      string
}

// SVG returns a self-contained SVG rendering of box.
func SVG(box *display.Box, opts Options) string {
	if box == nil {
		return `<svg xmlns="http://www.w3.org/2000/svg" width="1" height="1" viewBox="0 0 1 1"></svg>`
	}
	if opts.Padding < 0 {
		opts.Padding = 0
	}
	width := box.Width + 2*opts.Padding
	height := box.Ascent + box.Descent + 2*opts.Padding
	if width <= 0 {
		width = 1
	}
	if height <= 0 {
		height = 1
	}

	var body strings.Builder
	offsetX := opts.Padding
	offsetY := opts.Padding + box.Ascent
	for _, item := range box.Items {
		switch item.Kind {
		case display.ItemGlyphPath:
			writePath(&body, item.Path, paint(item.Paint, opts), offsetX, offsetY)
		case display.ItemRule:
			writeRect(&body, item.Rect, paint(item.Paint, opts), offsetX, offsetY)
		case display.ItemGlyphRef:
			// Glyph refs are metadata for hosts with their own glyph cache.
		}
	}

	var out strings.Builder
	fmt.Fprintf(&out, `<svg xmlns="http://www.w3.org/2000/svg" width="%s" height="%s" viewBox="0 0 %s %s">`,
		num(width), num(height), num(width), num(height))
	if opts.Title != "" {
		fmt.Fprintf(&out, "<title>%s</title>", html.EscapeString(opts.Title))
	}
	out.WriteString(body.String())
	out.WriteString("</svg>")
	return out.String()
}

func writePath(b *strings.Builder, path geometry.Path, fill string, dx, dy float32) {
	if len(path) == 0 {
		return
	}
	b.WriteString(`<path fill="`)
	b.WriteString(fill)
	b.WriteString(`" fill-rule="nonzero" d="`)
	for i, seg := range path {
		if i > 0 {
			b.WriteByte(' ')
		}
		switch seg.Op {
		case geometry.SegMoveTo:
			b.WriteByte('M')
			point(b, seg.Points[0], dx, dy)
		case geometry.SegLineTo:
			b.WriteByte('L')
			point(b, seg.Points[0], dx, dy)
		case geometry.SegQuadTo:
			b.WriteString("Q ")
			point(b, seg.Points[0], dx, dy)
			b.WriteByte(' ')
			point(b, seg.Points[1], dx, dy)
		case geometry.SegCubeTo:
			b.WriteString("C ")
			point(b, seg.Points[0], dx, dy)
			b.WriteByte(' ')
			point(b, seg.Points[1], dx, dy)
			b.WriteByte(' ')
			point(b, seg.Points[2], dx, dy)
		case geometry.SegClose:
			b.WriteByte('Z')
		}
	}
	b.WriteString(`"/>`)
}

func point(b *strings.Builder, p geometry.Point, dx, dy float32) {
	b.WriteString(num(p.X + dx))
	b.WriteByte(' ')
	b.WriteString(num(p.Y + dy))
}

func writeRect(b *strings.Builder, r geometry.Rect, fill string, dx, dy float32) {
	if r.Width <= 0 || r.Height <= 0 {
		return
	}
	fmt.Fprintf(b, `<rect x="%s" y="%s" width="%s" height="%s" fill="%s"/>`,
		num(r.X+dx), num(r.Y+dy), num(r.Width), num(r.Height), fill)
}

func paint(p display.Paint, opts Options) string {
	if p.HasExplicit {
		return fmt.Sprintf("#%02x%02x%02x", p.Explicit.R, p.Explicit.G, p.Explicit.B)
	}
	if color := opts.RoleColors[p.Role]; color != "" {
		return color
	}
	switch p.Role {
	case display.RoleError:
		return "#b00020"
	case display.RolePlaceholder:
		return "#777777"
	default:
		return "#000000"
	}
}

func num(v float32) string {
	if v == 0 {
		return "0"
	}
	s := strconv.FormatFloat(float64(v), 'f', 4, 32)
	s = strings.TrimRight(s, "0")
	s = strings.TrimSuffix(s, ".")
	return s
}
