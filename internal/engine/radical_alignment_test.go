package engine

import (
	"testing"

	"github.com/caelyreth/untie/display"
	"github.com/caelyreth/untie/internal/tex"
	"github.com/caelyreth/untie/ir"
	"github.com/caelyreth/untie/source"
)

func TestRadicalVinculumAlignsWithSurdTop(t *testing.T) {
	t.Parallel()

	_, layoutEngine := newNewCMEngine(t, source.DefaultLimits())
	tests := []struct {
		name   string
		source string
		mode   ir.Mode
		size   float32
	}{
		{"inline glyph", `\sqrt{x}`, ir.Inline, 16},
		{"display glyph", `\sqrt{x}`, ir.Display, 16},
		{"display fraction small", `\sqrt{\frac{a}{b}}`, ir.Display, 12},
		{"display fraction large", `\sqrt{\frac{a}{b}}`, ir.Display, 40},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			parsed := tex.Parse(test.source, test.mode, tex.Options{})
			if len(parsed.Diagnostics) != 0 {
				t.Fatalf("diagnostics = %+v", parsed.Diagnostics)
			}
			box, err := layoutEngine.Layout(t.Context(), parsed.Root, test.mode, display.Style{Size: test.size})
			if err != nil {
				t.Fatal(err)
			}

			surdY, ok := firstKindY(box.Items, display.ItemGlyphPath)
			if !ok {
				t.Fatal("radical sign path is missing")
			}
			ruleY, ok := firstKindY(box.Items, display.ItemRule)
			if !ok {
				t.Fatal("radical rule is missing")
			}
			if difference := surdY - ruleY; difference < -0.001 || difference > 0.001 {
				t.Fatalf("surd top %v differs from vinculum top %v by %v", surdY, ruleY, difference)
			}
		})
	}
}

func firstKindY(items []display.Item, kind display.ItemKind) (float32, bool) {
	for _, item := range items {
		if item.Kind != kind {
			continue
		}
		switch kind {
		case display.ItemGlyphPath:
			return item.Bound.Y, true
		case display.ItemRule:
			return item.Rect.Y, true
		}
	}
	return 0, false
}
