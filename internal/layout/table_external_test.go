package layout_test

import (
	"testing"

	"github.com/caelyreth/untie/display"
	"github.com/caelyreth/untie/geometry"
	"github.com/caelyreth/untie/internal/layout"
	"github.com/caelyreth/untie/ir"
	"github.com/caelyreth/untie/source"
)

type externalGlyphSource struct{}

func (externalGlyphSource) Glyph(req layout.GlyphRequest) (layout.Glyph, error) {
	return layout.Glyph{
		ID:            1,
		FontID:        "external-test",
		Text:          req.Text,
		Advance:       geometry.Em(geometry.EmScale / 2),
		Ascent:        geometry.Em(geometry.EmScale * 7 / 10),
		Descent:       geometry.Em(geometry.EmScale / 5),
		ExactGlyphRef: true,
		TextLike:      true,
	}, nil
}

type externalStyleFont struct{}

func (externalStyleFont) MathMetrics(layout.Style, ir.Variant) layout.MathMetrics {
	return layout.MathMetrics{AxisHeight: geometry.Em(geometry.EmScale / 4)}
}

type externalTableLayouter struct{}

func (externalTableLayouter) LayoutTable(req layout.TableRequest) (layout.Fragment, bool) {
	width, ascent, descent := geometry.Em(0), geometry.Em(0), geometry.Em(0)
	for _, row := range req.Rows {
		for _, cell := range row.Cells {
			width = maxEm(width, cell.Width)
			ascent = maxEm(ascent, cell.Ascent)
			descent = maxEm(descent, cell.Descent)
		}
	}
	fragment, err := layout.NewFragment(
		layout.Metrics{Width: width, Ascent: ascent, Descent: descent},
		ir.ClassInner,
		req.Style.Size,
		geometry.Em(geometry.EmScale),
	)
	if err != nil {
		return layout.Fragment{}, false
	}
	x := geometry.Em(0)
	for _, row := range req.Rows {
		for _, cell := range row.Cells {
			cell := cell
			if err := fragment.AddGroup(x, 0, &cell); err != nil {
				return layout.Fragment{}, false
			}
			x += cell.Width
		}
	}
	for _, line := range req.Horizontal {
		if err := fragment.AddRule(0, 0, width, line.Thickness, ir.Paint{}, false); err != nil {
			return layout.Fragment{}, false
		}
	}
	return *fragment, true
}

func TestExternalTableAdapterCanConstructResult(t *testing.T) {
	t.Parallel()

	renderer, err := layout.New(layout.Fonts{
		Glyphs:  externalGlyphSource{},
		Metrics: externalStyleFont{},
	}, layout.Options{Tables: externalTableLayouter{}})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	table := ir.NewTable(source.NewRange(0, 1), ir.TableRow{
		Cells: []ir.Node{ir.NewGlyph(source.NewRange(0, 1), "x")},
	})
	table.Horizontal = []ir.TableLine{{Count: 1, Thickness: mustExternalLength(0.02)}}
	box, err := renderer.Layout(table, ir.Inline, display.Style{Size: 16})
	if err != nil {
		t.Fatalf("Layout() error = %v", err)
	}
	if err := box.Validate(); err != nil {
		t.Fatalf("display validation error = %v", err)
	}
	if len(box.Items) < 2 {
		t.Fatalf("items = %d, want cell glyph and table rule", len(box.Items))
	}
}

func maxEm(a, b geometry.Em) geometry.Em {
	if a > b {
		return a
	}
	return b
}

func mustExternalLength(value float64) ir.Length {
	length, err := ir.EmLength(value)
	if err != nil {
		panic(err)
	}
	return length
}
