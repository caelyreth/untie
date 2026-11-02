package fixture

import (
	"fmt"
	"math"
	"strconv"
	"strings"
)

// FrameDiff is the first deterministic difference between two frames.
type FrameDiff struct {
	FormulaID string
	Path      string
	Expected  string
	Actual    string
	Message   string
}

// String includes the formula ID, path, expected value, and actual value.
func (d *FrameDiff) String() string {
	if d == nil {
		return ""
	}
	if d.Message != "" {
		return fmt.Sprintf("fixture %s: %s: %s (expected %s, got %s)", d.FormulaID, d.Path, d.Message, d.Expected, d.Actual)
	}
	return fmt.Sprintf("fixture %s: %s: expected %s, got %s", d.FormulaID, d.Path, d.Expected, d.Actual)
}

// CompareFrames compares frames in the same unit using tolerance. A non-nil
// result is deterministic: it always reports the first differing field in
// box, glyph, then shape order.
func CompareFrames(formulaID string, got, want Frame, tolerance float64) *FrameDiff {
	if math.IsNaN(tolerance) || math.IsInf(tolerance, 0) || tolerance < 0 {
		return &FrameDiff{
			FormulaID: formulaID,
			Path:      "tolerance",
			Expected:  "finite non-negative",
			Actual:    strconv.FormatFloat(tolerance, 'g', -1, 64),
			Message:   "invalid comparison tolerance",
		}
	}
	if diff := compareBox(formulaID, got.Box, want.Box, tolerance); diff != nil {
		return diff
	}
	if diff := compareNumber(formulaID, "fontSize", got.FontSize, want.FontSize, tolerance); diff != nil {
		return diff
	}
	for i := range want.Page {
		if diff := compareNumber(formulaID, fmt.Sprintf("page[%d]", i), got.Page[i], want.Page[i], tolerance); diff != nil {
			return diff
		}
	}
	if len(got.Fonts) != len(want.Fonts) {
		return countDiff(formulaID, "fonts", len(got.Fonts), len(want.Fonts))
	}
	for i := range want.Fonts {
		if got.Fonts[i] != want.Fonts[i] {
			return valueDiff(formulaID, fmt.Sprintf("fonts[%d]", i), want.Fonts[i], got.Fonts[i])
		}
	}

	if len(got.Glyphs) != len(want.Glyphs) {
		return countDiff(formulaID, "glyphs", len(got.Glyphs), len(want.Glyphs))
	}
	for i := range want.Glyphs {
		if diff := compareGlyph(formulaID, i, got, want, tolerance); diff != nil {
			return diff
		}
	}

	if len(got.Shapes) != len(want.Shapes) {
		return countDiff(formulaID, "shapes", len(got.Shapes), len(want.Shapes))
	}
	for i := range want.Shapes {
		if diff := compareShape(formulaID, i, got.Shapes[i], want.Shapes[i], tolerance); diff != nil {
			return diff
		}
	}
	return nil
}

func compareBox(id string, got, want *FrameBox, tolerance float64) *FrameDiff {
	if got == nil || want == nil {
		if got == nil && want == nil {
			return nil
		}
		return valueDiff(id, "box", fmt.Sprint(want != nil), fmt.Sprint(got != nil))
	}
	if got.Anchor != want.Anchor {
		return valueDiff(id, "box.anchor", want.Anchor, got.Anchor)
	}
	for _, field := range []struct {
		name string
		got  *float64
		want *float64
	}{
		{"box.width", got.Width, want.Width},
		{"box.height", got.Height, want.Height},
		{"box.ascent", got.Ascent, want.Ascent},
		{"box.descent", got.Descent, want.Descent},
	} {
		if (field.got == nil) != (field.want == nil) {
			return valueDiff(id, field.name, optionalPresence(field.want), optionalPresence(field.got))
		}
		if field.want != nil {
			if diff := compareNumber(id, field.name, *field.got, *field.want, tolerance); diff != nil {
				return diff
			}
		}
	}
	return nil
}

func compareGlyph(id string, index int, got, want Frame, tolerance float64) *FrameDiff {
	g := got.Glyphs[index]
	w := want.Glyphs[index]
	path := fmt.Sprintf("glyphs[%d]", index)
	if w.Text != "" {
		path += fmt.Sprintf(" (%q)", w.Text)
	}
	if w.Font < 0 || w.Font >= len(want.Fonts) {
		return valueDiff(id, path+".font", "<valid font index>", strconv.Itoa(w.Font))
	}
	if g.Font < 0 || g.Font >= len(got.Fonts) {
		return valueDiff(id, path+".font", want.Fonts[w.Font], fmt.Sprintf("invalid index %d", g.Font))
	}
	if got.Fonts[g.Font] != want.Fonts[w.Font] {
		return valueDiff(id, path+".font", want.Fonts[w.Font], got.Fonts[g.Font])
	}
	if g.Glyph != w.Glyph {
		return valueDiff(id, path+".id", fmt.Sprint(w.Glyph), fmt.Sprint(g.Glyph))
	}
	if g.Text != w.Text {
		return valueDiff(id, path+".text", w.Text, g.Text)
	}
	for _, field := range []struct {
		name string
		got  float64
		want float64
	}{
		{"x", g.X, w.X},
		{"y", g.Y, w.Y},
		{"size", g.Size, w.Size},
	} {
		if diff := compareNumber(id, path+"."+field.name, field.got, field.want, tolerance); diff != nil {
			return diff
		}
	}
	gotFill, wantFill := "#000000", "#000000"
	if g.HasFill {
		gotFill = g.Fill
	}
	if w.HasFill {
		wantFill = w.Fill
	}
	if gotFill != wantFill {
		return valueDiff(id, path+".fill", wantFill, gotFill)
	}

	gotTransform := [4]float64{1, 0, 0, 1}
	wantTransform := [4]float64{1, 0, 0, 1}
	if g.HasTransform {
		gotTransform = g.Transform
	}
	if w.HasTransform {
		wantTransform = w.Transform
	}
	for i := range wantTransform {
		if !near(gotTransform[i], wantTransform[i], tolerance) {
			return valueDiff(id, path+".transform",
				formatFloats(wantTransform[:]), formatFloats(gotTransform[:]))
		}
	}
	return nil
}

func compareShape(id string, index int, got, want FrameShape, tolerance float64) *FrameDiff {
	path := fmt.Sprintf("shapes[%d]", index)
	gotGeometry := got.ShapeGeometry()
	wantGeometry := want.ShapeGeometry()
	if gotGeometry != wantGeometry {
		return valueDiff(id, path+".geometry", wantGeometry, gotGeometry)
	}
	gotValues := got.ShapeGeometryValues()
	wantValues := want.ShapeGeometryValues()
	if len(gotValues) != len(wantValues) {
		return countDiff(id, path+"."+wantGeometry, len(gotValues), len(wantValues))
	}
	for i := range wantValues {
		if !near(gotValues[i], wantValues[i], tolerance) {
			return valueDiff(id, fmt.Sprintf("%s.%s[%d]", path, wantGeometry, i),
				formatFloat(wantValues[i]), formatFloat(gotValues[i]))
		}
	}
	if got.HasFill != want.HasFill {
		return valueDiff(id, path+".fill.present", fmt.Sprint(want.HasFill), fmt.Sprint(got.HasFill))
	}
	if want.HasFill && got.Fill != want.Fill {
		return valueDiff(id, path+".fill", want.Fill, got.Fill)
	}
	if (got.Stroke == nil) != (want.Stroke == nil) {
		return valueDiff(id, path+".stroke.present", fmt.Sprint(want.Stroke != nil), fmt.Sprint(got.Stroke != nil))
	}
	if want.Stroke != nil {
		if got.Stroke.Color != want.Stroke.Color {
			return valueDiff(id, path+".stroke.color", want.Stroke.Color, got.Stroke.Color)
		}
		if !near(got.Stroke.Thickness, want.Stroke.Thickness, tolerance) {
			return valueDiff(id, path+".stroke.thickness",
				formatFloat(want.Stroke.Thickness), formatFloat(got.Stroke.Thickness))
		}
	}
	return nil
}

func compareNumber(id, path string, got, want, tolerance float64) *FrameDiff {
	if near(got, want, tolerance) {
		return nil
	}
	return valueDiff(id, path, formatFloat(want), formatFloat(got))
}

func near(a, b, tolerance float64) bool {
	return math.Abs(a-b) <= tolerance
}

func valueDiff(id, path, want, got string) *FrameDiff {
	return &FrameDiff{FormulaID: id, Path: path, Expected: want, Actual: got}
}

func countDiff(id, path string, got, want int) *FrameDiff {
	return valueDiff(id, path+".length", strconv.Itoa(want), strconv.Itoa(got))
}

func optionalPresence(value *float64) string {
	if value == nil {
		return "<absent>"
	}
	return "<present: " + formatFloat(*value) + ">"
}

func formatFloat(value float64) string {
	return strconv.FormatFloat(value, 'g', -1, 64)
}

func formatFloats(values []float64) string {
	parts := make([]string, len(values))
	for i, value := range values {
		parts[i] = formatFloat(value)
	}
	return "[" + strings.Join(parts, ", ") + "]"
}
