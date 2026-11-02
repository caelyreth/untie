package engine

import (
	"testing"

	"github.com/caelyreth/untie/display"
	"github.com/caelyreth/untie/internal/tex"
	"github.com/caelyreth/untie/ir"
	"github.com/caelyreth/untie/source"
)

func TestDisplayOperatorSpacingScalesLinearly(t *testing.T) {
	t.Parallel()

	_, layoutEngine := newNewCMEngine(t, source.DefaultLimits())
	for _, input := range []string{
		`S=\int\mathcal{M}`,
		`S=\int d^4x\sqrt{-g}\,\mathcal{M}`,
	} {
		t.Run(input, func(t *testing.T) {
			parsed := tex.Parse(input, ir.Display, tex.Options{})
			if len(parsed.Diagnostics) != 0 {
				t.Fatalf("diagnostics = %+v", parsed.Diagnostics)
			}
			var widths [3]float32
			for i, size := range []float32{16, 32, 64} {
				box, err := layoutEngine.Layout(t.Context(), parsed.Root, ir.Display, display.Style{Size: size})
				if err != nil {
					t.Fatalf("size %v: %v", size, err)
				}
				widths[i] = box.Width
			}
			assertDouble(t, widths[1], widths[0], "16 to 32 DIP")
			assertDouble(t, widths[2], widths[1], "32 to 64 DIP")
		})
	}
}

func assertDouble(t *testing.T, doubled, original float32, label string) {
	t.Helper()
	ratio := doubled / original
	if ratio < 1.998 || ratio > 2.002 {
		t.Fatalf("%s width ratio = %v (%v / %v)", label, ratio, doubled, original)
	}
}
