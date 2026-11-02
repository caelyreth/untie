package layout

import (
	"testing"

	"github.com/caelyreth/untie/display"
	"github.com/caelyreth/untie/geometry"
	"github.com/caelyreth/untie/ir"
	"github.com/caelyreth/untie/source"
)

type recordingTextSource struct {
	tracking geometry.Em
}

func (r *recordingTextSource) ShapeText(req TextRequest) (TextRun, error) {
	r.tracking = req.Tracking
	return TextRun{
		Width:   em(1, 2) + req.Tracking,
		Ascent:  em(7, 10),
		Descent: em(2, 10),
		Glyphs: []PositionedGlyph{{
			Glyph: Glyph{
				ID:            1,
				FontID:        "recording-text",
				Advance:       em(1, 2),
				Ascent:        em(7, 10),
				Descent:       em(2, 10),
				ExactGlyphRef: true,
			},
		}},
	}, nil
}

func TestTextTrackingUsesUnscaledBaseEms(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		tracking ir.Length
		want     geometry.Em
	}{
		{"em", irEm(0.1), em(1, 10)},
		{"mu", mustMuLength(18), em(1, 1)},
		{"ex", mustExLength(t, 0.5), em(9, 40)},
		{"dip", mustDipLength(t, 1.6), em(1, 8)},
		{"pt", mustPtLength(t, 7.2), em(3, 4)},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			textSource := &recordingTextSource{}
			renderer, err := New(Fonts{
				Glyphs:  &fakeGlyphSource{glyphs: map[string]Glyph{}},
				Metrics: fakeStyleFont{},
				Text:    textSource,
			}, Options{})
			if err != nil {
				t.Fatal(err)
			}
			text := ir.NewText(source.NewRange(0, 1), "x")
			text.Tracking = test.tracking
			style := ir.NewStyle(source.NewRange(0, 1), text)
			style.SetSize = true
			style.NewSize = ir.SizeScript
			box, err := renderer.Layout(style, ir.Inline, display.Style{Size: 16})
			if err != nil {
				t.Fatal(err)
			}
			if !closeHost(hostEm(t, textSource.tracking, 16), hostEm(t, test.want, 16)) {
				t.Fatalf("tracking = %d, want %d", textSource.tracking, test.want)
			}
			if box.Width <= 0 {
				t.Fatalf("box width = %v", box.Width)
			}
		})
	}
}

func TestAbsoluteTextTrackingIsStyleInvariant(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		tracking ir.Length
		want     float32
	}{
		{"relative em", irEm(0.1), 7.68},
		{"absolute dip", mustDipLength(t, 1.6), 8},
		{"absolute pt", mustPtLength(t, 7.2), 16},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			textSource := &recordingTextSource{}
			renderer, err := New(Fonts{
				Glyphs:  &fakeGlyphSource{glyphs: map[string]Glyph{}},
				Metrics: fakeStyleFont{},
				Text:    textSource,
			}, Options{})
			if err != nil {
				t.Fatal(err)
			}
			text := ir.NewText(source.NewRange(0, 1), "x")
			text.Tracking = test.tracking
			style := ir.NewStyle(source.NewRange(0, 1), text)
			style.SetSize = true
			style.NewSize = ir.SizeScript
			box, err := renderer.Layout(style, ir.Inline, display.Style{Size: 16})
			if err != nil {
				t.Fatal(err)
			}
			if !closeHost(box.Width, test.want) {
				t.Fatalf("script width = %v, want %v", box.Width, test.want)
			}
		})
	}
}

func mustDipLength(t *testing.T, value float64) ir.Length {
	t.Helper()
	length, err := ir.DipLength(value)
	if err != nil {
		t.Fatal(err)
	}
	return length
}

func mustPtLength(t *testing.T, value float64) ir.Length {
	t.Helper()
	length, err := ir.PtLength(value)
	if err != nil {
		t.Fatal(err)
	}
	return length
}

func mustExLength(t *testing.T, value float64) ir.Length {
	t.Helper()
	length, err := ir.ExLength(value)
	if err != nil {
		t.Fatal(err)
	}
	return length
}

func hostEm(t *testing.T, value geometry.Em, size float32) float32 {
	t.Helper()
	state := &layoutState{hostSize: int64(float64(size) * float64(hostSubunits))}
	host, err := state.host(value)
	if err != nil {
		t.Fatal(err)
	}
	return host
}
