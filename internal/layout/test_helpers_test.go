package layout

import (
	"sync"
	"testing"

	"github.com/caelyreth/untie/display"
	"github.com/caelyreth/untie/geometry"
	"github.com/caelyreth/untie/ir"
	"github.com/caelyreth/untie/source"
)

type fakeGlyphSource struct {
	glyphs map[string]Glyph
	strict bool
	seen   map[string][]string
	mu     sync.Mutex
}

func (f *fakeGlyphSource) Glyph(req GlyphRequest) (Glyph, bool) {
	f.mu.Lock()
	if f.seen == nil {
		f.seen = make(map[string][]string)
	}
	f.seen[req.Text] = append([]string(nil), req.Features...)
	glyph, ok := f.glyphs[req.Text]
	f.mu.Unlock()
	if ok {
		glyph.Text = req.Text
		return glyph, true
	}
	if f.strict {
		return Glyph{}, false
	}
	extra := len(req.Text) - 1
	if extra < 0 {
		extra = 0
	}
	advance := em(1, 2) + geometry.Em(int64(extra)*geometry.EmScale/10)
	id := uint16(0)
	for _, r := range req.Text {
		id = id*31 + uint16(r)
	}
	return Glyph{
		ID:        id,
		FontID:    "test-math",
		Text:      req.Text,
		Advance:   advance,
		Ascent:    em(7, 10),
		Descent:   em(2, 10),
		Italic:    em(1, 20),
		TopAccent: em(1, 4),
		TextLike:  true,
		Outline: Path{
			{Op: geometry.SegMoveTo, Points: [3]Point{{X: 0, Y: -em(2, 10)}}},
			{Op: geometry.SegLineTo, Points: [3]Point{{X: advance, Y: -em(2, 10)}}},
			{Op: geometry.SegLineTo, Points: [3]Point{{X: advance, Y: em(7, 10)}}},
			{Op: geometry.SegLineTo, Points: [3]Point{{X: 0, Y: em(7, 10)}}},
			{Op: geometry.SegClose},
		},
	}, true
}

type fakeStyleFont struct{}

func (fakeStyleFont) MathMetrics(style Style, variant ir.Variant) MathMetrics {
	return testMetrics()
}

type fakeStretcher struct {
	base  *fakeGlyphSource
	calls int
}

func (f *fakeStretcher) Stretch(req StretchRequest) (Glyph, bool) {
	f.calls++
	glyph, ok := f.base.Glyph(req.GlyphRequest)
	if !ok {
		return Glyph{}, false
	}
	if req.Axis == Horizontal && req.Target > glyph.Advance {
		glyph.Advance = req.Target
		glyph.Outline = Path{
			{Op: geometry.SegMoveTo, Points: [3]Point{{X: 0, Y: -glyph.Descent}}},
			{Op: geometry.SegLineTo, Points: [3]Point{{X: glyph.Advance, Y: -glyph.Descent}}},
			{Op: geometry.SegLineTo, Points: [3]Point{{X: glyph.Advance, Y: glyph.Ascent}}},
			{Op: geometry.SegLineTo, Points: [3]Point{{X: 0, Y: glyph.Ascent}}},
			{Op: geometry.SegClose},
		}
	}
	if req.Axis == Vertical && req.Target > glyph.Ascent+glyph.Descent {
		glyph.Ascent = req.Target * 3 / 4
		glyph.Descent = req.Target - glyph.Ascent
		glyph.Outline = Path{
			{Op: geometry.SegMoveTo, Points: [3]Point{{X: 0, Y: -glyph.Descent}}},
			{Op: geometry.SegLineTo, Points: [3]Point{{X: glyph.Advance, Y: -glyph.Descent}}},
			{Op: geometry.SegLineTo, Points: [3]Point{{X: glyph.Advance, Y: glyph.Ascent}}},
			{Op: geometry.SegLineTo, Points: [3]Point{{X: 0, Y: glyph.Ascent}}},
			{Op: geometry.SegClose},
		}
	}
	return glyph, true
}

type fakeTextSource struct{}

func (fakeTextSource) ShapeText(req TextRequest) (TextRun, bool) {
	advance := em(1, 2)
	if len(req.Text) > 0 {
		advance = geometry.Em(int64(len([]rune(req.Text))) * geometry.EmScale / 2)
	}
	return TextRun{
		Width:   advance,
		Ascent:  em(7, 10),
		Descent: em(2, 10),
		Glyphs: []PositionedGlyph{{
			Glyph: Glyph{
				ID:       1,
				FontID:   "test-text",
				Text:     req.Text,
				Advance:  advance,
				Ascent:   em(7, 10),
				Descent:  em(2, 10),
				TextLike: true,
			},
		}},
	}, true
}

type fakeTableLayouter struct {
	called bool
	req    TableRequest
}

func (f *fakeTableLayouter) LayoutTable(req TableRequest) (Fragment, bool) {
	f.called = true
	f.req = req
	width := geometry.Em(0)
	ascent, descent := geometry.Em(0), geometry.Em(0)
	for _, row := range req.Rows {
		for _, cell := range row.Cells {
			width = max(width, cell.Width)
			ascent = max(ascent, cell.Ascent)
			descent = max(descent, cell.Descent)
		}
		if row.Tag != nil {
			width = max(width, row.Tag.Width)
			ascent = max(ascent, row.Tag.Ascent)
			descent = max(descent, row.Tag.Descent)
		}
	}
	fragment, err := newFragment(Metrics{Width: width, Ascent: ascent, Descent: descent}, ir.ClassInner, Text, geometry.Em(geometry.EmScale))
	if err != nil {
		return Fragment{}, false
	}
	return *fragment, true
}

func testMetrics() MathMetrics {
	return MathMetrics{
		ScriptPercentScaleDown:        80,
		ScriptScriptPercentScaleDown:  60,
		SpaceWidth:                    em(1, 4),
		XHeight:                       em(9, 20),
		AxisHeight:                    em(1, 4),
		SuperscriptShiftUp:            em(9, 20),
		SuperscriptShiftUpCramped:     em(7, 20),
		SuperscriptBottomMin:          em(1, 10),
		SuperscriptBaselineDropMax:    em(3, 10),
		SubscriptShiftDown:            em(1, 5),
		SubscriptTopMax:               em(7, 10),
		SubscriptBaselineDropMin:      em(1, 10),
		SubSuperscriptGapMin:          em(1, 10),
		SuperscriptBottomMaxWithSub:   em(6, 10),
		SpaceAfterScript:              em(1, 20),
		UpperLimitGapMin:              em(1, 10),
		UpperLimitBaselineRiseMin:     em(3, 5),
		LowerLimitGapMin:              em(1, 10),
		LowerLimitBaselineDropMin:     em(3, 10),
		StretchStackTopShiftUp:        em(4, 5),
		StretchStackBottomShiftDown:   em(4, 5),
		StretchStackGapAboveMin:       em(1, 10),
		StretchStackGapBelowMin:       em(1, 10),
		StackTopShiftUp:               em(1, 2),
		StackTopDisplayStyleShiftUp:   em(7, 10),
		StackBottomShiftDown:          em(1, 2),
		StackBottomDisplayShiftDown:   em(7, 10),
		StackGapMin:                   em(3, 20),
		StackDisplayStyleGapMin:       em(1, 5),
		FractionNumeratorShiftUp:      em(3, 10),
		FractionNumeratorDisplayUp:    em(1, 2),
		FractionDenominatorShiftDown:  em(2, 5),
		FractionDenominatorDisplayDn:  em(3, 5),
		FractionNumeratorGapMin:       em(1, 20),
		FractionNumeratorDisplayGap:   em(1, 10),
		FractionRuleThickness:         em(1, 20),
		FractionDenominatorGapMin:     em(1, 20),
		FractionDenominatorDisplayGap: em(1, 10),
		SkewedFractionVerticalGap:     em(1, 10),
		SkewedFractionHorizontalGap:   em(1, 2),
		OverbarVerticalGap:            em(3, 20),
		OverbarRuleThickness:          em(1, 20),
		OverbarExtraAscender:          em(1, 20),
		UnderbarVerticalGap:           em(3, 20),
		UnderbarRuleThickness:         em(1, 20),
		UnderbarExtraDescender:        em(1, 20),
		RadicalVerticalGap:            em(1, 10),
		RadicalDisplayStyleGap:        em(1, 5),
		RadicalRuleThickness:          em(1, 20),
		RadicalExtraAscender:          em(1, 20),
		RadicalKernBeforeDegree:       em(1, 10),
		RadicalKernAfterDegree:        -em(1, 10),
		RadicalDegreeBottomRaisePct:   60,
	}
}

func newTestRenderer(t testing.TB) (*Renderer, *fakeGlyphSource, *fakeStretcher) {
	t.Helper()
	glyphs := &fakeGlyphSource{glyphs: map[string]Glyph{}}
	stretcher := &fakeStretcher{base: glyphs}
	renderer, err := New(Fonts{
		Glyphs:  glyphs,
		Metrics: fakeStyleFont{},
		Stretch: stretcher,
		Text:    fakeTextSource{},
	}, Options{})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	return renderer, glyphs, stretcher
}

func em(n, d int64) geometry.Em {
	value, err := geometry.EmFromParts(n, d)
	if err != nil {
		panic(err)
	}
	return value
}

func irEm(value float64) ir.Length {
	length, err := ir.EmLength(value)
	if err != nil {
		panic(err)
	}
	return length
}

func mustMuLength(value float64) ir.Length {
	length, err := ir.MuLength(value)
	if err != nil {
		panic(err)
	}
	return length
}

func glyph(text string, class ir.MathClass) *ir.Glyph {
	node := ir.NewGlyph(source.NewRange(0, 1), text)
	node.Class = class
	return node
}

func testStyle(size float32) display.Style {
	return display.Style{Size: size}
}

func closeHost(got, want float32) bool {
	tolerance := float32(1) / float32(hostSubunits)
	diff := got - want
	return diff <= tolerance && diff >= -tolerance
}
