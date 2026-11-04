package main

import (
	"time"

	"github.com/caelyreth/untie"
	untiemygo "github.com/caelyreth/untie/contrib/mygo"
	"github.com/caelyreth/untie/render"
	"github.com/egoist/mygo/ui"
)

const morphDuration = 280 * time.Millisecond

// formulaMorph crossfades the previous and current formula boxes while
// interpolating their fitted scale. Boxes are immutable renderer results, so
// drawing can switch targets without disturbing the cache.
type formulaMorph struct {
	previous *untie.Box
	current  *untie.Box
	source   string
	started  time.Time
	message  string
}

func (m *formulaMorph) update(box *untie.Box, source string, now time.Time) {
	if box == nil {
		return
	}
	m.message = ""
	if m.current == box || m.source == source {
		m.current = box
		m.source = source
		return
	}
	if m.current != nil {
		m.previous = m.current
		m.started = now
	}
	m.current = box
	m.source = source
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
	if m.current == nil {
		return 150
	}
	return min(250, max(140, m.current.Bounds().Height+44))
}

func (m *formulaMorph) draw(p *ui.Painter, rect ui.Rect) {
	if m.current == nil {
		drawMorphMessage(p, rect, m.message)
		return
	}
	progress := float32(1)
	if !m.started.IsZero() {
		progress = float32(p.Now().Sub(m.started)) / float32(morphDuration)
		progress = min(1, max(0, progress))
	}
	progress = easeOutCubic(progress)
	if m.previous != nil && progress < 1 {
		drawFormula(p, m.previous, rect, 10+12*progress, (1-progress)*0.72)
	}
	drawFormula(p, m.current, rect, 22-12*progress, 0.28+0.72*progress)
	if progress < 1 {
		p.AnimationFrame()
	}
}

func drawFormula(p *ui.Painter, box *untie.Box, rect ui.Rect, padding, alpha float32) {
	if box == nil || rect.W <= 2*padding || rect.H <= 2*padding {
		return
	}
	_ = untiemygo.DrawBox(p, box, rect, untiemygo.DrawOptions{
		Padding: padding,
		Align:   render.AlignCenter,
		Paint: func(painter *ui.Painter, paint untie.Paint) ui.Color {
			return untiemygo.ThemePaintResolver(painter, paint).Alpha(alpha)
		},
	})
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
