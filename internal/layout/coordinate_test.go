package layout

import (
	"testing"

	"github.com/caelyreth/untie/display"
	"github.com/caelyreth/untie/geometry"
	"github.com/caelyreth/untie/ir"
	"github.com/caelyreth/untie/source"
)

func TestGroupPlacementUsesBaselineCoordinates(t *testing.T) {
	t.Parallel()

	renderer, _, _ := newTestRenderer(t)
	style := testStyle(16)
	style.GlyphRefs = true

	t.Run("scripts", func(t *testing.T) {
		scripts := ir.NewScripts(source.NewRange(0, 3), glyph("x", ir.ClassAlphabetic))
		scripts.TopRight = glyph("2", ir.ClassNormal)
		scripts.BottomRight = glyph("i", ir.ClassNormal)
		box, err := renderer.Layout(scripts, ir.Inline, style)
		if err != nil {
			t.Fatal(err)
		}
		refs := glyphItems(box)
		if len(refs) != 3 {
			t.Fatalf("refs = %d, want 3", len(refs))
		}
		if refs[0].At.Y != 0 || refs[1].At.Y >= 0 || refs[2].At.Y <= 0 {
			t.Fatalf("baseline positions = %v, %v, %v", refs[0].At.Y, refs[1].At.Y, refs[2].At.Y)
		}
	})

	t.Run("fraction", func(t *testing.T) {
		fraction := ir.NewFraction(source.NewRange(0, 3), glyph("1", ir.ClassNormal), glyph("2", ir.ClassNormal))
		box, err := renderer.Layout(fraction, ir.Inline, style)
		if err != nil {
			t.Fatal(err)
		}
		items := glyphItems(box)
		if len(items) != 2 || items[0].At.Y >= 0 || items[1].At.Y <= 0 {
			t.Fatalf("fraction positions = %+v", items)
		}
	})

	t.Run("accent", func(t *testing.T) {
		accentRenderer, glyphs, _ := newTestRenderer(t)
		glyphs.glyphs["^"] = Glyph{
			ID:           94,
			FontID:       "test-math",
			Text:         "^",
			Advance:      em(1, 2),
			Ascent:       em(4, 10),
			InkBottom:    -em(2, 10),
			HasInkBottom: true,
			TextLike:     true,
			Outline: Path{
				{Op: geometry.SegMoveTo, Points: [3]Point{{X: 0, Y: -em(4, 10)}}},
				{Op: geometry.SegLineTo, Points: [3]Point{{X: em(1, 2), Y: -em(4, 10)}}},
				{Op: geometry.SegLineTo, Points: [3]Point{{X: em(1, 2), Y: -em(2, 10)}}},
				{Op: geometry.SegLineTo, Points: [3]Point{{X: 0, Y: -em(2, 10)}}},
				{Op: geometry.SegClose},
			},
		}
		accent := ir.NewAccent(
			source.NewRange(0, 3),
			glyph("x", ir.ClassAlphabetic),
			glyph("^", ir.ClassNormal),
			ir.PositionAbove,
		)
		accent.Stretchy = false
		box, err := accentRenderer.Layout(accent, ir.Inline, style)
		if err != nil {
			t.Fatal(err)
		}
		items := glyphItems(box)
		if len(items) != 2 || items[0].At.Y != 0 || items[1].At.Y >= 0 {
			t.Fatalf("accent positions = %+v", items)
		}
	})

	t.Run("radical", func(t *testing.T) {
		radical := ir.NewRadical(
			source.NewRange(0, 3),
			glyph("x", ir.ClassNormal),
			glyph("3", ir.ClassNormal),
		)
		box, err := renderer.Layout(radical, ir.Inline, style)
		if err != nil {
			t.Fatal(err)
		}
		items := glyphItems(box)
		if len(items) < 3 || items[0].At.Y >= 0 || items[len(items)-1].At.Y != 0 {
			t.Fatalf("radical positions = %+v", items)
		}
	})

	t.Run("line", func(t *testing.T) {
		line := ir.NewLine(source.NewRange(0, 3), glyph("x", ir.ClassNormal), ir.PositionAbove)
		box, err := renderer.Layout(line, ir.Inline, style)
		if err != nil {
			t.Fatal(err)
		}
		items := glyphItems(box)
		if len(items) != 1 || items[0].At.Y != 0 {
			t.Fatalf("line body position = %+v", items)
		}
		for _, item := range box.Items {
			if item.Kind == display.ItemRule && item.Rect.Y >= 0 {
				t.Fatalf("overbar rule y = %v, want above baseline", item.Rect.Y)
			}
		}
	})

	t.Run("box", func(t *testing.T) {
		boxed := ir.NewBox(source.NewRange(0, 3), glyph("x", ir.ClassNormal))
		boxed.Padding[0] = emLength(t, 0.2)
		boxed.Padding[2] = emLength(t, 0.1)
		box, err := renderer.Layout(boxed, ir.Inline, style)
		if err != nil {
			t.Fatal(err)
		}
		items := glyphItems(box)
		if len(items) != 1 || items[0].At.Y != 0 {
			t.Fatalf("boxed body position = %+v", items)
		}
	})
}

func glyphItems(box *display.Box) []display.Item {
	items := make([]display.Item, 0, len(box.Items))
	for _, item := range box.Items {
		if item.Kind == display.ItemGlyphRef {
			items = append(items, item)
		}
	}
	return items
}
