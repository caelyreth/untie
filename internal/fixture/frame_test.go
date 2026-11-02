package fixture

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestFrameGlyphRejectsOutOfRangeIDsBeforeCast(t *testing.T) {
	t.Parallel()

	for _, raw := range []string{
		`[0,-1,0,0,11,"x"]`,
		`[0,65536,0,0,11,"x"]`,
		`[0,4294967297,0,0,11,"x"]`,
		`[0,1.5,0,0,11,"x"]`,
	} {
		var glyph FrameGlyph
		err := json.Unmarshal([]byte(raw), &glyph)
		if err == nil {
			t.Errorf("glyph %s unexpectedly accepted as %#v", raw, glyph)
		}
	}

	for _, raw := range []string{
		`[0,0,0,0,11,"x"]`,
		`[0,65535,0,0,11,"x"]`,
	} {
		var glyph FrameGlyph
		if err := json.Unmarshal([]byte(raw), &glyph); err != nil {
			t.Errorf("glyph %s rejected: %v", raw, err)
		}
	}
}

func TestCompareFramesRejectsNegativeFontIndicesWithoutPanicking(t *testing.T) {
	t.Parallel()

	for _, name := range []string{"actual", "expected"} {
		name := name
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			got, want := testFrame(), testFrame()
			if name == "actual" {
				got.Glyphs[0].Font = -1
			} else {
				want.Glyphs[0].Font = -1
			}
			diff := CompareFrames("negative-font", got, want, TypletFrameTolerancePoints)
			if diff == nil || !strings.HasSuffix(diff.Path, ".font") {
				t.Fatalf("unexpected diff: %#v", diff)
			}
		})
	}
}
