package layout

import (
	"testing"

	"github.com/caelyreth/untie/geometry"
	"github.com/caelyreth/untie/ir"
	"github.com/caelyreth/untie/source"
)

func TestSideSubscriptUsesSingleSymbolItalicCorrection(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		base ir.Node
	}{
		{"glyph", glyph("I", ir.ClassLarge)},
		{"operator", func() ir.Node {
			operator := ir.NewOperator(source.NewRange(0, 1), "I")
			operator.Class = ir.ClassLarge
			operator.Limits = ir.LimitsNever
			return operator
		}()},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			renderer, glyphs, _ := newTestRenderer(t)
			glyphs.glyphs["I"] = Glyph{
				ID:            73,
				FontID:        "test-math",
				Text:          "I",
				Advance:       em(1, 2),
				Ascent:        em(7, 10),
				Descent:       em(2, 10),
				Italic:        em(1, 10),
				ExactGlyphRef: true,
				TextLike:      false,
				Outline: Path{
					{Op: geometry.SegMoveTo, Points: [3]Point{{X: 0, Y: -em(2, 10)}}},
					{Op: geometry.SegLineTo, Points: [3]Point{{X: em(1, 2), Y: -em(2, 10)}}},
					{Op: geometry.SegLineTo, Points: [3]Point{{X: em(1, 2), Y: em(7, 10)}}},
					{Op: geometry.SegLineTo, Points: [3]Point{{X: 0, Y: em(7, 10)}}},
					{Op: geometry.SegClose},
				},
			}

			scripts := ir.NewScripts(source.NewRange(0, 2), test.base)
			scripts.BottomRight = glyph("M", ir.ClassNormal)
			scripts.Limits = ir.LimitsNever
			box, err := renderer.Layout(scripts, ir.Inline, testStyle(16))
			if err != nil {
				t.Fatal(err)
			}
			// Base 0.5em + script-space 0.05em + 80%-scaled script 0.4em -
			// base italic correction 0.1em.
			want := float32(16 * 0.85)
			if !closeHost(box.Width, want) {
				t.Fatalf("width = %v, want %v", box.Width, want)
			}
		})
	}
}
