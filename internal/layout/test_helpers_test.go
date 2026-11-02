package layout

import (
	"fmt"
	"sync"
	"testing"

	"github.com/caelyreth/untie/display"
	"github.com/caelyreth/untie/geometry"
	"github.com/caelyreth/untie/ir"
	"github.com/caelyreth/untie/source"
)

type displayLineInput struct {
	Baseline float32
	OffsetX  float32
	Box      *display.Box
	Tag      *display.TagBox
}

// assembleDisplayLinesForTest preserves the old line-local fixture shape for
// focused geometry tests while asserting the range-based production contract.
func assembleDisplayLinesForTest(
	state *layoutState,
	lines []displayLineInput,
) (*display.Box, error) {
	if len(lines) == 0 {
		return nil, fmt.Errorf("%w: display lines are empty", ErrInvalidInput)
	}
	if _, err := state.addHost(state.request.MaxWidth, 0); err != nil {
		return nil, err
	}
	parent := &display.Box{
		Width:      state.request.MaxWidth,
		AxisHeight: lines[0].Box.AxisHeight,
		Lines:      make([]display.LineBox, len(lines)),
	}
	var baseline float32
	for i, input := range lines {
		if input.Box == nil {
			return nil, fmt.Errorf("%w: nil display line", ErrInvalidInput)
		}
		if i > 0 {
			step, err := state.addHost(lines[i-1].Box.Descent, input.Box.Ascent)
			if err != nil {
				return nil, err
			}
			baseline, err = state.addHost(baseline, step)
			if err != nil {
				return nil, err
			}
		}
		offsetX, err := state.displayLineOffset(input.Box.Width)
		if err != nil {
			return nil, err
		}
		line := display.LineBox{
			Baseline: baseline,
			OffsetX:  offsetX,
			Width:    input.Box.Width,
			Ascent:   input.Box.Ascent,
			Descent:  input.Box.Descent,
			Overflow: input.Box.Overflow,
			Items:    display.ItemRange{Start: len(parent.Items)},
			Hits:     display.HitRange{Start: len(parent.Hits)},
			Tag:      input.Tag,
		}
		for _, item := range input.Box.Items {
			translated, err := state.offsetItem(cloneDisplayItemForTest(item), offsetX, baseline)
			if err != nil {
				return nil, err
			}
			parent.Items = append(parent.Items, translated)
		}
		for _, hit := range input.Box.Hits {
			hit.Area.X += offsetX
			hit.Area.Y += baseline
			parent.Hits = append(parent.Hits, hit)
		}
		line.Items.Count = len(parent.Items) - line.Items.Start
		line.Hits.Count = len(parent.Hits) - line.Hits.Start
		parent.Lines[i] = line
		parent.Italic = input.Box.Italic
		top := baseline - input.Box.Ascent
		parent.Ascent = maxFloat(parent.Ascent, maxFloat(0, -top))
		parent.Descent = maxFloat(parent.Descent, baseline+input.Box.Descent)
	}
	overflow, err := state.displayItemsOverflow(parent.Items, parent.Width, parent.Ascent, parent.Descent)
	if err != nil {
		return nil, err
	}
	for _, line := range parent.Lines {
		overflow = maxFloat(overflow, maxFloat(0, line.Width-parent.Width))
	}
	parent.Overflow = overflow
	return parent, nil
}

func lineBoxForTest(box *display.Box, line display.LineBox) *display.Box {
	if box == nil {
		return nil
	}
	out := &display.Box{
		Width:    line.Width,
		Ascent:   line.Ascent,
		Descent:  line.Descent,
		Overflow: line.Overflow,
		Items:    make([]display.Item, line.Items.Count),
		Hits:     make([]display.Hit, line.Hits.Count),
	}
	for i := range out.Items {
		out.Items[i] = cloneDisplayItemForTest(box.Items[line.Items.Start+i])
		translateItemForTest(&out.Items[i], -line.OffsetX, -line.Baseline)
	}
	copy(out.Hits, box.Hits[line.Hits.Start:line.Hits.End()])
	for i := range out.Hits {
		out.Hits[i].Area.X -= line.OffsetX
		out.Hits[i].Area.Y -= line.Baseline
	}
	return out
}

func cloneDisplayItemForTest(item display.Item) display.Item {
	item.Path = append([]geometry.Segment(nil), item.Path...)
	return item
}

func translateItemForTest(item *display.Item, dx, dy float32) {
	item.Bound.X += dx
	item.Bound.Y += dy
	item.Rect.X += dx
	item.Rect.Y += dy
	item.At.X += dx
	item.At.Y += dy
	for i := range item.Path {
		for j := range item.Path[i].Points {
			item.Path[i].Points[j].X += dx
			item.Path[i].Points[j].Y += dy
		}
	}
}

type fakeGlyphSource struct {
	glyphs map[string]Glyph
	strict bool
	seen   map[string][]string
	mu     sync.Mutex
}

func (f *fakeGlyphSource) Glyph(req GlyphRequest) (Glyph, error) {
	f.mu.Lock()
	if f.seen == nil {
		f.seen = make(map[string][]string)
	}
	f.seen[req.Text] = append([]string(nil), req.Features...)
	glyph, ok := f.glyphs[req.Text]
	f.mu.Unlock()
	if ok {
		glyph.Text = req.Text
		glyph.ExactGlyphRef = true
		return glyph, nil
	}
	if f.strict {
		return Glyph{}, ErrMissingGlyph
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
		ID:            id,
		FontID:        "test-math",
		Text:          req.Text,
		Advance:       advance,
		Ascent:        em(7, 10),
		Descent:       em(2, 10),
		Italic:        em(1, 20),
		TopAccent:     em(1, 4),
		ExactGlyphRef: true,
		TextLike:      true,
		Outline: Path{
			{Op: geometry.SegMoveTo, Points: [3]Point{{X: 0, Y: -em(2, 10)}}},
			{Op: geometry.SegLineTo, Points: [3]Point{{X: advance, Y: -em(2, 10)}}},
			{Op: geometry.SegLineTo, Points: [3]Point{{X: advance, Y: em(7, 10)}}},
			{Op: geometry.SegLineTo, Points: [3]Point{{X: 0, Y: em(7, 10)}}},
			{Op: geometry.SegClose},
		},
	}, nil
}

type fakeStyleFont struct{}

func (fakeStyleFont) MathMetrics(style Style, variant ir.Variant) MathMetrics {
	return testMetrics()
}

type fakeStretcher struct {
	base    *fakeGlyphSource
	calls   int
	targets []geometry.Em
}

func (f *fakeStretcher) Stretch(req StretchRequest) (Glyph, error) {
	f.calls++
	f.targets = append(f.targets, req.Target)
	glyph, err := f.base.Glyph(req.GlyphRequest)
	if err != nil {
		return Glyph{}, err
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
	return glyph, nil
}

type fakeTextSource struct{}

func (fakeTextSource) ShapeText(req TextRequest) (TextRun, error) {
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
				ID:            1,
				FontID:        "test-text",
				Text:          req.Text,
				Advance:       advance,
				Ascent:        em(7, 10),
				Descent:       em(2, 10),
				ExactGlyphRef: true,
				TextLike:      true,
			},
		}},
	}, nil
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
	// Internal em rounding may remain one em subunit after host conversion.
	const tolerance = float32(1) / 1024
	diff := got - want
	return diff <= tolerance && diff >= -tolerance
}
