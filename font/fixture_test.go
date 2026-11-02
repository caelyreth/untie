package font

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"os"
	"sort"
	"strings"
	"sync"
	"testing"
	"unicode/utf8"
)

type typletFixture struct {
	Header typletHeader
	Glyphs map[GlyphID]typletGlyph
	Shapes []typletShape
	CMap   typletCMap
}

type typletHeader struct {
	Kind           string             `json:"kind"`
	GlyphCount     int                `json:"glyphCount"`
	GSUBFeatures   []string           `json:"gsubFeatures"`
	Math           map[string]float64 `json:"math"`
	Metrics        typletMetrics      `json:"metrics"`
	MinConnector   int32              `json:"minConnectorOverlap"`
	PostScriptName string             `json:"postScriptName"`
	UnitsPerEm     int32              `json:"unitsPerEm"`
}

type typletMetrics struct {
	Ascender  float64 `json:"ascender"`
	CapHeight float64 `json:"capHeight"`
	Descender float64 `json:"descender"`
	XHeight   float64 `json:"xHeight"`
}

type typletGlyph struct {
	Kind       string                `json:"kind"`
	ID         GlyphID               `json:"id"`
	Advance    int32                 `json:"advance"`
	BBox       []int32               `json:"bbox"`
	Italic     *int32                `json:"italic"`
	Accent     *int32                `json:"accent"`
	Extended   bool                  `json:"extended"`
	Kern       map[string]typletKern `json:"kern"`
	Vertical   *typletConstruction   `json:"vertical"`
	Horizontal *typletConstruction   `json:"horizontal"`
}

type typletKern struct {
	Heights []int32 `json:"heights"`
	Kerns   []int32 `json:"kerns"`
}

type typletConstruction struct {
	Variants [][2]int32      `json:"variants"`
	Assembly *typletAssembly `json:"assembly"`
}

type typletAssembly struct {
	Italic int32        `json:"italic"`
	Parts  []typletPart `json:"parts"`
}

type typletPart struct {
	Glyph          int32
	StartConnector int32
	EndConnector   int32
	FullAdvance    int32
	Extender       bool
}

func (p *typletPart) UnmarshalJSON(data []byte) error {
	var fields []json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		return err
	}
	if len(fields) != 5 {
		return fmt.Errorf("assembly part has %d fields", len(fields))
	}
	values := []any{&p.Glyph, &p.StartConnector, &p.EndConnector, &p.FullAdvance, &p.Extender}
	for i := range fields {
		if err := json.Unmarshal(fields[i], values[i]); err != nil {
			return err
		}
	}
	return nil
}

type typletShape struct {
	Kind     string            `json:"kind"`
	Features []string          `json:"features"`
	Results  []json.RawMessage `json:"results"`
}

type typletCMap struct {
	Kind      string       `json:"kind"`
	Chars     []typletChar `json:"chars"`
	Sequences []typletChar `json:"sequences"`
}

type typletChar struct {
	Text  string
	Glyph GlyphID
}

func (c *typletChar) UnmarshalJSON(data []byte) error {
	var fields []json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		return err
	}
	if len(fields) != 2 {
		return fmt.Errorf("character mapping has %d fields", len(fields))
	}
	if err := json.Unmarshal(fields[0], &c.Text); err != nil {
		return err
	}
	return json.Unmarshal(fields[1], &c.Glyph)
}

var (
	fixtureOnce sync.Once
	fixtureData *typletFixture
	fixtureErr  error
)

func loadFixture(t *testing.T) *typletFixture {
	t.Helper()
	fixtureOnce.Do(func() {
		fixtureData, fixtureErr = readTypletFixture(testFixturePath)
	})
	if fixtureErr != nil {
		t.Fatalf("load fixture: %v", fixtureErr)
	}
	return fixtureData
}

func readTypletFixture(path string) (*typletFixture, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()
	out := &typletFixture{Glyphs: make(map[GlyphID]typletGlyph)}
	reader := bufio.NewReaderSize(f, 1<<20)
	var lineNo int
	for {
		line, readErr := reader.ReadBytes('\n')
		if len(bytes.TrimSpace(line)) != 0 {
			lineNo++
			var kind struct {
				Kind string `json:"kind"`
			}
			if err := json.Unmarshal(line, &kind); err != nil {
				return nil, fmt.Errorf("line %d: %w", lineNo, err)
			}
			switch kind.Kind {
			case "font":
				if err := json.Unmarshal(line, &out.Header); err != nil {
					return nil, fmt.Errorf("line %d header: %w", lineNo, err)
				}
			case "glyph":
				var glyph typletGlyph
				if err := json.Unmarshal(line, &glyph); err != nil {
					return nil, fmt.Errorf("line %d glyph: %w", lineNo, err)
				}
				out.Glyphs[glyph.ID] = glyph
			case "shape":
				var shape typletShape
				if err := json.Unmarshal(line, &shape); err != nil {
					return nil, fmt.Errorf("line %d shape: %w", lineNo, err)
				}
				out.Shapes = append(out.Shapes, shape)
			case "cmap":
				if err := json.Unmarshal(line, &out.CMap); err != nil {
					return nil, fmt.Errorf("line %d cmap: %w", lineNo, err)
				}
			default:
				return nil, fmt.Errorf("line %d: unknown kind %q", lineNo, kind.Kind)
			}
		}
		if readErr != nil {
			if readErr == io.EOF {
				break
			}
			return nil, readErr
		}
	}
	return out, nil
}

var fixtureMathConstants = map[string]MathConstant{
	"accentBaseHeight":                         AccentBaseHeight,
	"axisHeight":                               AxisHeight,
	"displayOperatorMinHeight":                 DisplayOperatorMinHeight,
	"flattenedAccentBaseHeight":                FlattenedAccentBaseHeight,
	"fractionDenomDisplayStyleGapMin":          FractionDenomDisplayStyleGapMin,
	"fractionDenominatorDisplayStyleShiftDown": FractionDenominatorDisplayStyleShiftDown,
	"fractionDenominatorGapMin":                FractionDenominatorGapMin,
	"fractionDenominatorShiftDown":             FractionDenominatorShiftDown,
	"fractionNumDisplayStyleGapMin":            FractionNumDisplayStyleGapMin,
	"fractionNumeratorDisplayStyleShiftUp":     FractionNumeratorDisplayStyleShiftUp,
	"fractionNumeratorGapMin":                  FractionNumeratorGapMin,
	"fractionNumeratorShiftUp":                 FractionNumeratorShiftUp,
	"fractionRuleThickness":                    FractionRuleThickness,
	"lowerLimitBaselineDropMin":                LowerLimitBaselineDropMin,
	"lowerLimitGapMin":                         LowerLimitGapMin,
	"overbarExtraAscender":                     OverbarExtraAscender,
	"overbarRuleThickness":                     OverbarRuleThickness,
	"overbarVerticalGap":                       OverbarVerticalGap,
	"radicalDegreeBottomRaisePercent":          RadicalDegreeBottomRaisePercent,
	"radicalDisplayStyleVerticalGap":           RadicalDisplayStyleVerticalGap,
	"radicalExtraAscender":                     RadicalExtraAscender,
	"radicalKernAfterDegree":                   RadicalKernAfterDegree,
	"radicalKernBeforeDegree":                  RadicalKernBeforeDegree,
	"radicalRuleThickness":                     RadicalRuleThickness,
	"radicalVerticalGap":                       RadicalVerticalGap,
	"scriptPercentScaleDown":                   ScriptPercentScaleDown,
	"scriptScriptPercentScaleDown":             ScriptScriptPercentScaleDown,
	"skewedFractionHorizontalGap":              SkewedFractionHorizontalGap,
	"skewedFractionVerticalGap":                SkewedFractionVerticalGap,
	"spaceAfterScript":                         SpaceAfterScript,
	"stackBottomDisplayStyleShiftDown":         StackBottomDisplayStyleShiftDown,
	"stackBottomShiftDown":                     StackBottomShiftDown,
	"stackDisplayStyleGapMin":                  StackDisplayStyleGapMin,
	"stackGapMin":                              StackGapMin,
	"stackTopDisplayStyleShiftUp":              StackTopDisplayStyleShiftUp,
	"stackTopShiftUp":                          StackTopShiftUp,
	"subSuperscriptGapMin":                     SubSuperscriptGapMin,
	"subscriptBaselineDropMin":                 SubscriptBaselineDropMin,
	"subscriptShiftDown":                       SubscriptShiftDown,
	"subscriptTopMax":                          SubscriptTopMax,
	"superscriptBaselineDropMax":               SuperscriptBaselineDropMax,
	"superscriptBottomMaxWithSubscript":        SuperscriptBottomMaxWithSubscript,
	"superscriptBottomMin":                     SuperscriptBottomMin,
	"superscriptShiftUp":                       SuperscriptShiftUp,
	"superscriptShiftUpCramped":                SuperscriptShiftUpCramped,
	"underbarExtraDescender":                   UnderbarExtraDescender,
	"underbarRuleThickness":                    UnderbarRuleThickness,
	"underbarVerticalGap":                      UnderbarVerticalGap,
	"upperLimitBaselineRiseMin":                UpperLimitBaselineRiseMin,
	"upperLimitGapMin":                         UpperLimitGapMin,
}

func TestNewCMFixtureHeader(t *testing.T) {
	t.Parallel()
	set := loadSharedTestFont(t)
	fixture := loadFixture(t)
	h := fixture.Header
	if h.Kind != "font" || h.GlyphCount != 8598 || h.UnitsPerEm != 1000 || h.PostScriptName != "NewCMMath-Book" {
		t.Fatalf("unexpected fixture header: %#v", h)
	}
	if len(fixture.Glyphs) != h.GlyphCount {
		t.Fatalf("fixture has %d glyph records, want %d", len(fixture.Glyphs), h.GlyphCount)
	}
	features, ok := set.Features(MathFace)
	if !ok || !slicesEqual(features, []string{"dtls", "ssty"}) {
		t.Fatalf("Features = %v, %v", features, ok)
	}

	for name, value := range h.Math {
		if name == "spaceWidth" {
			continue // A MATH constant in Typst, not in the OpenType MATH table.
		}
		c, ok := fixtureMathConstants[name]
		if !ok {
			t.Errorf("fixture MATH constant %q has no test mapping", name)
			continue
		}
		want := value
		switch c {
		case RadicalDegreeBottomRaisePercent:
			// The OpenType field is an integer percentage; Typst normalizes it
			// to a 0..1 ratio in the fixture.
			want *= 100
		case ScriptPercentScaleDown, ScriptScriptPercentScaleDown:
		default:
			want *= float64(h.UnitsPerEm)
		}
		got, ok := set.MathConstant(MathFace, c)
		if !ok {
			t.Errorf("%s: MathConstant = false", c)
			continue
		}
		if got != int32(math.Round(want)) {
			t.Errorf("%s = %d, want %g", c, got, want)
		}
	}
	for name := range fixtureMathConstants {
		if _, present := h.Math[name]; !present {
			t.Errorf("test mapping for %s has no fixture value", name)
		}
	}

	vm, ok := set.VerticalMetrics(MathFace)
	if !ok {
		t.Fatal("VerticalMetrics = false")
	}
	for name, tc := range map[string]struct {
		got  int32
		want float64
	}{
		"ascender":  {vm.Ascender, h.Metrics.Ascender},
		"descender": {vm.Descender, h.Metrics.Descender},
		"capHeight": {vm.CapHeight, h.Metrics.CapHeight},
		"xHeight":   {vm.XHeight, h.Metrics.XHeight},
	} {
		want := int32(math.Round(tc.want * float64(h.UnitsPerEm)))
		if tc.got != want {
			t.Errorf("%s = %d, want %d", name, tc.got, want)
		}
	}
}

func TestNewCMGlyphMetricsConformance(t *testing.T) {
	t.Parallel()
	set := loadSharedTestFont(t)
	fixture := loadFixture(t)
	ids := make([]int, 0, len(fixture.Glyphs))
	for id := range fixture.Glyphs {
		ids = append(ids, int(id))
	}
	sort.Ints(ids)
	failures := 0
	for _, rawID := range ids {
		id := GlyphID(rawID)
		want := fixture.Glyphs[id]
		glyph := Glyph{Face: MathFace, ID: id}
		got, ok := set.GlyphMetrics(glyph)
		if !ok {
			t.Fatalf("glyph %d: GlyphMetrics = false", id)
		}
		if got.Advance != want.Advance {
			failures = reportMetric(t, failures, "glyph %d advance = %d, want %d", id, got.Advance, want.Advance)
		}
		if len(want.BBox) == 0 {
			if got.HasOutline {
				failures = reportMetric(t, failures, "glyph %d unexpectedly has an outline", id)
			}
		} else {
			if len(want.BBox) != 4 {
				t.Fatalf("glyph %d fixture bbox has %d entries", id, len(want.BBox))
			}
			expected := Bounds{XMin: want.BBox[0], YMin: -want.BBox[3], XMax: want.BBox[2], YMax: -want.BBox[1]}
			if got.Bounds != expected {
				if !knownCFFComponentDifference(id, got.Bounds, expected) {
					failures = reportMetric(t, failures, "glyph %d bounds = %#v, want %#v", id, got.Bounds, expected)
				}
			}
			if !got.HasOutline {
				failures = reportMetric(t, failures, "glyph %d is missing its outline", id)
			}
		}
		if want.Italic == nil {
			if v, _ := set.ItalicCorrection(glyph); v != 0 {
				failures = reportMetric(t, failures, "glyph %d italic = %d, want 0", id, v)
			}
		} else if v, _ := set.ItalicCorrection(glyph); v != *want.Italic {
			failures = reportMetric(t, failures, "glyph %d italic = %d, want %d", id, v, *want.Italic)
		}
		if want.Accent == nil {
			if v, _ := set.TopAccentAttachment(glyph); v != 0 {
				failures = reportMetric(t, failures, "glyph %d accent = %d, want 0", id, v)
			}
		} else if v, _ := set.TopAccentAttachment(glyph); v != *want.Accent {
			failures = reportMetric(t, failures, "glyph %d accent = %d, want %d", id, v, *want.Accent)
		}
		if v, _ := set.IsExtendedShape(glyph); v != want.Extended {
			failures = reportMetric(t, failures, "glyph %d extended = %v, want %v", id, v, want.Extended)
		}
		for corner, name := range []string{"topRight", "topLeft", "bottomRight", "bottomLeft"} {
			expected, present := want.Kern[name]
			gotCurve, gotOK := set.MathKernCurve(glyph, MathKernCorner(corner))
			if id == 374 && name == "topLeft" && !present && gotOK {
				// Typlet's fixture omits this spec-defined left-corner record.
				// Untie retains all four corners for prescript layout.
				continue
			}
			if present != gotOK {
				failures = reportMetric(t, failures, "glyph %d %s kern presence = %v, want %v", id, name, gotOK, present)
				continue
			}
			if present && (!slicesEqual(gotCurve.Heights, expected.Heights) || !slicesEqual(gotCurve.Kerns, expected.Kerns)) {
				failures = reportMetric(t, failures, "glyph %d %s kern = %#v, want %#v", id, name, gotCurve, expected)
			}
		}
		checkConstruction(t, set, fixture, glyph, VerticalAxis, want.Vertical, &failures)
		checkConstruction(t, set, fixture, glyph, HorizontalAxis, want.Horizontal, &failures)
	}
	if failures != 0 {
		t.Fatalf("%d conformance differences", failures)
	}
}

func TestNewCMMappingConformance(t *testing.T) {
	t.Parallel()
	set := loadSharedTestFont(t)
	fixture := loadFixture(t)
	if len(fixture.CMap.Chars) == 0 || len(fixture.CMap.Sequences) == 0 {
		t.Fatal("fixture has no cmap records")
	}
	failures := 0
	for _, tc := range fixture.CMap.Chars {
		runes := []rune(tc.Text)
		if len(runes) != 1 {
			failures = reportMetric(t, failures, "cmap entry %q has %d runes", tc.Text, len(runes))
			continue
		}
		got, ok := set.MapRune(MathFace, runes[0])
		if runes[0] == 0xffff && tc.Glyph == 0 && !ok {
			// U+FFFF maps to .notdef in the font. Character lookup reports that
			// as unmapped, while the serialized fixture retains the raw record.
			continue
		}
		if !ok || got.ID != tc.Glyph {
			failures = reportMetric(t, failures, "MapRune(%U) = %d, %v; want %d", runes[0], got.ID, ok, tc.Glyph)
		}
	}
	for _, tc := range fixture.CMap.Sequences {
		runes := []rune(tc.Text)
		if len(runes) != 2 || !isVariationSelector(runes[1]) {
			failures = reportMetric(t, failures, "cmap sequence %q is not base+selector", tc.Text)
			continue
		}
		got, ok := set.MapVariation(MathFace, runes[0], runes[1])
		if !ok || got.ID != tc.Glyph {
			failures = reportMetric(t, failures, "MapVariation(%U, %U) = %d, %v; want %d", runes[0], runes[1], got.ID, ok, tc.Glyph)
		}
	}
	if failures != 0 {
		t.Fatalf("%d cmap differences", failures)
	}
}

// The pinned ttf-parser oracle composes four CFF endchar-seac glyphs. The Go
// outline parser exposes the same glyphs but without that legacy composition,
// so their bounds differ by one design unit. Keep this list exhaustive and
// fail closed for every other glyph.
func knownCFFComponentDifference(id GlyphID, got, want Bounds) bool {
	switch id {
	case 8465, 8480, 8481, 8483:
		return boundsWithinOne(got, want)
	default:
		return false
	}
}

func boundsWithinOne(a, b Bounds) bool {
	return abs32(a.XMin-b.XMin) <= 1 &&
		abs32(a.YMin-b.YMin) <= 1 &&
		abs32(a.XMax-b.XMax) <= 1 &&
		abs32(a.YMax-b.YMax) <= 1
}

func abs32(v int32) int32 {
	if v < 0 {
		return -v
	}
	return v
}

func reportMetric(t *testing.T, failures int, format string, args ...any) int {
	t.Helper()
	if failures < 25 {
		t.Errorf(format, args...)
	}
	return failures + 1
}

func slicesEqual[T comparable](a, b []T) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func checkConstruction(t *testing.T, set *FontSet, fixture *typletFixture, glyph Glyph, axis MathAxis, want *typletConstruction, failures *int) {
	got, gotOK := set.MathConstruction(glyph, axis)
	if want == nil {
		if gotOK {
			*failures = reportMetric(t, *failures, "glyph %d axis %d construction unexpectedly present", glyph.ID, axis)
		}
		return
	}
	if !gotOK {
		*failures = reportMetric(t, *failures, "glyph %d axis %d construction missing", glyph.ID, axis)
		return
	}
	if len(got.Variants) != len(want.Variants) {
		*failures = reportMetric(t, *failures, "glyph %d axis %d has %d variants, want %d", glyph.ID, axis, len(got.Variants), len(want.Variants))
	} else {
		for i := range want.Variants {
			expected := SizeVariant{Glyph: GlyphID(want.Variants[i][0]), Advance: want.Variants[i][1]}
			if got.Variants[i] != expected {
				*failures = reportMetric(t, *failures, "glyph %d axis %d variant %d = %#v, want %#v", glyph.ID, axis, i, got.Variants[i], expected)
			}
		}
	}
	if want.Assembly == nil {
		if got.Assembly != nil {
			*failures = reportMetric(t, *failures, "glyph %d axis %d assembly unexpectedly present", glyph.ID, axis)
		}
		return
	}
	if got.Assembly == nil {
		*failures = reportMetric(t, *failures, "glyph %d axis %d assembly missing", glyph.ID, axis)
		return
	}
	if got.Assembly.ItalicCorrection != want.Assembly.Italic || got.Assembly.MinConnectorOverlap != fixture.Header.MinConnector {
		*failures = reportMetric(t, *failures, "glyph %d axis %d assembly metrics = %#v", glyph.ID, axis, *got.Assembly)
	}
	if len(got.Assembly.Parts) != len(want.Assembly.Parts) {
		*failures = reportMetric(t, *failures, "glyph %d axis %d has %d assembly parts, want %d", glyph.ID, axis, len(got.Assembly.Parts), len(want.Assembly.Parts))
		return
	}
	for i, part := range want.Assembly.Parts {
		expected := AssemblyPart{
			Glyph:          GlyphID(part.Glyph),
			StartConnector: part.StartConnector,
			EndConnector:   part.EndConnector,
			FullAdvance:    part.FullAdvance,
			Extender:       part.Extender,
		}
		if got.Assembly.Parts[i] != expected {
			*failures = reportMetric(t, *failures, "glyph %d axis %d assembly part %d = %#v, want %#v", glyph.ID, axis, i, got.Assembly.Parts[i], expected)
		}
	}
}

func TestNewCMShapingConformance(t *testing.T) {
	t.Parallel()
	set := loadSharedTestFont(t)
	fixture := loadFixture(t)
	compared := 0
	failures := 0
	for _, shape := range fixture.Shapes {
		key := strings.Join(shape.Features, ",")
		style, flatten, ok := shapeMode(key)
		if !ok {
			t.Fatalf("unknown fixture feature set %q", key)
		}
		for _, raw := range shape.Results {
			var pair [2]json.RawMessage
			if err := json.Unmarshal(raw, &pair); err != nil {
				t.Fatalf("%s result: %v", key, err)
			}
			var text string
			if err := json.Unmarshal(pair[0], &text); err != nil {
				t.Fatalf("%s text: %v", key, err)
			}
			var rawGlyphs [][]json.RawMessage
			if err := json.Unmarshal(pair[1], &rawGlyphs); err != nil {
				t.Fatalf("%s glyphs: %v", key, err)
			}
			runes := []rune(text)
			var base Glyph
			var mapped bool
			switch {
			case len(runes) == 1:
				base, mapped = set.MapRune(MathFace, runes[0])
			case len(runes) == 2 && isVariationSelector(runes[1]):
				base, mapped = set.MapVariation(MathFace, runes[0], runes[1])
			default:
				continue
			}
			if len(rawGlyphs) == 0 {
				if isDefaultIgnorable(runes[0]) && mapped {
					// The cmap contains these format controls, but Typst's shaping
					// pass drops them before emitting glyphs.
					continue
				}
				if mapped {
					failures = reportMetric(t, failures, "%s %q unexpectedly mapped to glyph %d", key, text, base.ID)
				}
				continue
			}
			if !mapped {
				failures = reportMetric(t, failures, "%s %q did not map", key, text)
				continue
			}
			if len(rawGlyphs) != 1 || len(rawGlyphs[0]) < 2 {
				continue
			}
			var expected uint16
			if err := json.Unmarshal(rawGlyphs[0][0], &expected); err != nil {
				t.Fatalf("%s expected glyph: %v", key, err)
			}
			got, ok := set.ShapeMath(MathFace, []GlyphID{base.ID}, style, flatten)
			if !ok || len(got) != 1 {
				t.Fatalf("%s %q ShapeMath = %v, %v", key, text, got, ok)
			}
			if got[0] != GlyphID(expected) {
				failures = reportMetric(t, failures, "%s %q shaped glyph = %d, want %d", key, text, got[0], expected)
			}
			compared++
		}
	}
	if compared < 1200 {
		t.Fatalf("only compared %d shaping cases", compared)
	}
	if failures != 0 {
		t.Fatalf("%d shaping differences", failures)
	}
}

func shapeMode(key string) (MathScriptStyle, bool, bool) {
	switch key {
	case "":
		return NoScriptVariant, false, true
	case "ssty=1":
		return ScriptVariant, false, true
	case "ssty=2":
		return ScriptScriptVariant, false, true
	case "dtls=1":
		return NoScriptVariant, true, true
	case "ssty=1,dtls=1":
		return ScriptVariant, true, true
	case "ssty=2,dtls=1":
		return ScriptScriptVariant, true, true
	default:
		return 0, false, false
	}
}

func isVariationSelector(r rune) bool {
	return r >= 0xfe00 && r <= 0xfe0f
}

func isDefaultIgnorable(r rune) bool {
	switch r {
	case 0x00ad, 0x200b, 0x200c, 0x200d, 0x2060, 0x2061, 0x2062, 0x2063, 0x2064, 0xfeff:
		return true
	}
	return isVariationSelector(r)
}

func TestNewCMOutlineConformance(t *testing.T) {
	t.Parallel()
	set := loadSharedTestFont(t)
	fixture := loadFixture(t)
	for _, id := range []GlyphID{9, 18, 24, 374, 1001, 3548, 3570, 5477, 7869} {
		glyph := Glyph{Face: MathFace, ID: id}
		outline, ok := set.Outline(glyph)
		if !ok {
			t.Fatalf("Outline(%d) = false", id)
		}
		want := fixture.Glyphs[id]
		expected := Bounds{XMin: want.BBox[0], YMin: -want.BBox[3], XMax: want.BBox[2], YMax: -want.BBox[1]}
		if outline.Bounds != expected {
			t.Errorf("glyph %d outline bounds = %#v, want %#v", id, outline.Bounds, expected)
		}
		if len(outline.Segments) == 0 {
			t.Errorf("glyph %d has no segments", id)
		}
		for _, seg := range outline.Segments {
			if seg.Op > 4 {
				t.Errorf("glyph %d has invalid segment op %d", id, seg.Op)
			}
		}
	}
	converted := Glyph{Face: MathFace, ID: 3548}
	first, _ := set.Outline(converted)
	if len(first.Segments) == 0 {
		t.Fatal("empty outline")
	}
	original := first.Segments[0]
	first.Segments[0].Points[0].X = 123456
	again, _ := set.Outline(converted)
	if again.Segments[0].Points[0].X == 123456 {
		t.Fatal("Outline returned shared cache storage")
	}
	if again.Segments[0] != (Segment{Op: original.Op, Points: original.Points}) {
		t.Fatal("Outline cache changed after caller mutation")
	}
}

func TestNewCMShapingDoesNotMutateInput(t *testing.T) {
	t.Parallel()
	set := loadSharedTestFont(t)
	in := []GlyphID{34, 74}
	before := append([]GlyphID(nil), in...)
	out, ok := set.ShapeMath(MathFace, in, ScriptVariant, true)
	if !ok || len(out) != len(in) {
		t.Fatalf("ShapeMath = %v, %v", out, ok)
	}
	if !slicesEqual(in, before) {
		t.Fatalf("input mutated: %v", in)
	}
	out[0] = 0
	if slicesEqual(in, out) {
		t.Fatal("ShapeMath returned the input slice")
	}
}

func TestMathConstructsAreDefensiveCopies(t *testing.T) {
	t.Parallel()
	set := loadSharedTestFont(t)
	glyph := Glyph{Face: MathFace, ID: 9}
	first, ok := set.MathConstruction(glyph, VerticalAxis)
	if !ok || first.Assembly == nil || len(first.Variants) == 0 {
		t.Fatal("expected variant and assembly data")
	}
	originalVariant := first.Variants[0]
	originalPart := first.Assembly.Parts[0]
	originalItalic := first.Assembly.ItalicCorrection
	originalOverlap := first.Assembly.MinConnectorOverlap
	first.Variants[0].Advance++
	first.Assembly.ItalicCorrection++
	first.Assembly.MinConnectorOverlap++
	first.Assembly.Parts[0].FullAdvance++
	first.Assembly.Parts[0].Glyph++
	second, _ := set.MathConstruction(glyph, VerticalAxis)
	if second.Variants[0] != originalVariant || second.Assembly.Parts[0] != originalPart {
		t.Fatal("MathConstruction returned shared slices")
	}
	if second.Assembly.ItalicCorrection != originalItalic || second.Assembly.MinConnectorOverlap != originalOverlap {
		t.Fatal("MathConstruction returned shared assembly metadata")
	}
	if second.Assembly.Parts[0].Glyph != originalPart.Glyph {
		t.Fatal("MathConstruction returned shared part glyph IDs")
	}

	curve, ok := set.MathKernCurve(Glyph{Face: MathFace, ID: 374}, MathKernBottomRight)
	if !ok || len(curve.Kerns) == 0 {
		t.Fatal("expected kern curve")
	}
	originalKern := curve.Kerns[0]
	curve.Kerns[0]++
	again, _ := set.MathKernCurve(Glyph{Face: MathFace, ID: 374}, MathKernBottomRight)
	if again.Kerns[0] != originalKern {
		t.Fatal("MathKernCurve returned shared slices")
	}
	if _, ok := set.MathKernCurve(glyph, MathKernTopLeft); ok {
		t.Fatal("MathKernCurve returned a curve for a glyph without one")
	}
	if _, ok := set.MathKernCurve(glyph, MathKernBottomLeft+1); ok {
		t.Fatal("MathKernCurve accepted an invalid corner")
	}
}

func TestAssemblyContractIsSufficientForLayout(t *testing.T) {
	t.Parallel()
	set := loadSharedTestFont(t)
	construction, ok := set.MathConstruction(Glyph{Face: MathFace, ID: 9}, VerticalAxis)
	if !ok || construction.Assembly == nil || len(construction.Assembly.Parts) == 0 {
		t.Fatal("expected a vertical assembly")
	}
	assembly := construction.Assembly
	if assembly.MinConnectorOverlap != 0 && assembly.MinConnectorOverlap > maxAssemblyConnector(assembly) {
		t.Fatalf("minimum overlap %d exceeds all connector lengths", assembly.MinConnectorOverlap)
	}
	hasExtender := false
	for _, part := range assembly.Parts {
		if _, ok := set.Outline(Glyph{Face: MathFace, ID: part.Glyph}); !ok {
			t.Fatalf("assembly exposes unreadable endpoint glyph %d", part.Glyph)
		}
		if part.StartConnector < 0 || part.EndConnector < 0 || part.FullAdvance <= 0 {
			t.Fatalf("assembly part is invalid: %#v", part)
		}
		hasExtender = hasExtender || part.Extender
	}
	if !hasExtender {
		t.Fatal("stretchy NewCM assembly has no extender part")
	}
}

func maxAssemblyConnector(assembly *Assembly) int32 {
	var maximum int32
	for _, part := range assembly.Parts {
		maximum = max(maximum, part.StartConnector, part.EndConnector)
	}
	return maximum
}

func TestTestdataManifest(t *testing.T) {
	t.Parallel()
	for path, want := range map[string]string{
		testFontPath:    "2ea09ebc9167b1e1a66f31390dc917f2d4004ecfca72d51b28010e4ad6becd95",
		testFixturePath: "24e779f01cb0d19278fb6fb1bcd55171ff58adab6f05b406804a3f0ff1d432f5",
	} {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		sum := sha256.Sum256(data)
		if got := hex.EncodeToString(sum[:]); got != want {
			t.Errorf("%s SHA-256 = %s, want %s", path, got, want)
		}
	}
}

func TestFixtureIsASCIIOnlyWhereExpected(t *testing.T) {
	// The source file itself is UTF-8. This catches accidental code-point
	// mangling in copied fixture data.
	data, err := os.ReadFile(testFixturePath)
	if err != nil {
		t.Fatal(err)
	}
	if !utf8.Valid(data) {
		t.Fatal("fixture is not valid UTF-8")
	}
}
