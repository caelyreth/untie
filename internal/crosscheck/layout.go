package crosscheck

import (
	"context"
	"fmt"
	"math"

	"github.com/caelyreth/untie/display"
	"github.com/caelyreth/untie/font"
	"github.com/caelyreth/untie/internal/engine"
	"github.com/caelyreth/untie/internal/fixture"
	"github.com/caelyreth/untie/ir"
	"github.com/caelyreth/untie/source"
)

// toleranceDIP is Typlet's 0.0015pt rounding tolerance converted once to DIP.
const toleranceDIP = fixture.TypletFrameToleranceDIP

type layoutHarness struct {
	engine *engine.Engine
}

func newLayoutHarness(fontBytes []byte) (*layoutHarness, error) {
	fonts, err := font.Load(font.Fonts{
		Math: font.FontData{Bytes: fontBytes, ID: "NewCMMath-Book"},
	})
	if err != nil {
		return nil, err
	}
	renderer, err := engine.New(fonts, source.DefaultLimits())
	if err != nil {
		return nil, err
	}
	return &layoutHarness{engine: renderer}, nil
}

func (h *layoutHarness) layout(ctx context.Context, root ir.Node, mode ir.Mode, size float64) (*display.Box, error) {
	return h.engine.Layout(ctx, root, mode, display.Style{Size: float32(size)})
}

type glyphView struct {
	Font string
	ID   uint16
	X    float64
	Y    float64
	Size float64
}

type layoutView struct {
	Width     float64
	Ascent    float64
	Descent   float64
	Italic    float64
	Axis      float64
	Overflow  float64
	TopHeight float64
	TopAnchor bool
	Glyphs    []glyphView
}

func outputView(box *display.Box) layoutView {
	view := layoutView{
		Width: float64(box.Width), Ascent: float64(box.Ascent), Descent: float64(box.Descent),
		Italic: float64(box.Italic), Axis: float64(box.AxisHeight), Overflow: float64(box.Overflow),
		TopHeight: float64(box.Ascent + box.Descent),
	}
	for _, item := range box.Items {
		if item.Kind != display.ItemGlyphPath && item.Kind != display.ItemGlyphRef {
			continue
		}
		view.Glyphs = append(view.Glyphs, glyphView{
			Font: item.FontID, ID: item.Glyph,
			X: float64(item.At.X), Y: float64(item.At.Y), Size: float64(item.Em),
		})
	}
	return view
}

func fixtureView(frame fixture.Frame) (layoutView, error) {
	converted, err := fixture.FrameToDIP(frame)
	if err != nil {
		return layoutView{}, err
	}
	if converted.Box == nil {
		return layoutView{}, fmt.Errorf("fixture frame has no box")
	}
	view := layoutView{Width: *converted.Box.Width}
	switch converted.Box.Anchor {
	case "baseline":
		view.Ascent = *converted.Box.Ascent
		view.Descent = *converted.Box.Descent
		view.TopHeight = view.Ascent + view.Descent
	case "top":
		view.TopAnchor = true
		view.TopHeight = *converted.Box.Height
	}
	view.Glyphs = make([]glyphView, len(converted.Glyphs))
	for i, glyph := range converted.Glyphs {
		if glyph.HasTransform {
			return layoutView{}, fmt.Errorf("fixture glyph %d has an unsupported transform", i)
		}
		view.Glyphs[i] = glyphView{
			Font: converted.Fonts[glyph.Font], ID: uint16(glyph.Glyph),
			X: glyph.X, Y: glyph.Y, Size: glyph.Size,
		}
	}
	return view, nil
}

// CompareOutputs compares two renderer outputs in baseline coordinates.
func CompareOutputs(got, want *display.Box) string {
	if got == nil || want == nil {
		return "output: both boxes are required"
	}
	return compareLayoutViews("output", outputView(got), outputView(want), false)
}

// CompareOutputToFixture compares renderer output to a Typlet frame. Both
// values are first expressed in the fixture frame's anchor coordinate system.
func CompareOutputToFixture(got *display.Box, want fixture.Frame) string {
	if got == nil {
		return "fixture: renderer output is nil"
	}
	actual := outputView(got)
	expected, err := fixtureView(want)
	if err != nil {
		return err.Error()
	}
	if expected.TopAnchor {
		actual.YToTop(actual.Ascent)
	}
	return compareLayoutViews("fixture", actual, expected, true)
}

func (v *layoutView) YToTop(baseline float64) {
	for i := range v.Glyphs {
		v.Glyphs[i].Y = baseline + v.Glyphs[i].Y
	}
	v.TopAnchor = true
}

func compareLayoutViews(prefix string, got, want layoutView, fixtureCoords bool) string {
	for _, field := range []struct {
		name string
		got  float64
		want float64
	}{
		{"width", got.Width, want.Width},
	} {
		if !near(field.got, field.want) {
			return fmt.Sprintf("%s.%s: expected %.6f, got %.6f", prefix, field.name, field.want, field.got)
		}
	}
	if fixtureCoords && want.TopAnchor {
		if !near(got.TopHeight, want.TopHeight) {
			return fmt.Sprintf("%s.top-height: expected %.6f, got %.6f", prefix, want.TopHeight, got.TopHeight)
		}
	} else {
		for _, field := range []struct {
			name string
			got  float64
			want float64
		}{
			{"ascent", got.Ascent, want.Ascent},
			{"descent", got.Descent, want.Descent},
			{"italic", got.Italic, want.Italic},
			{"axis-height", got.Axis, want.Axis},
			{"overflow", got.Overflow, want.Overflow},
		} {
			if !near(field.got, field.want) {
				return fmt.Sprintf("%s.%s: expected %.6f, got %.6f", prefix, field.name, field.want, field.got)
			}
		}
	}
	if len(got.Glyphs) != len(want.Glyphs) {
		return fmt.Sprintf("%s.glyphs: expected %d, got %d", prefix, len(want.Glyphs), len(got.Glyphs))
	}
	for i := range want.Glyphs {
		g := got.Glyphs[i]
		w := want.Glyphs[i]
		if g.Font != w.Font || g.ID != w.ID {
			return fmt.Sprintf("%s.glyphs[%d]: expected %s/%d, got %s/%d",
				prefix, i, w.Font, w.ID, g.Font, g.ID)
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
			if !near(field.got, field.want) {
				return fmt.Sprintf("%s.glyphs[%d].%s: expected %.6f, got %.6f",
					prefix, i, field.name, field.want, field.got)
			}
		}
	}
	return ""
}

func near(got, want float64) bool {
	return !math.IsNaN(got) && !math.IsInf(got, 0) && math.Abs(got-want) <= toleranceDIP
}
