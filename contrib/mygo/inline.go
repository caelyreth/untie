package untiemygo

import (
	"github.com/caelyreth/untie"
	"github.com/caelyreth/untie/render"
	"github.com/egoist/mygo/ui"
)

// InlineMetricsMode selects which vertical metrics an inline object reports to
// MyGo text layout.
type InlineMetricsMode uint8

const (
	// InlineIncludeOverflow includes certified overflow in width, height, and
	// depth. It preserves the compact preview behavior.
	InlineIncludeOverflow InlineMetricsMode = iota
	// InlineIgnoreOverflow reports nominal Box.Width, Box.Ascent, and
	// Box.Descent. Certified overflow remains drawable inside the surrounding
	// text clip instead of increasing line height.
	InlineIgnoreOverflow
)

// InlineOptions configure Markdown inline math.
type InlineOptions struct {
	Metrics InlineMetricsMode
}

func (o InlineOptions) validate() bool {
	return o.Metrics == InlineIncludeOverflow || o.Metrics == InlineIgnoreOverflow
}

func inlineObjectMetrics(
	box *untie.Box,
	mode InlineMetricsMode,
) (width, height, depth float32) {
	if box == nil {
		return 0, 0, 0
	}
	if mode == InlineIgnoreOverflow {
		return max(0, box.Width), max(0, box.Ascent), max(0, box.Descent)
	}
	bounds := box.Bounds()
	height, depth = inlineExtents(bounds)
	return max(0, bounds.Width), height, depth
}

func inlinePlacement(
	box *untie.Box,
	rect ui.Rect,
	mode InlineMetricsMode,
) (render.Transform, ui.Rect) {
	if mode == InlineIgnoreOverflow {
		overflow := max(0, box.Overflow)
		return render.Transform{
			Scale: 1,
			Offset: untie.Point{
				X: rect.X,
				Y: rect.Y + box.Ascent,
			},
		}, ui.Rect{
			X: rect.X - overflow,
			Y: rect.Y - overflow,
			W: rect.W + 2*overflow,
			H: rect.H + 2*overflow,
		}
	}
	bounds := box.Bounds()
	height, _ := inlineExtents(bounds)
	return render.Transform{
		Scale: 1,
		Offset: untie.Point{
			X: rect.X - bounds.X,
			Y: rect.Y + height,
		},
	}, rect
}
