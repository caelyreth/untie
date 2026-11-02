package font

import "testing"

func TestFeatureNormalization(t *testing.T) {
	t.Parallel()
	set := shapeTestFont()

	tests := []struct {
		name    string
		glyphs  []GlyphID
		feature Feature
		want    GlyphID
	}{
		{"dtls enabled", []GlyphID{10}, Feature{Tag: "dtls", Value: 1}, 20},
		{"dtls nonzero", []GlyphID{10}, Feature{Tag: "dtls", Value: 2}, 20},
		{"dtls disabled", []GlyphID{10}, Feature{Tag: "dtls", Value: 0}, 10},
		{"ssty first alternate", []GlyphID{10}, Feature{Tag: "ssty", Value: 1}, 30},
		{"ssty second alternate", []GlyphID{10}, Feature{Tag: "ssty", Value: 2}, 31},
		{"ssty disabled", []GlyphID{10}, Feature{Tag: "ssty", Value: 0}, 10},
		{"ssty absent alternate", []GlyphID{10}, Feature{Tag: "ssty", Value: 3}, 10},
		{"flac disabled", []GlyphID{10}, Feature{Tag: "flac", Value: 0}, 10},
		{"flac enabled", []GlyphID{10}, Feature{Tag: "flac", Value: 1}, 10},
		{"short tag", []GlyphID{10}, Feature{Tag: "dtl", Value: 1}, 10},
		{"long tag", []GlyphID{10}, Feature{Tag: "dtlsx", Value: 1}, 10},
		{"padded tag", []GlyphID{10}, Feature{Tag: " dtls", Value: 1}, 10},
		{"case-sensitive tag", []GlyphID{10}, Feature{Tag: "DTLS", Value: 1}, 10},
		{"control byte", []GlyphID{10}, Feature{Tag: "dtl\x00", Value: 1}, 10},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := set.Shape(MathFace, tc.glyphs, tc.feature)
			if !ok || len(got) != 1 {
				t.Fatalf("Shape = %v, %v", got, ok)
			}
			if got[0] != tc.want {
				t.Fatalf("Shape = %d, want %d", got[0], tc.want)
			}
		})
	}
}

func TestShapeMathAppliesDtlsBeforeSsty(t *testing.T) {
	t.Parallel()
	set := shapeTestFont()

	in := []GlyphID{10}
	got, ok := set.ShapeMath(MathFace, in, ScriptVariant, true)
	if !ok || len(got) != 1 {
		t.Fatalf("ShapeMath = %v, %v", got, ok)
	}
	if got[0] != 40 {
		t.Fatalf("ShapeMath = %d, want dtls(10)=20 followed by ssty(20)=40", got[0])
	}

	reverse, ok := set.Shape(MathFace, in,
		Feature{Tag: "ssty", Value: 1},
		Feature{Tag: "dtls", Value: 1},
	)
	if !ok || len(reverse) != 1 {
		t.Fatalf("reverse Shape = %v, %v", reverse, ok)
	}
	if reverse[0] == got[0] {
		t.Fatalf("synthetic order test is not order-sensitive: both orders produced %d", got[0])
	}
}

func TestNewCMShapeMathAppliesBothFeatures(t *testing.T) {
	t.Parallel()
	set := loadSharedTestFont(t)
	base, ok := set.MapRune(MathFace, 'i')
	if !ok {
		t.Fatal("MapRune(i) = false")
	}
	dtls, ok := set.Shape(MathFace, []GlyphID{base.ID}, Feature{Tag: "dtls", Value: 1})
	if !ok || len(dtls) != 1 || dtls[0] == base.ID {
		t.Fatalf("dtls Shape = %v, %v; base glyph %d", dtls, ok, base.ID)
	}
	sstyOne, ok := set.Shape(MathFace, dtls, Feature{Tag: "ssty", Value: 1})
	if !ok || len(sstyOne) != 1 || sstyOne[0] == dtls[0] {
		t.Fatalf("ssty Shape = %v, %v; dtls glyph %d", sstyOne, ok, dtls[0])
	}
	got, ok := set.ShapeMath(MathFace, []GlyphID{base.ID}, ScriptVariant, true)
	if !ok || len(got) != 1 {
		t.Fatalf("ShapeMath = %v, %v", got, ok)
	}
	if got[0] != sstyOne[0] || got[0] != 5361 {
		t.Fatalf("ShapeMath = %d, want %d from dtls then ssty", got[0], sstyOne[0])
	}
}

func shapeTestFont() *FontSet {
	return &FontSet{faces: []*parsedFace{{
		gsub: gsubTable{features: map[string][]gsubLookup{
			"dtls": {{subtables: []gsubSubtable{{
				single: map[GlyphID]GlyphID{
					10: 20,
					30: 31,
				},
			}}}},
			"ssty": {{subtables: []gsubSubtable{{
				alternates: map[GlyphID][]GlyphID{
					10: {30, 31},
					20: {40},
				},
			}}}},
		}},
	}}}
}
