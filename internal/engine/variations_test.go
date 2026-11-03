package engine

import (
	"context"
	"errors"
	"os"
	"slices"
	"testing"

	"github.com/caelyreth/untie/font"
	"github.com/caelyreth/untie/geometry"
	"github.com/caelyreth/untie/internal/layout"
	"github.com/caelyreth/untie/ir"
	"github.com/caelyreth/untie/source"
)

func TestMathConstructionUsesVariableGlyphMetrics(t *testing.T) {
	t.Parallel()

	data, err := os.ReadFile("../../testdata/upstream/fonts/SourceSans-VF-MATH.ttf")
	if err != nil {
		t.Fatal(err)
	}
	load := func(weight float32) *adapter {
		t.Helper()
		set, err := font.LoadMathFont(data, font.WithFontVariations(
			font.Variation{Tag: "wght", Value: weight},
		))
		if err != nil {
			t.Fatalf("LoadMathFont(%g): %v", weight, err)
		}
		adapter, err := newAdapter(set, source.DefaultLimits())
		if err != nil {
			t.Fatalf("newAdapter(%g): %v", weight, err)
		}
		return adapter
	}
	narrow := load(200)
	wide := load(900)
	request := layout.StretchRequest{
		GlyphRequest: layout.GlyphRequest{
			Context: context.Background(),
			Text:    "A",
			Variant: ir.VariantNormal,
			Font:    ir.FontMath,
		},
		Axis:   layout.Horizontal,
		Target: geometry.Em(570 * geometry.EmScale / 1000),
	}

	if _, err := narrow.Stretch(request); !errors.Is(err, layout.ErrInsufficientStretch) {
		t.Fatalf("narrow Stretch error = %v, want ErrInsufficientStretch", err)
	}
	stretched, err := wide.Stretch(request)
	if err != nil {
		t.Fatalf("wide Stretch: %v", err)
	}
	if stretched.Advance < request.Target {
		t.Fatalf("wide advance = %d, want at least %d", stretched.Advance, request.Target)
	}
	if len(stretched.Outline) == 0 {
		t.Fatal("wide stretched glyph has no outline")
	}

	narrowGlyph, err := narrow.Glyph(layout.GlyphRequest{
		Context: context.Background(),
		Text:    "A",
		Variant: ir.VariantNormal,
		Font:    ir.FontMath,
	})
	if err != nil {
		t.Fatalf("narrow Glyph: %v", err)
	}
	wideGlyph, err := wide.Glyph(layout.GlyphRequest{
		Context: context.Background(),
		Text:    "A",
		Variant: ir.VariantNormal,
		Font:    ir.FontMath,
	})
	if err != nil {
		t.Fatalf("wide Glyph: %v", err)
	}
	if slices.Equal(narrowGlyph.Outline, wideGlyph.Outline) {
		t.Fatal("adapter outline cache did not honor variable coordinates")
	}
}
