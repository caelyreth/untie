package engine

import (
	"testing"

	"github.com/caelyreth/untie/display"
	"github.com/caelyreth/untie/internal/tex"
	"github.com/caelyreth/untie/internal/typst"
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

func TestIntegralSubscriptParity(t *testing.T) {
	t.Parallel()

	_, layoutEngine := newNewCMEngine(t, source.DefaultLimits())
	texRoot := tex.Parse(`\int_{\mathcal M}`, ir.Display, tex.Options{}).Root
	typstRoot := typst.Parse(`integral_(cal(M))`, ir.Display, typst.Options{}).Root
	texBox, err := layoutEngine.Layout(t.Context(), texRoot, ir.Display, display.Style{Size: 32})
	if err != nil {
		t.Fatal(err)
	}
	typstBox, err := layoutEngine.Layout(t.Context(), typstRoot, ir.Display, display.Style{Size: 32})
	if err != nil {
		t.Fatal(err)
	}
	if texBox.Width != typstBox.Width ||
		texBox.Ascent != typstBox.Ascent ||
		texBox.Descent != typstBox.Descent {
		t.Fatalf("TeX box = %+v, Typst box = %+v", texBox, typstBox)
	}
}

func assertDouble(t *testing.T, doubled, original float32, label string) {
	t.Helper()
	ratio := doubled / original
	if ratio < 1.998 || ratio > 2.002 {
		t.Fatalf("%s width ratio = %v (%v / %v)", label, ratio, doubled, original)
	}
}
