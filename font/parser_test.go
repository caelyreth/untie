package font

import (
	"encoding/binary"
	"strings"
	"testing"
)

func TestPostScriptName(t *testing.T) {
	t.Parallel()
	name := "Untie-Test"
	utf16Name := make([]byte, 0, len(name)*2)
	for _, r := range name {
		var b [2]byte
		binary.BigEndian.PutUint16(b[:], uint16(r))
		utf16Name = append(utf16Name, b[:]...)
	}
	table := make([]byte, 6+12+len(utf16Name))
	binary.BigEndian.PutUint16(table[2:4], 1)
	binary.BigEndian.PutUint16(table[4:6], 18)
	binary.BigEndian.PutUint16(table[6:8], 3)
	binary.BigEndian.PutUint16(table[8:10], 1)
	binary.BigEndian.PutUint16(table[10:12], 0x0409)
	binary.BigEndian.PutUint16(table[12:14], 6)
	binary.BigEndian.PutUint16(table[14:16], uint16(len(utf16Name)))
	copy(table[18:], utf16Name)
	got, err := postScriptName(table)
	if err != nil {
		t.Fatal(err)
	}
	if got != name {
		t.Fatalf("postScriptName = %q, want %q", got, name)
	}
}

func TestPostScriptNameRejectsMalformed(t *testing.T) {
	t.Parallel()
	tests := [][]byte{
		nil,
		{0, 1},
		{0, 2, 0, 0, 0, 0},
		{0, 0, 0, 1, 0, 6},
		{0, 0, 0, 1, 0, 18, 0, 3, 0, 1, 0, 9, 0, 6, 0, 1, 0, 99},
	}
	for _, data := range tests {
		if _, err := postScriptName(data); err == nil {
			t.Errorf("postScriptName(%x) error = nil", data)
		}
	}
}

func TestParseMathRejectsMalformed(t *testing.T) {
	t.Parallel()
	header := make([]byte, 10)
	binary.BigEndian.PutUint16(header[0:2], 1)
	tests := map[string][]byte{
		"short":           {0, 1},
		"wrong version":   {0, 2, 0, 0, 0, 0, 0, 0, 0, 0},
		"bad constants":   append(mathWithOffsets(header, 10, 0, 0), 0, 0),
		"bad glyph info":  mathWithOffsets(header, 0, 9999, 0),
		"bad variants":    mathWithOffsets(header, 0, 0, 9999),
		"short constants": mathWithOffsets(header, 10, 0, 0),
	}
	for name, data := range tests {
		t.Run(name, func(t *testing.T) {
			if _, err := parseMathTable(data); err == nil {
				t.Fatal("parseMathTable() error = nil")
			}
		})
	}
	if _, err := parseAssembly(make([]byte, 6), 0); err == nil {
		t.Fatal("parseAssembly accepted an empty part list")
	}
}

func TestParseGSUBRejectsMalformed(t *testing.T) {
	t.Parallel()
	tests := map[string][]byte{
		"short":         {0, 1},
		"wrong version": {0, 2, 0, 0, 0, 10, 0, 10, 0, 10},
		"bad feature":   {0, 1, 0, 0, 0, 10, 0, 0x27, 0x0f, 10},
		"bad lookup":    {0, 1, 0, 0, 0, 10, 0, 10, 0x27, 0x0f},
	}
	for name, data := range tests {
		t.Run(name, func(t *testing.T) {
			if _, err := parseGSUBTable(data); err == nil {
				t.Fatal("parseGSUBTable() error = nil")
			}
		})
	}
}

func TestValidateGSUBGlyphReferences(t *testing.T) {
	t.Parallel()
	tests := map[string]gsubTable{
		"positive count": {features: map[string][]gsubLookup{
			"dtls": {{subtables: []gsubSubtable{{single: map[GlyphID]GlyphID{1: 2}}}}},
		}},
		"single source": {features: map[string][]gsubLookup{
			"dtls": {{subtables: []gsubSubtable{{single: map[GlyphID]GlyphID{8: 2}}}}},
		}},
		"single replacement": {features: map[string][]gsubLookup{
			"dtls": {{subtables: []gsubSubtable{{single: map[GlyphID]GlyphID{1: 8}}}}},
		}},
		"alternate source": {features: map[string][]gsubLookup{
			"ssty": {{subtables: []gsubSubtable{{alternates: map[GlyphID][]GlyphID{8: {2}}}}}},
		}},
		"alternate replacement": {features: map[string][]gsubLookup{
			"ssty": {{subtables: []gsubSubtable{{alternates: map[GlyphID][]GlyphID{1: {8}}}}}},
		}},
	}
	for name, gsub := range tests {
		t.Run(name, func(t *testing.T) {
			wantErr := name != "positive count"
			if err := gsub.validateGlyphIDs(8); (err != nil) != wantErr {
				t.Fatalf("validateGlyphIDs error = %v, wantErr %v", err, wantErr)
			}
		})
	}
	if err := (gsubTable{}).validateGlyphIDs(0); err == nil {
		t.Fatal("validateGlyphIDs accepted zero glyphs")
	}
}

func TestAlternateSubstitutionRejectsEmptyAlternateSet(t *testing.T) {
	t.Parallel()
	table := make([]byte, 16)
	binary.BigEndian.PutUint16(table[0:2], 1)
	binary.BigEndian.PutUint16(table[2:4], 8)
	binary.BigEndian.PutUint16(table[4:6], 1)
	binary.BigEndian.PutUint16(table[6:8], 14)
	binary.BigEndian.PutUint16(table[8:10], 1)
	binary.BigEndian.PutUint16(table[10:12], 1)
	binary.BigEndian.PutUint16(table[12:14], 10)
	binary.BigEndian.PutUint16(table[14:16], 0)
	if _, err := parseAlternateSubstitution(table, 1); err == nil {
		t.Fatal("parseAlternateSubstitution accepted an empty alternate set")
	}
}

func TestValidateMATHGlyphReferences(t *testing.T) {
	t.Parallel()
	tests := map[string]mathTable{
		"positive count": {
			italic:        map[GlyphID]int32{1: 0},
			topAccent:     map[GlyphID]int32{1: 0},
			kerns:         map[GlyphID]*mathKernRecord{1: {}},
			extendedShape: map[GlyphID]struct{}{1: {}},
			vertical: map[GlyphID]mathConstruction{
				1: {variants: []SizeVariant{{Glyph: 2}}, assembly: &Assembly{Parts: []AssemblyPart{{Glyph: 3}}}},
			},
		},
		"italic":   {italic: map[GlyphID]int32{8: 0}},
		"accent":   {topAccent: map[GlyphID]int32{8: 0}},
		"kern":     {kerns: map[GlyphID]*mathKernRecord{8: {}}},
		"extended": {extendedShape: map[GlyphID]struct{}{8: {}}},
		"construction": {vertical: map[GlyphID]mathConstruction{
			8: {variants: []SizeVariant{{Glyph: 2}}},
		}},
		"variant": {horizontal: map[GlyphID]mathConstruction{
			1: {variants: []SizeVariant{{Glyph: 8}}},
		}},
		"assembly": {vertical: map[GlyphID]mathConstruction{
			1: {assembly: &Assembly{Parts: []AssemblyPart{{Glyph: 8}}}},
		}},
	}
	for name, math := range tests {
		t.Run(name, func(t *testing.T) {
			wantErr := name != "positive count"
			if err := math.validateGlyphIDs(8); (err != nil) != wantErr {
				t.Fatalf("validateGlyphIDs error = %v, wantErr %v", err, wantErr)
			}
		})
	}
	if err := (mathTable{}).validateGlyphIDs(0); err == nil {
		t.Fatal("validateGlyphIDs accepted zero glyphs")
	}
}

func TestEmptyGlyphAndInvalidGlyph(t *testing.T) {
	t.Parallel()
	set := loadSharedTestFont(t)
	outline, ok := set.Outline(Glyph{Face: MathFace, ID: 1})
	if !ok {
		t.Fatal("Outline(space) = false")
	}
	if len(outline.Segments) != 0 || !outline.Bounds.Empty() {
		t.Fatalf("Outline(space) = %#v", outline)
	}
	if _, ok := set.Outline(Glyph{Face: MathFace, ID: 65535}); ok {
		t.Fatal("Outline(out-of-range) = true")
	}
	if _, ok := set.GlyphMetrics(Glyph{Face: TextFace, ID: 1}); ok {
		t.Fatal("metrics for absent text face = true")
	}
}

func TestKernCurveAt(t *testing.T) {
	t.Parallel()
	curve := KernCurve{Heights: []int32{10, 20}, Kerns: []int32{-1, -2, -3}}
	for _, tc := range []struct {
		height int32
		want   int32
	}{
		{5, -1},
		{10, -2},
		{15, -2},
		{20, -3},
		{100, -3},
	} {
		if got := curve.At(tc.height); got != tc.want {
			t.Errorf("At(%d) = %d, want %d", tc.height, got, tc.want)
		}
	}
	if got := (KernCurve{}).At(1); got != 0 {
		t.Fatalf("empty At = %d", got)
	}
}

func TestMathConstantStringAndFallbackRange(t *testing.T) {
	t.Parallel()
	if got := AxisHeight.String(); got != "axisHeight" {
		t.Fatalf("AxisHeight.String() = %q", got)
	}
	if got := MathConstant(255).String(); !strings.Contains(got, "255") {
		t.Fatalf("invalid constant string = %q", got)
	}
	if _, ok := FallbackFace(-1); ok {
		t.Fatal("FallbackFace(-1) = true")
	}
	if _, ok := FallbackFace(maxFallbackFaces); ok {
		t.Fatal("FallbackFace(max) = true")
	}
	last, ok := FallbackFace(maxFallbackFaces - 1)
	if !ok || last == 0 {
		t.Fatalf("FallbackFace(last) = %d, %v", last, ok)
	}
}

func mathWithOffsets(header []byte, constants, glyphInfo, variants uint16) []byte {
	out := append([]byte(nil), header...)
	binary.BigEndian.PutUint16(out[4:6], constants)
	binary.BigEndian.PutUint16(out[6:8], glyphInfo)
	binary.BigEndian.PutUint16(out[8:10], variants)
	return out
}
