package main

import (
	"fmt"
	"time"

	"github.com/caelyreth/untie"
	untiemygo "github.com/caelyreth/untie/contrib/mygo"
	"github.com/caelyreth/untie/display"
	"github.com/caelyreth/untie/geometry"
	"github.com/caelyreth/untie/render"
	"github.com/egoist/mygo/ui"
)

const morphDuration = 320 * time.Millisecond

type morphShapeKind uint8

const (
	morphPath morphShapeKind = iota
	morphRule
)

// morphShape is one drawable command resolved into box coordinates. Keys are
// based on source identity, not item order, so appended TeX keeps existing
// characters attached while their glyphs move.
type morphShape struct {
	key    string
	kind   morphShapeKind
	paint  display.Paint
	bound  geometry.Rect
	path   geometry.Path
	rect   geometry.Rect
	glyph  uint16
	fontID string
}

// formulaMorph interpolates command geometry between two immutable layouts.
// Stable paths move point-by-point; changed glyphs and rules interpolate their
// bounds, so fraction bars and similar properties resize instead of fading.
type formulaMorph struct {
	previousBox *untie.Box
	currentBox  *untie.Box
	previous    []morphShape
	current     []morphShape
	source      string
	started     time.Time
	message     string
	hideErrors  bool
}

func (m *formulaMorph) update(box *untie.Box, sourceText string, now time.Time, hideErrors bool) {
	if box == nil {
		return
	}
	m.message = ""
	if m.currentBox == box && m.source == sourceText && m.hideErrors == hideErrors {
		m.currentBox = box
		m.source = sourceText
		return
	}
	if m.currentBox != nil {
		m.previousBox = m.currentBox
		m.previous = m.current
		m.started = now
	}
	m.currentBox = box
	m.current = extractMorphShapes(box, hideErrors)
	m.source = sourceText
	m.hideErrors = hideErrors
}

func (m *formulaMorph) fail(err error) {
	m.message = "Unable to render formula"
	if err != nil {
		m.message = err.Error()
	}
}

func (m *formulaMorph) reset() {
	*m = formulaMorph{}
}

func (m *formulaMorph) height() float32 {
	if m.currentBox == nil {
		return 150
	}
	return max(90, m.currentBox.Bounds().Height+44)
}

func (m *formulaMorph) contentWidth() float32 {
	if m.currentBox == nil {
		return 1
	}
	return max(1, m.currentBox.Bounds().Width+44)
}

func (m *formulaMorph) draw(p *ui.Painter, rect ui.Rect) {
	if m.currentBox == nil || len(m.current) == 0 {
		drawMorphMessage(p, rect, m.message)
		return
	}
	progress := float32(1)
	if !m.started.IsZero() {
		progress = float32(p.Now().Sub(m.started)) / float32(morphDuration)
		progress = min(1, max(0, progress))
	}
	progress = easeOutCubic(progress)

	fromX, fromY, fromScale := fitTransform(m.previousBox, rect)
	toX, toY, toScale := fitTransform(m.currentBox, rect)
	x := lerp(fromX, toX, progress)
	y := lerp(fromY, toY, progress)
	scale := lerp(fromScale, toScale, progress)

	matches := matchMorphShapes(m.previous, m.current)
	p.Clip(rect, 0, func() {
		for i, next := range m.current {
			if previous := matches[i]; previous >= 0 {
				drawMatchedShape(p, m.previous[previous], next, x, y, scale, progress)
				continue
			}
			drawAppearingShape(p, next, x, y, scale, progress)
		}
	})
	if progress < 1 {
		p.AnimationFrame()
	}
}

func matchMorphShapes(previous, current []morphShape) []int {
	matches := make([]int, len(current))
	for i := range matches {
		matches[i] = -1
	}
	byKey := make(map[string]int, len(previous))
	for i, shape := range previous {
		byKey[shape.key] = i
	}
	used := make([]bool, len(previous))
	for i, next := range current {
		if index, ok := byKey[next.key]; ok {
			matches[i] = index
			used[index] = true
		}
	}
	for i, next := range current {
		if matches[i] >= 0 {
			continue
		}
		best := -1
		var bestDistance float32
		for j, candidate := range previous {
			if used[j] || candidate.kind != next.kind {
				continue
			}
			if next.kind == morphPath && (candidate.fontID != next.fontID || candidate.glyph != next.glyph) {
				continue
			}
			distance := rectCenterDistance(candidate.bound, next.bound)
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

func rectCenterDistance(a, b geometry.Rect) float32 {
	ax, ay := a.X+a.Width/2, a.Y+a.Height/2
	bx, by := b.X+b.Width/2, b.Y+b.Height/2
	dx, dy := ax-bx, ay-by
	return dx*dx + dy*dy
}

func extractMorphShapes(box *untie.Box, hideErrors bool) []morphShape {
	if box == nil {
		return nil
	}
	shapes := make([]morphShape, 0, len(box.Items))
	ordinals := make(map[string]int)
	for _, item := range box.Items {
		if hideErrors && (item.Paint.Role == display.RoleError || item.Paint.Role == display.RolePlaceholder) {
			continue
		}
		var shape morphShape
		shape.paint = item.Paint
		shape.bound = item.Bound
		shape.glyph = item.Glyph
		shape.fontID = item.FontID
		switch item.Kind {
		case display.ItemGlyphPath:
			if item.PathIndex < 0 || item.PathIndex >= len(box.Paths) {
				continue
			}
			path, err := render.ResolvePath(render.Command{
				Kind: render.CommandPath,
				Path: box.Paths[item.PathIndex],
				At:   item.At,
				Em:   item.Em,
			})
			if err != nil {
				continue
			}
			shape.kind = morphPath
			shape.path = path
		case display.ItemRule:
			shape.kind = morphRule
			shape.rect = item.Rect
		default:
			continue
		}
		base := morphIdentity(box, item)
		ordinal := ordinals[base]
		ordinals[base] = ordinal + 1
		shape.key = fmt.Sprintf("%s:%d:%d", base, shape.kind, ordinal)
		shapes = append(shapes, shape)
	}
	return shapes
}

func morphIdentity(box *untie.Box, item display.Item) string {
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

func drawMatchedShape(
	p *ui.Painter,
	previous, next morphShape,
	x, y, scale, progress float32,
) {
	color := untiemygo.ThemePaintResolver(p, next.paint)
	if previous.kind == morphRule && next.kind == morphRule {
		rect := lerpRect(previous.rect, next.rect, progress)
		p.Fill(transformedRect(rect, x, y, scale), color, 0)
		return
	}
	if next.kind == morphRule {
		rect := lerpRect(previous.bound, next.rect, progress)
		p.Fill(transformedRect(rect, x, y, scale), color, 0)
		return
	}
	if next.kind != morphPath {
		return
	}
	if previous.kind == morphPath && compatiblePaths(previous.path, next.path) {
		path := lerpPath(previous.path, next.path, progress)
		fillPath(p, path, x, y, scale, color)
		return
	}
	interpolatedBounds := lerpRect(previous.bound, next.bound, progress)
	path := mapPathBounds(next.path, next.bound, interpolatedBounds)
	fillPath(p, path, x, y, scale, color)
}

func drawAppearingShape(
	p *ui.Painter,
	shape morphShape,
	x, y, scale, progress float32,
) {
	color := untiemygo.ThemePaintResolver(p, shape.paint)
	appear := 0.72 + 0.28*progress
	if shape.kind == morphRule {
		rect := scaleRect(shape.rect, appear)
		p.Fill(transformedRect(rect, x, y, scale), color, 0)
		return
	}
	path := scalePath(shape.path, shape.bound, appear)
	fillPath(p, path, x, y, scale, color)
}

func fitTransform(box *untie.Box, viewport ui.Rect) (x, y, scale float32) {
	if box == nil || viewport.W <= 0 || viewport.H <= 0 {
		return viewport.X, viewport.Y, 1
	}
	bounds := box.Bounds()
	if bounds.Width <= 0 || bounds.Height <= 0 {
		return viewport.X, viewport.Y, 1
	}
	const padding = 22
	x = viewport.X + (viewport.W-bounds.Width)/2 - bounds.X
	y = viewport.Y + padding - bounds.Y
	return x, y, 1
}

func compatiblePaths(a, b geometry.Path) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i].Op != b[i].Op {
			return false
		}
	}
	return true
}

func lerpPath(a, b geometry.Path, t float32) geometry.Path {
	out := make(geometry.Path, len(b))
	for i := range b {
		out[i].Op = b[i].Op
		for j := range b[i].Points {
			out[i].Points[j] = geometry.Point{
				X: lerp(a[i].Points[j].X, b[i].Points[j].X, t),
				Y: lerp(a[i].Points[j].Y, b[i].Points[j].Y, t),
			}
		}
	}
	return out
}

func mapPathBounds(path geometry.Path, from, to geometry.Rect) geometry.Path {
	out := make(geometry.Path, len(path))
	for i, segment := range path {
		out[i].Op = segment.Op
		for j, point := range segment.Points {
			out[i].Points[j] = geometry.Point{
				X: mapRange(point.X, from.X, from.X+from.Width, to.X, to.X+to.Width),
				Y: mapRange(point.Y, from.Y, from.Y+from.Height, to.Y, to.Y+to.Height),
			}
		}
	}
	return out
}

func scalePath(path geometry.Path, bound geometry.Rect, scale float32) geometry.Path {
	center := geometry.Point{X: bound.X + bound.Width/2, Y: bound.Y + bound.Height/2}
	out := make(geometry.Path, len(path))
	for i, segment := range path {
		out[i].Op = segment.Op
		for j, point := range segment.Points {
			out[i].Points[j] = geometry.Point{
				X: center.X + (point.X-center.X)*scale,
				Y: center.Y + (point.Y-center.Y)*scale,
			}
		}
	}
	return out
}

func fillPath(p *ui.Painter, path geometry.Path, x, y, scale float32, color ui.Color) {
	var uiPath ui.Path
	uiPath.Reset()
	for _, segment := range path {
		switch segment.Op {
		case geometry.SegMoveTo:
			point := transformPoint(segment.Points[0], x, y, scale)
			uiPath.MoveTo(point.X, point.Y)
		case geometry.SegLineTo:
			point := transformPoint(segment.Points[0], x, y, scale)
			uiPath.LineTo(point.X, point.Y)
		case geometry.SegQuadTo:
			control := transformPoint(segment.Points[0], x, y, scale)
			point := transformPoint(segment.Points[1], x, y, scale)
			uiPath.QuadTo(control.X, control.Y, point.X, point.Y)
		case geometry.SegCubeTo:
			control1 := transformPoint(segment.Points[0], x, y, scale)
			control2 := transformPoint(segment.Points[1], x, y, scale)
			point := transformPoint(segment.Points[2], x, y, scale)
			uiPath.CubeTo(control1.X, control1.Y, control2.X, control2.Y, point.X, point.Y)
		case geometry.SegClose:
			uiPath.Close()
		}
	}
	p.FillPath(&uiPath, color)
}

func transformPoint(point geometry.Point, x, y, scale float32) geometry.Point {
	return geometry.Point{X: x + point.X*scale, Y: y + point.Y*scale}
}

func transformedRect(rect geometry.Rect, x, y, scale float32) ui.Rect {
	return ui.Rect{
		X: x + rect.X*scale,
		Y: y + rect.Y*scale,
		W: rect.Width * scale,
		H: rect.Height * scale,
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

func mapRange(value, fromMin, fromMax, toMin, toMax float32) float32 {
	fromSpan := fromMax - fromMin
	if fromSpan == 0 {
		return toMin
	}
	return toMin + (value-fromMin)/fromSpan*(toMax-toMin)
}

func lerp(a, b, t float32) float32 {
	return a + (b-a)*t
}

func drawMorphMessage(p *ui.Painter, rect ui.Rect, message string) {
	if message == "" {
		message = "Math appears as the stream reaches it."
	}
	padding := min(float32(28), max(float32(12), rect.H*0.14))
	width := max(0, rect.W-2*padding)
	span := ui.Span{Text: message, Color: p.Theme().TextMuted, Size: 14}
	_, height := p.MeasureText(width, span)
	p.RichText(rect.X+padding, max(rect.Y+padding, rect.Y+(rect.H-height)/2), width, span)
}

func easeOutCubic(t float32) float32 {
	left := 1 - t
	return 1 - left*left*left
}
