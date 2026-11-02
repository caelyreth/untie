package typst

import (
	"testing"

	"github.com/caelyreth/untie/ir"
	"github.com/caelyreth/untie/source"
)

func TestDecimalNumbersLowerToNumberGlyphRuns(t *testing.T) {
	t.Parallel()

	result := parse(t, `9.81 + 2.`)
	requireNoDiagnostics(t, "decimal number runs", result)
	var glyphs []*ir.Glyph
	ir.Walk(result.Root, func(node ir.Node) bool {
		if glyph, ok := node.(*ir.Glyph); ok && (glyph.Text == "." || (glyph.Text >= "0" && glyph.Text <= "9")) {
			glyphs = append(glyphs, glyph)
		}
		return true
	})
	if len(glyphs) != 6 {
		t.Fatalf("number glyphs = %d, want 6", len(glyphs))
	}
	want := []string{"9", ".", "8", "1", "2", "."}
	for i, text := range want {
		if glyphs[i].Text != text {
			t.Fatalf("glyph %d = %q, want %q", i, glyphs[i].Text, text)
		}
		if glyphs[i].Class != ir.ClassNormal {
			t.Fatalf("glyph %d class = %v, want normal", i, glyphs[i].Class)
		}
	}
	if glyphs[0].Span != source.NewRange(0, 1) ||
		glyphs[1].Span != source.NewRange(1, 2) ||
		glyphs[2].Span != source.NewRange(2, 3) ||
		glyphs[3].Span != source.NewRange(3, 4) {
		t.Fatalf("decimal spans = %v, %v, %v, %v", glyphs[0].Span, glyphs[1].Span, glyphs[2].Span, glyphs[3].Span)
	}
}
