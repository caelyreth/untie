package layout

import (
	"errors"
	"testing"

	"github.com/caelyreth/untie/display"
	"github.com/caelyreth/untie/geometry"
	"github.com/caelyreth/untie/ir"
	"github.com/caelyreth/untie/source"
)

func absoluteDelimiterSizing(millionths int64) ir.DelimiterSizing {
	return ir.AbsoluteDelimiterSizing(ir.RawLength(ir.LengthEm, millionths))
}

type delimiterSizeCase struct {
	name   string
	sizing ir.DelimiterSizing
	target geometry.Em
}

func delimiterSizeCases(t *testing.T) []delimiterSizeCase {
	t.Helper()
	relative, err := ir.PercentLength(150)
	if err != nil {
		t.Fatal(err)
	}
	return []delimiterSizeCase{
		{name: "tier1", sizing: absoluteDelimiterSizing(1_200_000), target: em(12, 10)},
		{name: "tier2", sizing: absoluteDelimiterSizing(1_800_000), target: em(18, 10)},
		{name: "tier3", sizing: absoluteDelimiterSizing(2_400_000), target: em(24, 10)},
		{name: "tier4", sizing: absoluteDelimiterSizing(3_000_000), target: em(3, 1)},
		{name: "relative", sizing: ir.RelativeDelimiterSizing(relative), target: em(3, 2)},
	}
}

func TestSizedDelimiterTargetPolicies(t *testing.T) {
	t.Parallel()

	for _, tt := range delimiterSizeCases(t) {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			renderer, _, stretcher := newTestRenderer(t)
			node := ir.NewSizedDelimiter(source.NewRange(0, 1), "(", tt.sizing)
			node.Class = ir.ClassOpening
			if _, err := renderer.Layout(node, ir.Inline, testStyle(16)); err != nil {
				t.Fatalf("Layout() error = %v", err)
			}
			if len(stretcher.targets) != 1 || stretcher.targets[0] != tt.target {
				t.Fatalf("stretch targets = %v, want [%v]", stretcher.targets, tt.target)
			}
		})
	}
}

func TestSizedDelimiterStretchesIndependentlyOfMathStyle(t *testing.T) {
	t.Parallel()

	for _, tt := range delimiterSizeCases(t)[:4] {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			renderer, _, stretcher := newTestRenderer(t)
			span := source.NewRange(0, 7)
			node := ir.NewSizedDelimiter(span, "(", tt.sizing)
			node.Class = ir.ClassOpening
			// TeX explicit tiers request text style explicitly in neutral IR.
			node.Size = ir.SizeText

			inline, err := renderer.Layout(node, ir.Display, testStyle(16))
			if err != nil {
				t.Fatalf("inline layout error = %v", err)
			}
			style := ir.NewStyle(span, node)
			style.SetSize = true
			style.NewSize = ir.SizeScript
			script, err := renderer.Layout(style, ir.Display, testStyle(16))
			if err != nil {
				t.Fatalf("script layout error = %v", err)
			}
			wantExtent := float32(tt.target.Float() * 16)
			for name, box := range map[string]*display.Box{"inline": inline, "script": script} {
				if !closeHost(box.Ascent+box.Descent, wantExtent) {
					t.Fatalf("%s extent = %v, want %v", name, box.Ascent+box.Descent, wantExtent)
				}
				if box.Width <= 0 {
					t.Fatalf("%s width = %v, want positive", name, box.Width)
				}
			}
			if len(stretcher.targets) != 2 {
				t.Fatalf("stretch targets = %v, want inline and script requests", stretcher.targets)
			}
			if stretcher.targets[0] != tt.target {
				t.Fatalf("inline target = %v, want %v", stretcher.targets[0], tt.target)
			}
			if stretcher.targets[1] != tt.target {
				t.Fatalf("script target = %v, want style-independent %v", stretcher.targets[1], tt.target)
			}
		})
	}
}

func TestSizedDelimiterWithoutExplicitSizeInheritsCurrentMathStyle(t *testing.T) {
	t.Parallel()

	renderer, _, _ := newTestRenderer(t)
	span := source.NewRange(0, 5)
	node := ir.NewSizedDelimiter(span, "(", absoluteDelimiterSizing(2_400_000))
	node.Class = ir.ClassOpening
	style := ir.NewStyle(span, node)
	style.SetSize = true
	style.NewSize = ir.SizeScript

	box, err := renderer.Layout(style, ir.Display, testStyle(16))
	if err != nil {
		t.Fatalf("Layout() error = %v", err)
	}
	// The test font's script scale is 80%, so a 2.4em current-style target
	// produces a 1.92em output extent.
	if !closeHost(box.Ascent+box.Descent, float32(1.92*16)) {
		t.Fatalf("extent = %v, want inherited script-scaled 1.92em", box.Ascent+box.Descent)
	}
}

func TestFencedDelimiterSizingPolicies(t *testing.T) {
	t.Parallel()

	relative := func(value float64) ir.DelimiterSizing {
		length, err := ir.PercentLength(value)
		if err != nil {
			t.Fatal(err)
		}
		return ir.RelativeDelimiterSizing(length)
	}
	tests := []struct {
		name   string
		sizing ir.DelimiterSizing
		target geometry.Em
		style  ir.MathSize
	}{
		{name: "content", target: em(9, 10)},
		{name: "relative", sizing: relative(150), target: em(135, 100)},
		{name: "absolute text", sizing: absoluteDelimiterSizing(2_400_000), target: em(24, 10)},
		{name: "absolute script", sizing: absoluteDelimiterSizing(2_400_000), target: em(24, 10), style: ir.SizeScript},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			renderer, _, stretcher := newTestRenderer(t)
			span := source.NewRange(0, 5)
			fenced := ir.NewFenced(span, glyph("x", ir.ClassAlphabetic))
			fenced.Open = glyph("(", ir.ClassOpening)
			fenced.Close = glyph(")", ir.ClassClosing)
			fenced.Balanced = true
			fenced.Sizing = tt.sizing
			if tt.style != 0 {
				fenced.Size = tt.style
			}
			if _, err := renderer.Layout(fenced, ir.Display, testStyle(16)); err != nil {
				t.Fatalf("Layout() error = %v", err)
			}
			if len(stretcher.targets) != 2 {
				t.Fatalf("stretch targets = %v, want opening and closing", stretcher.targets)
			}
			for i, target := range stretcher.targets {
				if target != tt.target && target != tt.target-1 {
					t.Fatalf("target[%d] = %v, want %v", i, target, tt.target)
				}
			}
		})
	}
}

func TestSizedDelimiterPreservesClassSpacingAndSourceHit(t *testing.T) {
	t.Parallel()

	renderer, _, _ := newTestRenderer(t)
	x := glyph("x", ir.ClassNormal)
	x.Span = source.NewRange(0, 1)
	x.Origin = x.Span
	relation := ir.NewSizedDelimiter(source.NewRange(1, 7), ".", absoluteDelimiterSizing(1_200_000))
	relation.Origin = source.NewRange(2, 7)
	relation.Class = ir.ClassRelation
	y := glyph("y", ir.ClassNormal)
	y.Span = source.NewRange(8, 9)
	y.Origin = y.Span
	root := ir.NewRow(source.NewRange(0, 9), x, relation, y)

	box, err := renderer.Layout(root, ir.Inline, testStyle(16))
	if err != nil {
		t.Fatalf("Layout() error = %v", err)
	}
	if len(box.Items) != 2 {
		t.Fatalf("items = %#v, want only x and y glyphs", box.Items)
	}
	thick, err := geometry.EmFromParts(5, 18)
	if err != nil {
		t.Fatal(err)
	}
	wantY := float32(thick.Float()*16*2) + 8
	if !closeHost(box.Items[1].At.X, wantY) {
		t.Fatalf("relation spacing y.x = %v, want %v", box.Items[1].At.X, wantY)
	}

	var hit *display.Hit
	for i := range box.Hits {
		if box.Hits[i].Kind == display.HitFence && box.Hits[i].Source == relation.Origin {
			hit = &box.Hits[i]
			break
		}
	}
	if hit == nil {
		t.Fatalf("hits = %#v, want sized-delimiter fence hit for %v", box.Hits, relation.Origin)
	}
	if hit.Area.Width != 0 {
		t.Fatalf("null delimiter hit width = %v, want 0", hit.Area.Width)
	}
}

func TestSizedDelimiterRejectsMalformedInputAndMissingStretch(t *testing.T) {
	t.Parallel()

	renderer, _, _ := newTestRenderer(t)
	relative := func(value float64) ir.DelimiterSizing {
		length, err := ir.PercentLength(value)
		if err != nil {
			t.Fatal(err)
		}
		return ir.RelativeDelimiterSizing(length)
	}
	for name, node := range map[string]*ir.SizedDelimiter{
		"empty": {
			Base:   ir.NewBase(ir.KindSizedDelimiter, source.NewRange(0, 1)),
			Sizing: absoluteDelimiterSizing(1_200_000),
		},
		"content sizing":      ir.NewSizedDelimiter(source.NewRange(0, 1), "(", ir.ContentDelimiterSizing()),
		"invalid sizing":      ir.NewSizedDelimiter(source.NewRange(0, 1), "(", ir.DelimiterSizing{Kind: ir.DelimiterSizingAbsolute + 1}),
		"wrong relative unit": ir.NewSizedDelimiter(source.NewRange(0, 1), "(", ir.RelativeDelimiterSizing(ir.RawLength(ir.LengthEm, 1_000_000))),
		"wrong absolute unit": ir.NewSizedDelimiter(source.NewRange(0, 1), "(", ir.AbsoluteDelimiterSizing(mustPercent(t, 150))),
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			if _, err := renderer.Layout(node, ir.Inline, testStyle(16)); !errors.Is(err, ErrInvalidInput) {
				t.Fatalf("Layout() error = %v, want invalid input", err)
			}
		})
	}

	glyphs := &fakeGlyphSource{glyphs: map[string]Glyph{
		"(": {
			ID:      1,
			FontID:  "test-math",
			Advance: em(1, 4),
			Ascent:  em(1, 2),
			Descent: em(1, 10),
		},
	}, strict: true}
	noStretch, err := New(Fonts{Glyphs: glyphs, Metrics: fakeStyleFont{}}, Options{})
	if err != nil {
		t.Fatal(err)
	}
	node := ir.NewSizedDelimiter(source.NewRange(0, 4), "(", relative(150))
	node.Class = ir.ClassOpening
	if _, err := layoutStrict(noStretch, node, testStyle(16)); !errors.Is(err, ErrInsufficientStretch) {
		t.Fatalf("Layout() error = %v, want insufficient stretch", err)
	}
}

func mustPercent(t *testing.T, value float64) ir.Length {
	t.Helper()
	length, err := ir.PercentLength(value)
	if err != nil {
		t.Fatal(err)
	}
	return length
}
