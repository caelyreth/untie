package crosscheck

import (
	"strings"
	"testing"

	"github.com/caelyreth/untie"
	"github.com/caelyreth/untie/display"
	"github.com/caelyreth/untie/internal/quality/fixture"
	"github.com/caelyreth/untie/ir"
	"github.com/caelyreth/untie/source"
)

func TestTypstLRSizingPolicyAndGeometry(t *testing.T) {
	root, _ := crosscheckPaths(t)
	harness := testHarness(t, root)

	tests := []struct {
		source string
		kind   ir.DelimiterSizingKind
		value  float64
	}{
		{source: `lr((frac(a+b,c)))`, kind: ir.DelimiterSizingContent},
		{source: `lr((frac(a+b,c)), size: #100%)`, kind: ir.DelimiterSizingContent},
		{source: `lr((frac(a+b,c)), size: #150%)`, kind: ir.DelimiterSizingRelative, value: 150},
		{source: `lr((frac(a+b,c)), size: #4em)`, kind: ir.DelimiterSizingAbsolute, value: 4},
	}
	var defaultHeights [2]float32
	for _, test := range tests {
		expr := untie.ParseTypst(test.source, ir.Inline, untie.ParseOptions{})
		formula := requireValidRoot(t, expr)
		fenced := findTypstFenced(t, formula.Root)
		if fenced.Sizing.Kind != test.kind {
			t.Fatalf("%s sizing kind = %v, want %v", test.source, fenced.Sizing.Kind, test.kind)
		}
		if test.value != 0 && fenced.Sizing.Value.Float() != test.value {
			t.Fatalf("%s sizing value = %v, want %v", test.source, fenced.Sizing.Value.Float(), test.value)
		}
		openAt := strings.Index(test.source, "((") + 1
		open := source.NewRange(openAt, openAt+1)
		var modeHeights []float32
		for _, mode := range []ir.Mode{ir.Inline, ir.Display} {
			box, err := harness.layout(t.Context(), formula.Root, mode, 16)
			if err != nil {
				t.Fatalf("%s mode %v: %v", test.source, mode, err)
			}
			left := requireHit(t, box, open, display.HitFence)
			rightmostFenceHit(t, box)
			modeHeights = append(modeHeights, left.Area.Height)
		}
		for modeIndex, height := range modeHeights {
			switch test.kind {
			case ir.DelimiterSizingContent:
				if defaultHeights[modeIndex] == 0 {
					defaultHeights[modeIndex] = height
				} else if !near(float64(defaultHeights[modeIndex]), float64(height)) {
					t.Fatalf("%s mode %d height = %v, want default %v",
						test.source, modeIndex, height, defaultHeights[modeIndex])
				}
			case ir.DelimiterSizingRelative:
				if height <= defaultHeights[modeIndex] {
					t.Fatalf("%s mode %d height = %v, want greater than default %v",
						test.source, modeIndex, height, defaultHeights[modeIndex])
				}
			case ir.DelimiterSizingAbsolute:
				if !near(float64(height), 64) {
					t.Fatalf("%s mode %d fence height = %v, want 4em = 64 DIP", test.source, modeIndex, height)
				}
			}
		}
	}
}

func TestTypstNamedSpacePreservesGlyphHitAndClassSpacing(t *testing.T) {
	root, _ := crosscheckPaths(t)
	harness := testHarness(t, root)

	expr := untie.ParseTypst(`space`, ir.Inline, untie.ParseOptions{})
	box, err := harness.layout(t.Context(), requireValidRoot(t, expr).Root, ir.Inline, 16)
	if err != nil {
		t.Fatal(err)
	}
	if len(box.Items) != 1 || (box.Items[0].Kind != display.ItemGlyphPath &&
		box.Items[0].Kind != display.ItemGlyphRef) {
		t.Fatalf("space items = %#v, want one source glyph", box.Items)
	}
	glyphItem := box.Items[0]
	if glyphItem.Glyph != 1 || !near(float64(glyphItem.Bound.Width), 5.3120117) {
		t.Fatalf("space glyph = id %d width %v, want font space glyph", glyphItem.Glyph, glyphItem.Bound.Width)
	}
	glyphHit, ok := findHit(box, source.NewRange(0, 5), display.HitAtom)
	if !ok || !near(float64(glyphHit.Area.Width), 5.3120117) {
		t.Fatalf("space hit = %#v, want placed source glyph", glyphHit)
	}

	spaced := untie.ParseTypst(`a, space b`, ir.Inline, untie.ParseOptions{})
	spacedBox, err := harness.layout(t.Context(), requireValidRoot(t, spaced).Root, ir.Inline, 16)
	if err != nil {
		t.Fatal(err)
	}
	comma, commaOK := findHit(spacedBox, source.NewRange(1, 2), display.HitAtom)
	space, spaceOK := findHit(spacedBox, source.NewRange(3, 8), display.HitAtom)
	if !commaOK || !spaceOK {
		t.Fatalf("punctuation hits = %#v", spacedBox.Hits)
	}
	if !near(float64(comma.Area.X+comma.Area.Width), float64(space.Area.X)) {
		t.Fatalf("punctuation inserted %.6f DIP before class-space glyph",
			float64(space.Area.X-(comma.Area.X+comma.Area.Width)))
	}

	corpus, err := LoadPairedCorpus(root)
	if err != nil {
		t.Fatal(err)
	}
	testCase := caseByID(t, corpus, "wide-accents")
	expectedFrame, err := fixture.FrameToDIP(*testCase.Fixture.Frame)
	if err != nil {
		t.Fatal(err)
	}
	typst := untie.ParseTypst(testCase.Source, caseMode(testCase), untie.ParseOptions{})
	typstBox, err := harness.layout(t.Context(), requireValidRoot(t, typst).Root, caseMode(testCase), expectedFrame.FontSize)
	if err != nil {
		t.Fatal(err)
	}
	var expectedSpaces []fixture.FrameGlyph
	for _, glyph := range expectedFrame.Glyphs {
		if glyph.Glyph == 1 && glyph.Text == " " {
			expectedSpaces = append(expectedSpaces, glyph)
		}
	}
	var actualSpaces []display.Item
	for _, item := range typstBox.Items {
		if item.Glyph == 1 && (item.Kind == display.ItemGlyphPath || item.Kind == display.ItemGlyphRef) {
			actualSpaces = append(actualSpaces, item)
		}
	}
	if len(expectedSpaces) != 3 || len(actualSpaces) != len(expectedSpaces) {
		t.Fatalf("wide-accents fixture spaces = expected %d, got %d", len(expectedSpaces), len(actualSpaces))
	}
	for i := range expectedSpaces {
		if !near(float64(actualSpaces[i].Bound.X), expectedSpaces[i].X) ||
			!near(float64(actualSpaces[i].Bound.Y), expectedSpaces[i].Y) ||
			!near(float64(actualSpaces[i].Em), expectedSpaces[i].Size) {
			t.Fatalf("wide-accents space %d = %#v, want fixture %#v", i, actualSpaces[i], expectedSpaces[i])
		}
	}
}

func TestTypstDisplayOnlyLimitsUseModeNotAtomClass(t *testing.T) {
	root, _ := crosscheckPaths(t)
	harness := testHarness(t, root)
	expr := untie.ParseTypst(`limits(A, inline: #false)_1^2`, ir.Inline, untie.ParseOptions{})
	formula := requireValidRoot(t, expr)

	inlineBox, err := harness.layout(t.Context(), formula.Root, ir.Inline, 16)
	if err != nil {
		t.Fatal(err)
	}
	displayBox, err := harness.layout(t.Context(), formula.Root, ir.Display, 16)
	if err != nil {
		t.Fatal(err)
	}
	inlineBase := requireHit(t, inlineBox, source.NewRange(7, 8), display.HitAtom)
	inlineTop := requireHit(t, inlineBox, source.NewRange(28, 29), display.HitScript)
	if inlineTop.Area.X < inlineBase.Area.X+inlineBase.Area.Width {
		t.Fatalf("inline display-only limits = base %#v top %#v, want side script", inlineBase, inlineTop)
	}

	displayBase := requireHit(t, displayBox, source.NewRange(7, 8), display.HitAtom)
	displayTop := requireHit(t, displayBox, source.NewRange(28, 29), display.HitScript)
	baseCenter := displayBase.Area.X + displayBase.Area.Width/2
	topCenter := displayTop.Area.X + displayTop.Area.Width/2
	if !near(float64(baseCenter), float64(topCenter)) || displayTop.Area.Y >= displayBase.Area.Y {
		t.Fatalf("display display-only limits = base %#v top %#v, want above center", displayBase, displayTop)
	}
}

func TestTypstNestedCornerAttachmentsKeepEffectiveLimits(t *testing.T) {
	root, _ := crosscheckPaths(t)
	harness := testHarness(t, root)

	limitsExpr := untie.ParseTypst(`attach(attach(limits(A), tl:b), t:x)`, ir.Inline, untie.ParseOptions{})
	limitsRoot := requireValidRoot(t, limitsExpr)
	for _, mode := range []ir.Mode{ir.Inline, ir.Display} {
		box, err := harness.layout(t.Context(), limitsRoot.Root, mode, 16)
		if err != nil {
			t.Fatal(err)
		}
		base := requireHit(t, box, source.NewRange(21, 22), display.HitAtom)
		top := requireHit(t, box, source.NewRange(34, 35), display.HitScript)
		if top.Area.Y >= base.Area.Y || top.Area.X >= base.Area.X+base.Area.Width {
			t.Fatalf("mode %v nested limits = base %#v top %#v, want above placement", mode, base, top)
		}
	}

	scriptsExpr := untie.ParseTypst(`attach(attach(scripts(sum), tl:b), t:x)`, ir.Inline, untie.ParseOptions{})
	scriptsRoot := requireValidRoot(t, scriptsExpr)
	for _, mode := range []ir.Mode{ir.Inline, ir.Display} {
		box, err := harness.layout(t.Context(), scriptsRoot.Root, mode, 16)
		if err != nil {
			t.Fatal(err)
		}
		base := requireHit(t, box, source.NewRange(22, 25), display.HitAtom)
		top := requireHit(t, box, source.NewRange(37, 38), display.HitScript)
		if top.Area.X < base.Area.X+base.Area.Width {
			t.Fatalf("mode %v nested scripts = base %#v top %#v, want side script", mode, base, top)
		}
	}
}

func TestTypstAccentNaturalFitGeometry(t *testing.T) {
	root, _ := crosscheckPaths(t)
	harness := testHarness(t, root)

	tests := []struct {
		src         string
		baseWidth   float64
		accentWidth float64
	}{
		{`hat(x)`, 9.1521, 4.895996},
		{`hat(sum)`, 16.895996, 10.336182},
		{`hat(integral)`, 10.639893, 4.895996},
		{`hat(op("sin"))`, 19.76001, 12.320068},
	}
	for _, test := range tests {
		expr := untie.ParseTypst(test.src, ir.Inline, untie.ParseOptions{})
		box, err := harness.layout(t.Context(), requireValidRoot(t, expr).Root, ir.Inline, 16)
		if err != nil {
			t.Fatal(err)
		}
		if !near(float64(box.Width), test.baseWidth) {
			t.Fatalf("%s width = %.6f, want %.6f", test.src, box.Width, test.baseWidth)
		}
		accent := box.Items[len(box.Items)-1]
		if !near(float64(accent.Bound.Width), test.accentWidth) {
			t.Fatalf("%s accent width = %.6f, want %.6f", test.src, accent.Bound.Width, test.accentWidth)
		}
	}
}

func TestTypstFractionDeparenthesizationPreservesPartHits(t *testing.T) {
	root, _ := crosscheckPaths(t)
	harness := testHarness(t, root)
	src := `(a+b)/(c+d)`
	expr := untie.ParseTypst(src, ir.Inline, untie.ParseOptions{})
	box, err := harness.layout(t.Context(), requireValidRoot(t, expr).Root, ir.Inline, 16)
	if err != nil {
		t.Fatal(err)
	}
	numerator := requireHit(t, box, source.NewRange(1, 4), display.HitFractionNumerator)
	denominator := requireHit(t, box, source.NewRange(7, 10), display.HitFractionDenominator)
	if numerator.Area.Width <= 0 || denominator.Area.Width <= 0 {
		t.Fatalf("fraction part hits = numerator %#v denominator %#v", numerator, denominator)
	}
}

func findHit(box *display.Box, span source.Range, kind display.HitKind) (display.Hit, bool) {
	for _, hit := range box.Hits {
		if hit.Source == span && hit.Kind == kind {
			return hit, true
		}
	}
	return display.Hit{}, false
}

func requireHit(t *testing.T, box *display.Box, span source.Range, kind display.HitKind) display.Hit {
	t.Helper()
	hit, ok := findHit(box, span, kind)
	if !ok {
		t.Fatalf("hits = %#v, want kind %v at %v", box.Hits, kind, span)
	}
	return hit
}

func rightmostFenceHit(t *testing.T, box *display.Box) display.Hit {
	t.Helper()
	var found display.Hit
	have := false
	for _, hit := range box.Hits {
		if hit.Kind == display.HitFence && (!have || hit.Area.X > found.Area.X) {
			found, have = hit, true
		}
	}
	if !have {
		t.Fatalf("hits = %#v, want a closing fence hit", box.Hits)
	}
	return found
}

func findTypstFenced(t *testing.T, root ir.Node) *ir.Fenced {
	t.Helper()
	var found *ir.Fenced
	ir.Walk(root, func(node ir.Node) bool {
		if fenced, ok := node.(*ir.Fenced); ok && found == nil {
			found = fenced
		}
		return found == nil
	})
	if found == nil {
		t.Fatal("no Fenced in Typst IR")
	}
	return found
}

func caseByID(t *testing.T, corpus *fixture.Corpus, id string) fixture.Case {
	t.Helper()
	for _, testCase := range corpus.Cases {
		if testCase.ID == id {
			return testCase
		}
	}
	t.Fatalf("case %q not found", id)
	return fixture.Case{}
}
