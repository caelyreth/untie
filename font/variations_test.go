package font

import (
	"context"
	"math"
	"os"
	"slices"
	"testing"
)

func TestVariableFontConfigurationAndIdentity(t *testing.T) {
	t.Parallel()

	data := readVariableMathFont(t)
	narrow, err := LoadMathFont(data, WithFontVariations(Variation{Tag: "wght", Value: 200}))
	if err != nil {
		t.Fatalf("LoadMathFont(narrow): %v", err)
	}
	wide, err := LoadMathFont(data, WithFontVariations(Variation{Tag: "wght", Value: 900}))
	if err != nil {
		t.Fatalf("LoadMathFont(wide): %v", err)
	}

	narrowInfo, _ := narrow.Face(MathFace)
	wideInfo, _ := wide.Face(MathFace)
	if !narrowInfo.Variable || !wideInfo.Variable {
		t.Fatalf("variable metadata = %v, %v", narrowInfo.Variable, wideInfo.Variable)
	}
	if narrowInfo.ID == wideInfo.ID {
		t.Fatal("variation coordinates did not participate in derived face IDs")
	}
	if narrow.KeySHA256() == wide.KeySHA256() {
		t.Fatal("variation coordinates did not participate in FontSet identity")
	}

	axes, ok := narrow.VariationAxes(MathFace)
	if !ok || len(axes) != 1 {
		t.Fatalf("VariationAxes = %+v, %v", axes, ok)
	}
	if axes[0] != (VariationAxis{Tag: "wght", Minimum: 200, Default: 200, Maximum: 900}) {
		t.Fatalf("wght axis = %+v", axes[0])
	}
	coordinates, ok := wide.VariationCoordinates(MathFace)
	if !ok || !slices.Equal(coordinates, []Variation{{Tag: "wght", Value: 900}}) {
		t.Fatalf("VariationCoordinates = %+v, %v", coordinates, ok)
	}

	narrowGlyph, _ := narrow.MapRune(MathFace, 'A')
	wideGlyph, _ := wide.MapRune(MathFace, 'A')
	narrowMetrics, _ := narrow.GlyphMetrics(narrowGlyph)
	wideMetrics, _ := wide.GlyphMetrics(wideGlyph)
	if narrowMetrics.Advance == wideMetrics.Advance {
		t.Fatalf("variation did not change advance: %d", narrowMetrics.Advance)
	}
	if narrowMetrics.Bounds == wideMetrics.Bounds {
		t.Fatalf("variation did not change outline bounds: %+v", narrowMetrics.Bounds)
	}
	narrowOutline, _ := narrow.Outline(narrowGlyph)
	wideOutline, _ := wide.Outline(wideGlyph)
	if slices.Equal(narrowOutline.Segments, wideOutline.Segments) {
		t.Fatal("variation did not change outline segments")
	}
	narrowVertical, _ := narrow.VerticalMetrics(MathFace)
	wideVertical, _ := wide.VerticalMetrics(MathFace)
	if narrowVertical.XHeight == wideVertical.XHeight {
		t.Fatalf("MVAR x-height did not vary: %d", narrowVertical.XHeight)
	}

	narrowConstruction, ok := narrow.MathConstruction(narrowGlyph, HorizontalAxis)
	if !ok {
		t.Fatal("narrow horizontal MATH construction missing")
	}
	wideConstruction, ok := wide.MathConstruction(wideGlyph, HorizontalAxis)
	if !ok {
		t.Fatal("wide horizontal MATH construction missing")
	}
	if !slices.Equal(narrowConstruction.Variants, wideConstruction.Variants) {
		t.Fatal("the static MATH construction recipe changed with coordinates")
	}

	narrowShaped := shapeSingleRune(t, narrow, 'A')
	wideShaped := shapeSingleRune(t, wide, 'A')
	if shapedAdvance(narrowShaped) == shapedAdvance(wideShaped) {
		t.Fatal("text shaping advance did not change with coordinates")
	}
}

func TestVariableFontDefaultsAndOwnership(t *testing.T) {
	t.Parallel()

	data := readVariableMathFont(t)
	set, err := LoadMathFont(data)
	if err != nil {
		t.Fatalf("LoadMathFont: %v", err)
	}
	coordinates, ok := set.VariationCoordinates(MathFace)
	if !ok || !slices.Equal(coordinates, []Variation{{Tag: "wght", Value: 200}}) {
		t.Fatalf("default coordinates = %+v, %v", coordinates, ok)
	}
	configured, err := LoadMathFont(data, WithFontVariations(Variation{Tag: "wght", Value: 765.5}))
	if err != nil {
		t.Fatalf("LoadMathFont(configured): %v", err)
	}
	copied := configured.Config()
	if !slices.Equal(copied.Math.Variations, []Variation{{Tag: "wght", Value: 765.5}}) {
		t.Fatalf("Config variations = %+v", copied.Math.Variations)
	}
	copied.Math.Variations[0].Value = 200
	if got := configured.Config().Math.Variations[0].Value; got != 765.5 {
		t.Fatalf("Config returned shared variation storage: %g", got)
	}
}

func TestVariableTextFaceParticipatesInFallbackShaping(t *testing.T) {
	t.Parallel()

	mathData := readTestFont(t)
	variableData := readVariableMathFont(t)
	shape := func(weight float32) ShapedText {
		t.Helper()
		text := FontFace{
			Bytes:      variableData,
			ID:         "variable-text",
			Variations: []Variation{{Tag: "wght", Value: weight}},
		}
		set, err := LoadFontSet(FontSetConfig{
			Math: FontFace{Bytes: mathData, ID: "math"},
			Text: &text,
		})
		if err != nil {
			t.Fatalf("LoadFontSet: %v", err)
		}
		shaped, err := set.ShapeText(TextShapeRequest{
			Context: context.Background(),
			Text:    "A",
			Faces:   []FaceRef{TextFace, MathFace},
		})
		if err != nil {
			t.Fatalf("ShapeText: %v", err)
		}
		if len(shaped.Clusters) != 1 || shaped.Clusters[0].Face != TextFace {
			t.Fatalf("selected clusters = %+v", shaped.Clusters)
		}
		return shaped
	}
	if shapedAdvance(shape(200)) == shapedAdvance(shape(900)) {
		t.Fatal("variable text face did not affect fallback-shaped advance")
	}
}

func TestVariableFontRejectsInvalidCoordinates(t *testing.T) {
	t.Parallel()

	data := readVariableMathFont(t)
	tests := []struct {
		name       string
		variations []Variation
	}{
		{"unknown axis", []Variation{{Tag: "wdth", Value: 100}}},
		{"below minimum", []Variation{{Tag: "wght", Value: 199}}},
		{"above maximum", []Variation{{Tag: "wght", Value: 901}}},
		{"duplicate axis", []Variation{{Tag: "wght", Value: 300}, {Tag: "wght", Value: 400}}},
		{"short tag", []Variation{{Tag: "wgh", Value: 300}}},
		{"control tag", []Variation{{Tag: "wg\x00t", Value: 300}}},
		{"NaN", []Variation{{Tag: "wght", Value: float32(math.NaN())}}},
		{"positive infinity", []Variation{{Tag: "wght", Value: float32(math.Inf(1))}}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if _, err := LoadMathFont(data, WithFontVariations(tc.variations...)); err == nil {
				t.Fatal("LoadMathFont() error = nil")
			}
		})
	}
	if _, err := LoadMathFont(readTestFont(t), WithFontVariations(Variation{Tag: "wght", Value: 500})); err == nil {
		t.Fatal("static face accepted variation coordinates")
	}
}

func TestArbitraryVariableAxesAreDeterministic(t *testing.T) {
	t.Parallel()

	mathData := readTestFont(t)
	variableData, err := os.ReadFile("../testdata/upstream/fonts/Estedad-VF.ttf")
	if err != nil {
		t.Fatal(err)
	}
	load := func(variations []Variation) *FontSet {
		t.Helper()
		text := FontFace{Bytes: variableData, ID: "estedad", Variations: variations}
		set, err := LoadFontSet(FontSetConfig{
			Math: FontFace{Bytes: mathData, ID: "math"},
			Text: &text,
		})
		if err != nil {
			t.Fatalf("LoadFontSet: %v", err)
		}
		return set
	}
	first := load([]Variation{{Tag: "wdth", Value: 150}, {Tag: "wght", Value: 400}})
	second := load([]Variation{{Tag: "wght", Value: 400}, {Tag: "wdth", Value: 150}})
	if first.KeySHA256() != second.KeySHA256() {
		t.Fatal("variation request order changed FontSet identity")
	}
	axes, ok := first.VariationAxes(TextFace)
	if !ok || len(axes) != 2 {
		t.Fatalf("VariationAxes = %+v, %v", axes, ok)
	}
	coordinates, ok := first.VariationCoordinates(TextFace)
	if !ok || !slices.Equal(coordinates, []Variation{
		{Tag: "wght", Value: 400},
		{Tag: "wdth", Value: 150},
	}) {
		t.Fatalf("VariationCoordinates = %+v, %v", coordinates, ok)
	}
}

func TestVariableFontGSUBFeatureConditions(t *testing.T) {
	t.Parallel()

	// One ssty feature has no default lookup. FeatureVariations v1.0 selects
	// lookup 0 only at normalized coordinate 1.0.
	gsub := make([]byte, 100)
	put16 := func(off int, value uint16) { gsub[off] = byte(value >> 8); gsub[off+1] = byte(value) }
	put32 := func(off int, value uint32) {
		gsub[off] = byte(value >> 24)
		gsub[off+1] = byte(value >> 16)
		gsub[off+2] = byte(value >> 8)
		gsub[off+3] = byte(value)
	}
	put16(0, 1)
	put16(2, 1)
	put16(6, 14)
	put16(8, 26)
	put32(10, 52)
	// FeatureList at 14: one "ssty" record with no default lookup.
	put16(14, 1)
	copy(gsub[16:20], "ssty")
	put16(20, 8)
	put16(22, 0)
	put16(24, 0)
	// LookupList at 26: one single-substitution lookup at 30. Its subtable
	// is intentionally identity, since this test exercises selection.
	put16(26, 1)
	put16(28, 4)
	put16(30, 1)
	put16(32, 0)
	put16(34, 1)
	put16(36, 8)
	put16(38, 1)
	put16(40, 6)
	put16(42, 0)
	put16(44, 1)
	put16(46, 1)
	put16(48, 0)
	// FeatureVariations at 52.
	put16(52, 1)
	put16(54, 0)
	put32(56, 1)
	put32(60, 16)
	put32(64, 30)
	// ConditionSet at 68: one axis-0 condition for normalized coordinate 1.
	put16(68, 1)
	put32(70, 6)
	put16(74, 1)
	put16(76, 0)
	put16(78, 0x4000)
	put16(80, 0x4000)
	// FeatureTableSubstitution at 82 replaces feature 0 with lookup 0.
	put16(82, 1)
	put16(84, 0)
	put16(86, 1)
	put16(88, 0)
	put32(90, 12)
	put16(94, 0)
	put16(96, 1)
	put16(98, 0)

	parse := func(coordinate float32) gsubTable {
		t.Helper()
		table, err := parseGSUBTable(gsub, []float32{coordinate})
		if err != nil {
			t.Fatalf("parseGSUBTable(%g): %v", coordinate, err)
		}
		return table
	}
	if got := parse(0); len(got.features["ssty"]) != 0 {
		t.Fatalf("ssty unexpectedly active outside condition: %+v", got.features["ssty"])
	}
	if got := parse(1); len(got.features["ssty"]) != 1 {
		t.Fatalf("ssty feature variation not selected: %+v", got.features["ssty"])
	}
}

func shapeSingleRune(t *testing.T, set *FontSet, r rune) ShapedText {
	t.Helper()
	shaped, err := set.ShapeText(TextShapeRequest{
		Context: context.Background(),
		Text:    string(r),
		Faces:   []FaceRef{MathFace},
	})
	if err != nil {
		t.Fatalf("ShapeText: %v", err)
	}
	return shaped
}

func shapedAdvance(shaped ShapedText) int64 {
	var total int64
	for _, glyph := range shaped.Glyphs {
		total += int64(glyph.Advance)
	}
	return total
}

func readVariableMathFont(t testing.TB) []byte {
	t.Helper()
	data, err := os.ReadFile("../testdata/upstream/fonts/SourceSans-VF-MATH.ttf")
	if err != nil {
		t.Fatalf("read variable math font: %v", err)
	}
	return data
}
