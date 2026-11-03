package engine

import (
	"context"
	"slices"
	"testing"

	"github.com/caelyreth/untie/display"
	"github.com/caelyreth/untie/font"
	"github.com/caelyreth/untie/geometry"
	"github.com/caelyreth/untie/internal/layout"
	"github.com/caelyreth/untie/internal/tex"
	"github.com/caelyreth/untie/ir"
	"github.com/caelyreth/untie/source"
)

func TestCenterOnAxisTracksGlyphOrigin(t *testing.T) {
	t.Parallel()

	set, _ := newNewCMEngine(t, source.DefaultLimits())
	adapter, err := newAdapter(set, source.DefaultLimits())
	if err != nil {
		t.Fatal(err)
	}
	glyph, err := adapter.Stretch(layout.StretchRequest{
		GlyphRequest: layout.GlyphRequest{
			Context: context.Background(),
			Text:    "√",
			Style:   layout.Style{Size: layout.Text},
			Font:    ir.FontMath,
		},
		Axis:   layout.Vertical,
		Target: geometry.Em(5 * geometry.EmScale / 4),
	})
	if err != nil {
		t.Fatal(err)
	}

	// NewCM's 1.4 em radical variant has a 0.950/0.451 em ink box. The
	// centered frame rounds to 0.9505/0.4505 em around the 0.25 em axis,
	// moving the frame baseline by -33 fixed-point subunits while the glyph
	// origin stays on the font's original baseline.
	if glyph.ID != 6822 {
		t.Fatalf("stretched radical ID = %d, want 6822", glyph.ID)
	}
	if glyph.Ascent != 62_292 || glyph.Descent != 29_524 {
		t.Fatalf("centered radical metrics = (%d,%d), want (62292,29524)", glyph.Ascent, glyph.Descent)
	}
	if glyph.OriginY != -33 {
		t.Fatalf("centered radical origin offset = %d, want -33", glyph.OriginY)
	}
	base, err := adapter.glyphFromFont(
		font.Glyph{Face: font.MathFace, ID: font.GlyphID(glyph.ID)},
		layout.Style{},
		true,
		"√",
	)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(glyph.Outline, base.Outline) {
		t.Fatal("centerOnAxis baked its origin shift into the font outline")
	}
}

func TestNestedRadicalGlyphOrigins(t *testing.T) {
	t.Parallel()

	_, engine := newNewCMEngine(t, source.DefaultLimits())
	parsed := tex.Parse(`\sqrt{\sqrt{\sqrt{x}}}`, ir.Inline, tex.Options{})
	for _, diagnostic := range parsed.Diagnostics {
		if diagnostic.Severity == source.SeverityError {
			t.Fatalf("parse diagnostics: %+v", parsed.Diagnostics)
		}
	}
	const fixtureSize = float32(11 * 4.0 / 3.0)
	box, err := engine.Layout(context.Background(), parsed.Root, ir.Inline, display.Style{
		Size:      fixtureSize,
		GlyphRefs: true,
	})
	if err != nil {
		t.Fatal(err)
	}

	var refs []display.Item
	for _, item := range box.Items {
		if item.Kind == display.ItemGlyphRef {
			refs = append(refs, item)
		}
	}
	want := []struct {
		id uint16
		x  float32
		y  float32
	}{
		{id: 6822, x: 0, y: -2.361267},
		{id: 6821, x: 14.666672, y: -1.2832489},
		{id: 1001, x: 29.333344, y: -10.626038},
		{id: 3570, x: 41.550568, y: 0},
	}
	if len(refs) != len(want) {
		t.Fatalf("nested radical refs = %d, want %d", len(refs), len(want))
	}
	for i := range want {
		if refs[i].Glyph != want[i].id {
			t.Fatalf("ref[%d] glyph = %d, want %d", i, refs[i].Glyph, want[i].id)
		}
		if !closeExact(refs[i].At.X, want[i].x) || !closeExact(refs[i].At.Y, want[i].y) {
			t.Fatalf("ref[%d] origin = (%v,%v), want (%v,%v)",
				i, refs[i].At.X, refs[i].At.Y, want[i].x, want[i].y)
		}
	}

	paired := 0
	for i := 0; i+1 < len(box.Items); i++ {
		path := box.Items[i]
		ref := box.Items[i+1]
		if path.Kind != display.ItemGlyphPath || ref.Kind != display.ItemGlyphRef ||
			path.Glyph != ref.Glyph || path.FontID != ref.FontID {
			continue
		}
		paired++
		if path.At != ref.At {
			t.Fatalf("nested radical glyph %d path origin %+v differs from ref origin %+v", path.Glyph, path.At, ref.At)
		}
		if path.Bound != ref.Bound {
			t.Fatalf("nested radical glyph %d path bound %+v differs from ref bound %+v", path.Glyph, path.Bound, ref.Bound)
		}
	}
	if paired != len(want) {
		t.Fatalf("path/ref origin pairs = %d, want %d", paired, len(want))
	}
}

func closeExact(got, want float32) bool {
	const tolerance = float32(1) / 1_000_000
	delta := got - want
	return delta <= tolerance && delta >= -tolerance
}
