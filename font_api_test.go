package untie

import (
	"context"
	"errors"
	"os"
	"slices"
	"strings"
	"testing"

	"github.com/caelyreth/untie/font"
)

func TestFontSetupAliasesAndFacade(t *testing.T) {
	t.Parallel()

	data := testFontBytes(t)
	config := FontSetConfig{
		Math: FontFace{Bytes: data},
	}
	set, err := LoadFontSet(config)
	if err != nil {
		t.Fatalf("LoadFontSet: %v", err)
	}
	info, ok := set.Face(0)
	if !ok || !strings.HasPrefix(info.ID, "math-") {
		t.Fatalf("Face(0) = %#v, %v", info, ok)
	}

	set, err = LoadMathFont(data, WithFontID("host-math"), WithFontIndex(0))
	if err != nil {
		t.Fatalf("LoadMathFont: %v", err)
	}
	info, _ = set.Face(0)
	if info.ID != "host-math" {
		t.Fatalf("WithFontID produced %q", info.ID)
	}

	if _, err := LoadMathFont([]byte("not a font")); !errors.Is(err, ErrInvalidFonts) {
		t.Fatalf("LoadMathFont malformed error = %v, want ErrInvalidFonts", err)
	}
}

func TestLoadSTIXTwoMathFont(t *testing.T) {
	t.Parallel()

	data, err := os.ReadFile("testdata/upstream/fonts/STIXTwoMath-Regular.otf")
	if err != nil {
		t.Fatal(err)
	}
	fonts, err := LoadMathFont(data)
	if err != nil {
		t.Fatalf("LoadMathFont(STIX Two Math): %v", err)
	}
	info, ok := fonts.Face(font.MathFace)
	if !ok || info.PostScriptName != "STIXTwoMath-Regular" || info.GlyphCount != 6760 {
		t.Fatalf("STIX face = %#v, %v", info, ok)
	}
	for _, tc := range []struct {
		glyph   font.GlyphID
		corner  font.MathKernCorner
		heights []int32
		kerns   []int32
	}{
		{3, font.MathKernTopRight, []int32{252, 352}, []int32{0, -18, -66}},
		{12, font.MathKernTopLeft, []int32{252}, []int32{64, -28}},
	} {
		curve, ok := fonts.MathKernCurve(font.Glyph{Face: font.MathFace, ID: tc.glyph}, tc.corner)
		if !ok || !slices.Equal(curve.Heights, tc.heights) || !slices.Equal(curve.Kerns, tc.kerns) {
			t.Errorf("glyph %d corner %d = %#v, %v; want heights %v, kerns %v", tc.glyph, tc.corner, curve, ok, tc.heights, tc.kerns)
		}
	}
	renderer, err := NewRenderer(fonts, RendererOptions{})
	if err != nil {
		t.Fatal(err)
	}
	for _, src := range []string{
		`x_i^2`,
		`\frac{-b \pm \sqrt{b^2-4ac}}{2a}`,
		`\left(\sum_{i=0}^n x_i\right)^2`,
	} {
		result, err := renderer.Render(context.Background(), src, RenderRequest{Mode: Display})
		if err != nil {
			t.Fatalf("Render(%q): %v", src, err)
		}
		if result.Box == nil || len(result.Box.Items) == 0 || len(result.Diagnostics) != 0 {
			t.Errorf("Render(%q) = %#v, want a nonempty box without diagnostics", src, result)
		}
	}
}
