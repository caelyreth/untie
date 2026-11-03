package engine

import (
	"context"
	"errors"
	"os"
	"testing"

	"github.com/caelyreth/untie/display"
	"github.com/caelyreth/untie/font"
	"github.com/caelyreth/untie/geometry"
	"github.com/caelyreth/untie/internal/layout"
	"github.com/caelyreth/untie/internal/tex"
	"github.com/caelyreth/untie/internal/typst"
	"github.com/caelyreth/untie/ir"
	"github.com/caelyreth/untie/source"
)

const positionTolerance = 0.002

type glyphPosition struct {
	id uint16
	x  float32
	y  float32
}

func TestNewCMExactBaselinePlacement(t *testing.T) {
	set, engine := newNewCMEngine(t, source.Limits{})
	tests := []struct {
		name    string
		source  string
		width   float32
		ascent  float32
		descent float32
		refs    []glyphPosition
		mode    ir.Mode
	}{
		{
			name: "x", source: "x",
			width: 9.1521, ascent: 10.9279785, descent: 0,
			refs: []glyphPosition{{3570, 0, 0}},
		},
		{
			name: "superscript", source: "x^2",
			width: 16.420898, ascent: 10.9279785, descent: 0,
			refs: []glyphPosition{{3570, 0, 0}, {5477, 9.152344, -5.8085938}},
		},
		{
			name: "subscript", source: "x_i",
			width: 14.572998, ascent: 10.9279785, descent: 0,
			refs: []glyphPosition{{3570, 0, 0}, {5821, 9.152344, 3.9521484}},
		},
		{
			name: "both scripts", source: "x_i^2",
			width: 16.420898, ascent: 10.9279785, descent: 0,
			refs: []glyphPosition{{3570, 0, 0}, {5477, 9.152344, -5.9208984}, {5821, 9.152344, 4.064453}},
		},
		{
			name: "fraction", source: `\frac{1}{2}`,
			width: 9.572998, ascent: 10.9279785, descent: 0,
			refs: []glyphPosition{{5430, 1.6005859, -6.303711}, {5477, 1.6005859, 5.520508}},
		},
		{
			name: "sum limits", source: `\sum_{i=1}^{n}`,
			width: 23.104492, ascent: 23.451172, descent: 17.420898,
			refs: []glyphPosition{
				{6816, 0, 0},
				{5826, 7.598633, -18.511719},
				{5821, 1.7460938, 17.308594},
				{30, 6.2714844, 17.308594},
				{5430, 14.985352, 17.308594},
			},
			mode: ir.Display,
		},
		{
			name: "radical", source: `\sqrt{x}`,
			width: 22.47998, ascent: 10.9279785, descent: 0,
			refs: []glyphPosition{{1001, 0, -11.591797}, {3570, 13.328125, 0}},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			box := layoutTeX(t, engine, test.source, test.mode)
			assertBoxMetrics(t, box, test.width, test.ascent, test.descent)
			assertRefs(t, box, test.refs)
		})
	}
	t.Run("accent", func(t *testing.T) {
		result := tex.Parse(`\hat{x}`, ir.Inline, tex.Options{})
		box, err := engine.Layout(context.Background(), result.Root, ir.Inline, display.Style{Size: 16, GlyphRefs: true})
		if err != nil {
			t.Fatalf("accent layout: %v", err)
		}
		assertBoxMetrics(t, box, 9.1521, 10.9279785, 0)
		assertRefs(t, box, []glyphPosition{{3570, 0, 0}, {63, -0.4000651, 0}})
	})

	// The parenthesis path is an assembly at this height; both delimiters center
	// on the math axis while the fraction keeps its own baseline.
	box := layoutTeX(t, engine, `\left(\frac{x}{y}\right)`, ir.Inline)
	assertBoxMetrics(t, box, 27.193604, 10.9279785, 0.5246582)
	assertRefs(t, box, []glyphPosition{
		{6534, 0, 0},
		{5836, 9.968018, -6.303955},
		{5837, 10.303955, 5.5200195},
		{6535, 18.825684, 0},
	})
	mapped, ok := set.MapRune(font.MathFace, '(')
	if !ok {
		t.Fatal("NewCM does not map '('")
	}
	construction, ok := set.MathConstruction(mapped, font.VerticalAxis)
	if !ok || construction.Assembly == nil {
		t.Fatal("NewCM '(' has no vertical assembly")
	}
}

func TestNewCMVariantsTextAndOperators(t *testing.T) {
	_, engine := newNewCMEngine(t, source.Limits{})
	t.Run("named operators", func(t *testing.T) {
		box := layoutTeX(t, engine, `\sin x`, ir.Inline)
		assertRefs(t, box, []glyphPosition{{84, 0, 0}, {74, 6.303711, 0}, {79, 10.751953, 0}, {3570, 22.426758, 0}})

		box = layoutTeX(t, engine, `\operatorname{log} x`, ir.Inline)
		assertRefs(t, box, []glyphPosition{{77, 0, 0}, {80, 4.5283203, 0}, {72, 12.52832, 0}, {3570, 23.40332, 0}})
	})
	t.Run("uppercase Greek", func(t *testing.T) {
		plain := layoutTeX(t, engine, `\Gamma`, ir.Inline)
		italic := layoutTeX(t, engine, `\mathit{\Gamma}`, ir.Inline)
		assertRefs(t, plain, []glyphPosition{{374, 0, 0}})
		assertRefs(t, italic, []glyphPosition{{4184, 0, 0}})
		if plain.Items[1].Glyph == italic.Items[1].Glyph {
			t.Fatal("\\Gamma and \\mathit{\\Gamma} selected the same glyph")
		}
	})
	t.Run("text", func(t *testing.T) {
		plain := layoutTeX(t, engine, `\text{ab}`, ir.Inline)
		assertBoxMetrics(t, plain, 16.895996, 10.9279785, 0)
		assertRefs(t, plain, []glyphPosition{{66, 0, 0}, {67, 8, 0}})

		bold := layoutTeX(t, engine, `\textbf{ab}`, ir.Inline)
		assertBoxMetrics(t, bold, 19.168213, 10.9279785, 0)
		assertRefs(t, bold, []glyphPosition{{3496, 0, 0}, {3497, 8.944336, 0}})

		tracking, err := ir.EmLength(0.1)
		if err != nil {
			t.Fatal(err)
		}
		text := ir.NewText(source.NewRange(0, 2), "ab")
		text.Tracking = tracking
		box, err := engine.Layout(context.Background(), text, ir.Inline, display.Style{Size: 16, GlyphRefs: true})
		if err != nil {
			t.Fatalf("tracked text: %v", err)
		}
		assertBoxMetrics(t, box, 20.096191, 10.9279785, 0)
		assertRefs(t, box, []glyphPosition{{66, 0, 0}, {67, 9.6, 0}})

	})
}

func TestNewCMMetricsIncludeDisplayOperatorMinHeight(t *testing.T) {
	t.Parallel()

	set, _ := newNewCMEngine(t, source.Limits{})
	adapter, err := newAdapter(set, source.Limits{})
	if err != nil {
		t.Fatal(err)
	}
	metrics := adapter.MathMetrics(layout.Style{Size: layout.Display}, ir.VariantAuto)
	want := geometry.Em(13 * geometry.EmScale / 10)
	if !closeEm(metrics.DisplayOperatorMinHeight, want) {
		t.Fatalf("DisplayOperatorMinHeight = %d, want %d", metrics.DisplayOperatorMinHeight, want)
	}
}

func TestAdapterStretchShortfallAssemblyAndLimits(t *testing.T) {
	set, _ := newNewCMEngine(t, source.Limits{})
	adapter, err := newAdapter(set, source.Limits{})
	if err != nil {
		t.Fatal(err)
	}
	base, err := adapter.Glyph(layout.GlyphRequest{
		Context: context.Background(), Text: "(", Style: layout.Style{Size: layout.Text}, Font: ir.FontMath,
	})
	if err != nil {
		t.Fatalf("base parenthesis: %v", err)
	}
	target := base.Advance + geometry.Em(geometry.EmScale)
	short, err := adapter.Stretch(layout.StretchRequest{
		GlyphRequest: layout.GlyphRequest{
			Context: context.Background(), Text: "(", Style: layout.Style{Size: layout.Text}, Font: ir.FontMath,
		},
		Axis: layout.Vertical, Target: target, ShortFall: target,
	})
	if err != nil {
		t.Fatalf("shortfall stretch: %v", err)
	}
	if short.ID != base.ID {
		t.Fatalf("shortfall glyph = %d, want base %d", short.ID, base.ID)
	}

	variant, err := adapter.Stretch(layout.StretchRequest{
		GlyphRequest: layout.GlyphRequest{
			Context: context.Background(), Text: "(", Style: layout.Style{Size: layout.Text}, Font: ir.FontMath,
		},
		Axis: layout.Vertical, Target: 2 * geometry.Em(geometry.EmScale),
	})
	if err != nil {
		t.Fatalf("variant stretch: %v", err)
	}
	if variant.ID == base.ID {
		t.Fatalf("variant stretch retained base glyph %d", base.ID)
	}
	if extent, err := glyphExtent(variant, layout.Vertical); err != nil || extent < 2*geometry.Em(geometry.EmScale) {
		t.Fatalf("variant extent = %d, %v", extent, err)
	}

	assembled, err := adapter.Stretch(layout.StretchRequest{
		GlyphRequest: layout.GlyphRequest{
			Context: context.Background(), Text: "(", Style: layout.Style{Size: layout.Text}, Font: ir.FontMath,
		},
		Axis: layout.Vertical, Target: 20 * geometry.Em(geometry.EmScale),
	})
	if err != nil {
		t.Fatalf("assembly stretch: %v", err)
	}
	if len(assembled.Parts) <= 1 {
		t.Fatalf("assembly parts = %d, want multiple glyph parts", len(assembled.Parts))
	}
	if assembled.ExactGlyphRef {
		t.Fatal("assembly claims an exact single-glyph reference")
	}
	if len(assembled.Parts[0].Glyph.Outline) == 0 {
		t.Fatal("assembly part has no outline")
	}
	if assembled.Descent <= 0 || assembled.Ascent <= assembled.Descent {
		t.Fatalf("assembly not centered on axis: ascent=%d descent=%d", assembled.Ascent, assembled.Descent)
	}
	radicalAssembly, err := adapter.Stretch(layout.StretchRequest{
		GlyphRequest: layout.GlyphRequest{
			Context: context.Background(), Text: "√", Style: layout.Style{Size: layout.Text}, Font: ir.FontMath,
		},
		Axis: layout.Vertical, Target: 20 * geometry.Em(geometry.EmScale),
	})
	if err != nil {
		t.Fatalf("radical assembly stretch: %v", err)
	}
	if len(radicalAssembly.Parts) <= 1 || radicalAssembly.ExactGlyphRef {
		t.Fatalf("radical assembly = parts %d exact-ref %v", len(radicalAssembly.Parts), radicalAssembly.ExactGlyphRef)
	}

	limited, err := newAdapter(set, source.Limits{MaxOutputItems: 2})
	if err != nil {
		t.Fatal(err)
	}
	_, err = limited.Stretch(layout.StretchRequest{
		GlyphRequest: layout.GlyphRequest{
			Context: context.Background(), Text: "(", Style: layout.Style{Size: layout.Text}, Font: ir.FontMath,
		},
		Axis: layout.Vertical, Target: 20 * geometry.Em(geometry.EmScale),
	})
	if !errors.Is(err, layout.ErrLimit) {
		t.Fatalf("assembly limit error = %v, want ErrLimit", err)
	}
	repeats, err := newAdapter(set, source.Limits{MaxAssemblyRepeats: 1})
	if err != nil {
		t.Fatal(err)
	}
	_, err = repeats.Stretch(layout.StretchRequest{
		GlyphRequest: layout.GlyphRequest{
			Context: context.Background(), Text: "(", Style: layout.Style{Size: layout.Text}, Font: ir.FontMath,
		},
		Axis: layout.Vertical, Target: 20 * geometry.Em(geometry.EmScale),
	})
	if !errors.Is(err, layout.ErrLimit) {
		t.Fatalf("assembly repeat-limit error = %v, want ErrLimit", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err = adapter.Stretch(layout.StretchRequest{
		GlyphRequest: layout.GlyphRequest{
			Context: ctx, Text: "(", Style: layout.Style{Size: layout.Text}, Font: ir.FontMath,
		},
		Axis: layout.Vertical, Target: geometry.Em(geometry.EmScale),
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled stretch error = %v, want context.Canceled", err)
	}
}

func TestAutomaticDisplayOperatorHeightIsBestEffort(t *testing.T) {
	t.Parallel()

	_, engine := newNewCMEngine(t, source.Limits{})
	texBox := layoutTeX(t, engine, `\operatorname{T}`, ir.Display)
	typstRoot := typst.Parse(`op("T")`, ir.Display, typst.Options{}).Root
	typstBox, err := engine.Layout(context.Background(), typstRoot, ir.Display, display.Style{Size: 16})
	if err != nil {
		t.Fatalf("Typst op display layout: %v", err)
	}
	large := ir.NewGlyph(source.NewRange(0, 1), "x")
	large.Class = ir.ClassLarge
	irBox, err := engine.Layout(context.Background(), large, ir.Display, display.Style{Size: 16})
	if err != nil {
		t.Fatalf("class(large, x) display layout: %v", err)
	}
	for name, box := range map[string]*display.Box{"tex operator": texBox, "typst op": typstBox, "large glyph": irBox} {
		if box == nil || box.Width <= 0 || box.Ascent+box.Descent <= 0 {
			t.Fatalf("%s box = %+v", name, box)
		}
	}
}

func TestAdapterStretchReachesTallMatrixDelimiter(t *testing.T) {
	t.Parallel()

	set, _ := newNewCMEngine(t, source.Limits{})
	adapter, err := newAdapter(set, source.Limits{})
	if err != nil {
		t.Fatal(err)
	}
	target := geometry.Em(300_418)
	glyph, err := adapter.Stretch(layout.StretchRequest{
		GlyphRequest: layout.GlyphRequest{
			Context: context.Background(),
			Text:    "[",
			Style:   layout.Style{Size: layout.Display},
			Font:    ir.FontMath,
		},
		Axis:   layout.Vertical,
		Target: target,
	})
	if err != nil {
		t.Fatalf("Stretch() error = %v", err)
	}
	if extent, err := glyphExtent(glyph, layout.Vertical); err != nil || extent < target {
		t.Fatalf("stretched extent = %d, %v; want at least %d", extent, err, target)
	}
}

func TestExpandAssemblyPreservesPartOrder(t *testing.T) {
	parts := []font.AssemblyPart{
		{Glyph: 1, FullAdvance: 10},
		{Glyph: 2, FullAdvance: 20, Extender: true},
		{Glyph: 3, FullAdvance: 30, Extender: true},
		{Glyph: 4, FullAdvance: 40},
	}
	sequence, err := expandAssembly(parts, 2)
	if err != nil {
		t.Fatal(err)
	}
	want := []font.GlyphID{1, 2, 2, 3, 3, 4}
	if len(sequence) != len(want) {
		t.Fatalf("sequence length = %d, want %d", len(sequence), len(want))
	}
	for i, id := range want {
		if sequence[i].Glyph != id {
			t.Fatalf("sequence[%d] = %d, want %d", i, sequence[i].Glyph, id)
		}
	}
}

func TestVariantSelectionAndBaseMetricUnits(t *testing.T) {
	set, _ := newNewCMEngine(t, source.Limits{})
	adapter, err := newAdapter(set, source.Limits{})
	if err != nil {
		t.Fatal(err)
	}
	textMetrics := adapter.MathMetrics(layout.Style{Size: layout.Text}, ir.VariantAuto)
	scriptMetrics := adapter.MathMetrics(layout.Style{Size: layout.Script}, ir.VariantAuto)
	if textMetrics != scriptMetrics {
		t.Fatal("MathMetrics changed with style; contract requires base ems")
	}
	if textMetrics.AxisHeight <= 0 || textMetrics.AccentBaseHeight <= 0 {
		t.Fatalf("missing base metrics: %+v", textMetrics)
	}

	tests := []struct {
		rune    rune
		variant ir.Variant
		want    string
	}{
		{'x', ir.VariantAuto, "𝑥"},
		{'Γ', ir.VariantAuto, "Γ"},
		{'Γ', ir.VariantItalic, "𝛤"},
		{'ω', ir.VariantAuto, "𝜔"},
	}
	for _, test := range tests {
		got, ok := styleRune(test.rune, test.variant, true)
		if !ok || got != test.want {
			t.Errorf("styleRune(%q, %d) = %q, %v; want %q, true", test.rune, test.variant, got, ok, test.want)
		}
	}
	if _, ok := styleRune('x', ir.Variant(255), true); ok {
		t.Fatal("unsupported variant silently fell back")
	}
	run, err := adapter.ShapeText(layout.TextRequest{
		Context: context.Background(), Text: "ab", Style: layout.Style{Size: layout.Text},
		Font: ir.FontText, Tracking: -geometry.Em(geometry.EmScale / 10),
	})
	if err != nil {
		t.Fatalf("negative tracking: %v", err)
	}
	wantX, _ := geometry.EmFromParts(2, 5)
	wantWidth, _ := geometry.EmFromParts(856, 1000)
	if len(run.Glyphs) != 2 || !closeEm(run.Glyphs[1].At.X, wantX) || !closeEm(run.Width, wantWidth) {
		t.Fatalf("negative tracking run = %+v", run)
	}
	features, err := mathFeatures([]string{"flac=1", "ss01=2"}, layout.Style{Size: layout.Script})
	if err != nil {
		t.Fatal(err)
	}
	dtls, ssty := -1, -1
	for i, feature := range features {
		switch feature.Tag {
		case "dtls":
			dtls = i
		case "ssty":
			ssty = i
		}
	}
	if dtls < 0 || ssty < 0 || dtls >= ssty {
		t.Fatalf("feature order = %+v, want dtls before ssty", features)
	}
	features, err = mathFeatures([]string{"flac=0"}, layout.Style{Size: layout.Script})
	if err != nil {
		t.Fatal(err)
	}
	for _, feature := range features {
		if feature.Tag == "dtls" {
			t.Fatal("flac=0 enabled dtls")
		}
	}
}

func TestEngineRejectsInvalidFontSetAndAdapterHonorsTextLimits(t *testing.T) {
	if _, err := New(&font.FontSet{}, source.Limits{}); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("New(zero FontSet) error = %v", err)
	}
	set, _ := newNewCMEngine(t, source.Limits{})
	adapter, err := newAdapter(set, source.Limits{MaxTextBytes: 1})
	if err != nil {
		t.Fatal(err)
	}
	_, err = adapter.ShapeText(layout.TextRequest{
		Context: context.Background(), Text: "ab", Style: layout.Style{Size: layout.Text}, Font: ir.FontText,
	})
	if !errors.Is(err, layout.ErrLimit) {
		t.Fatalf("text limit error = %v, want ErrLimit", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err = adapter.ShapeText(layout.TextRequest{
		Context: ctx, Text: "a", Style: layout.Style{Size: layout.Text}, Font: ir.FontText,
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled text error = %v, want context.Canceled", err)
	}
	segments, err := newAdapter(set, source.Limits{MaxPathSegments: 1})
	if err != nil {
		t.Fatal(err)
	}
	_, err = segments.ShapeText(layout.TextRequest{
		Context: context.Background(), Text: "a", Style: layout.Style{Size: layout.Text}, Font: ir.FontText,
	})
	if !errors.Is(err, layout.ErrLimit) {
		t.Fatalf("path limit error = %v, want ErrLimit", err)
	}
}

func newNewCMEngine(t *testing.T, limits source.Limits) (*font.FontSet, *Engine) {
	t.Helper()
	bytes, err := os.ReadFile("../../testdata/upstream/fonts/NewCMMath-Book.otf")
	if err != nil {
		t.Fatalf("read font: %v", err)
	}
	set, err := font.LoadFontSet(font.FontSetConfig{Math: font.FontFace{Bytes: bytes, ID: "NewCMMath-Book"}})
	if err != nil {
		t.Fatalf("font.LoadFontSet() error = %v", err)
	}
	engine, err := New(set, limits)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	return set, engine
}

func layoutTeX(t *testing.T, engine *Engine, text string, mode ir.Mode) *display.Box {
	t.Helper()
	result := tex.Parse(text, mode, tex.Options{})
	for _, diagnostic := range result.Diagnostics {
		if diagnostic.Severity == source.SeverityError {
			t.Fatalf("%s diagnostics: %+v", text, result.Diagnostics)
		}
	}
	box, err := engine.Layout(context.Background(), result.Root, mode, display.Style{Size: 16, GlyphRefs: true})
	if err != nil {
		t.Fatalf("Layout(%s) error = %v", text, err)
	}
	if err := box.Validate(); err != nil {
		t.Fatalf("Box.Validate(%s) error = %v", text, err)
	}
	return box
}

func assertBoxMetrics(t *testing.T, box *display.Box, width, ascent, descent float32) {
	t.Helper()
	if !closeFloat(box.Width, width) || !closeFloat(box.Ascent, ascent) || !closeFloat(box.Descent, descent) {
		t.Fatalf(
			"box metrics = (%v,%v,%v), want (%v,%v,%v)",
			box.Width,
			box.Ascent,
			box.Descent,
			width,
			ascent,
			descent,
		)
	}
}

func assertRefs(t *testing.T, box *display.Box, want []glyphPosition) {
	t.Helper()
	refs := glyphRefs(box)
	if len(refs) != len(want) {
		t.Fatalf("glyph refs = %d (%+v), want %d (%+v)", len(refs), refs, len(want), want)
	}
	for i := range want {
		got := refs[i]
		if got.Glyph != want[i].id || !closeFloat(got.At.X, want[i].x) || !closeFloat(got.At.Y, want[i].y) {
			t.Fatalf("refs = %+v; ref[%d] = glyph %d at (%v,%v), want glyph %d at (%v,%v)",
				refs, i, got.Glyph, got.At.X, got.At.Y, want[i].id, want[i].x, want[i].y)
		}
	}
}

func glyphRefs(box *display.Box) []display.Item {
	refs := make([]display.Item, 0, len(box.Items))
	for _, item := range box.Items {
		if item.Kind == display.ItemGlyphRef {
			refs = append(refs, item)
		}
	}
	return refs
}

func closeFloat(got, want float32) bool {
	delta := got - want
	return delta <= positionTolerance && delta >= -positionTolerance
}

func closeEm(got, want geometry.Em) bool {
	delta := got - want
	return delta >= -2 && delta <= 2
}
