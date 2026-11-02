package fixture

import (
	"strings"
	"testing"
)

func TestCompareFramesToleranceAndDiagnostics(t *testing.T) {
	t.Parallel()
	want := testFrame()
	got := testFrame()
	got.Glyphs[0].X += 0.001
	if diff := CompareFrames("formula", got, want, TypletFrameTolerancePoints); diff != nil {
		t.Fatalf("small difference rejected: %s", diff)
	}

	got.Glyphs[0].X += 0.001
	diff := CompareFrames("formula", got, want, TypletFrameTolerancePoints)
	if diff == nil {
		t.Fatal("expected a glyph difference")
	}
	if diff.FormulaID != "formula" || diff.Path != `glyphs[0] ("x").x` {
		t.Fatalf("unexpected diff path: %#v", diff)
	}
	text := diff.String()
	if !strings.Contains(text, "formula") || !strings.Contains(text, "glyphs[0]") ||
		!strings.Contains(text, "expected") || !strings.Contains(text, "got") {
		t.Fatalf("diff is not actionable: %s", text)
	}
	if again := CompareFrames("formula", got, want, TypletFrameTolerancePoints).String(); again != text {
		t.Fatalf("diff is not deterministic:\n%s\n%s", text, again)
	}
}

func TestCompareFramesDefaultsAndShapeDiff(t *testing.T) {
	t.Parallel()
	want := testFrame()
	got := testFrame()
	got.Glyphs[0].HasFill = false
	got.Glyphs[0].Fill = ""
	got.Glyphs[0].HasTransform = false
	if diff := CompareFrames("defaults", got, want, TypletFrameTolerancePoints); diff != nil {
		t.Fatalf("default fill/transform should match: %s", diff)
	}

	got.Shapes[0].Stroke.Thickness += 0.1
	diff := CompareFrames("shapes", got, want, TypletFrameTolerancePoints)
	if diff == nil || diff.Path != "shapes[0].stroke.thickness" {
		t.Fatalf("unexpected shape diff: %#v", diff)
	}
}

func TestCompareFramesRejectsInvalidFontIndexWithoutPanicking(t *testing.T) {
	t.Parallel()
	got := testFrame()
	got.Glyphs[0].Font = 99
	diff := CompareFrames("bad-font", got, testFrame(), TypletFrameTolerancePoints)
	if diff == nil || !strings.Contains(diff.Actual, "invalid index 99") {
		t.Fatalf("unexpected diff: %#v", diff)
	}
}

func testFrame() Frame {
	return Frame{
		Box: &FrameBox{
			Anchor: "baseline", Width: ptr(10), Ascent: ptr(7), Descent: ptr(1),
		},
		FontSize: 11,
		Fonts:    []string{"NewCMMath-Book"},
		Glyphs: []FrameGlyph{{
			Font: 0, Glyph: 3548, X: 0, Y: 0, Size: 11, Text: "x",
			Fill: "#000000", HasFill: true,
			Transform: [4]float64{1, 0, 0, 1}, HasTransform: true,
		}},
		Shapes: []FrameShape{{
			Line:   []float64{0, 1, 10, 1},
			Stroke: &ShapeStroke{Color: "#000000", Thickness: 0.5},
		}},
	}
}
