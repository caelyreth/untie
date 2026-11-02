package engine

import (
	"context"
	"testing"

	"github.com/caelyreth/untie/display"
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
}

func closeExact(got, want float32) bool {
	const tolerance = float32(1) / 1_000_000
	delta := got - want
	return delta <= tolerance && delta >= -tolerance
}
