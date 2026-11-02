package engine

import (
	"testing"

	"github.com/caelyreth/untie/display"
	"github.com/caelyreth/untie/ir"
	"github.com/caelyreth/untie/source"
)

func TestNewCMSizedDelimiterGeometry(t *testing.T) {
	t.Parallel()

	_, engine := newNewCMEngine(t, source.DefaultLimits())
	tests := []struct {
		source      string
		width       float32
		ascent      float32
		descent     float32
		glyphRefs   int
		outputItems int
	}{
		{source: `\bigl(`, width: 8.367919922, ascent: 15.552001953, descent: 7.552001953, glyphRefs: 1, outputItems: 2},
		{source: `\Bigl(`, width: 10.607910156, ascent: 20.736083984, descent: 12.736083984, glyphRefs: 1, outputItems: 2},
		{source: `\biggl(`, width: 14, ascent: 27.919921875, descent: 19.919921875, glyphRefs: 1, outputItems: 2},
		{source: `\Biggl(`, width: 14, ascent: 28, descent: 20, outputItems: 3},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.source, func(t *testing.T) {
			t.Parallel()
			box := layoutTeX(t, engine, tt.source, ir.Display)
			assertBoxMetrics(t, box, tt.width, tt.ascent, tt.descent)
			if refs := glyphRefs(box); len(refs) != tt.glyphRefs {
				t.Fatalf("%s glyph refs = %d (%+v), want %d", tt.source, len(refs), refs, tt.glyphRefs)
			}
			if len(box.Items) != tt.outputItems {
				t.Fatalf("%s output items = %d, want %d", tt.source, len(box.Items), tt.outputItems)
			}
			var hit *display.Hit
			for i := range box.Hits {
				if box.Hits[i].Kind == display.HitFence && box.Hits[i].Source == source.NewRange(0, len(tt.source)) {
					hit = &box.Hits[i]
					break
				}
			}
			if hit == nil {
				t.Fatalf("%s hits = %+v, want full-source fence hit", tt.source, box.Hits)
			}
		})
	}
}

func TestNewCMSizedDelimiterIgnoresCurrentMathStyleForTarget(t *testing.T) {
	t.Parallel()

	_, engine := newNewCMEngine(t, source.DefaultLimits())
	textBox := layoutTeX(t, engine, `\bigl(`, ir.Display)
	scriptBox := layoutTeX(t, engine, `\scriptstyle\bigl(`, ir.Display)
	assertBoxMetrics(t, scriptBox, textBox.Width, textBox.Ascent, textBox.Descent)
}
