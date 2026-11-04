package main

import "github.com/egoist/mygo/ui"

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
