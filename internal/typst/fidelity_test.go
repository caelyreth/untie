package typst

import (
	"testing"

	"github.com/caelyreth/untie/ir"
	"github.com/caelyreth/untie/source"
)

func TestFractionDeparenthesizesOneLeftOperandPair(t *testing.T) {
	t.Parallel()

	src := `(a+b)/c`
	result := parse(t, src)
	requireNoDiagnostics(t, src, result)
	fraction := find[*ir.Fraction](t, result.Root)
	if fraction.Span != source.NewRange(0, len(src)) || fraction.Origin != fraction.Span {
		t.Fatalf("fraction span/origin = %#v/%#v, want full expression", fraction.Span, fraction.Origin)
	}
	numerator, ok := fraction.Numerator.(*ir.Row)
	if !ok || numerator.Span != source.NewRange(1, 4) {
		t.Fatalf("numerator = %#v, want row spanning a+b", fraction.Numerator)
	}
	if glyphs := glyphTextCount(result.Root, "("); glyphs != 0 {
		t.Fatalf("left operand retained %d opening delimiters", glyphs)
	}

	nestedSrc := `((a+b))/c`
	nested := parse(t, nestedSrc)
	requireNoDiagnostics(t, nestedSrc, nested)
	nestedFraction := find[*ir.Fraction](t, nested.Root)
	if glyphs := glyphTextCount(nested.Root, "("); glyphs != 1 {
		t.Fatalf("nested left operand retained %d opening delimiters, want one pair", glyphs)
	}
	if glyphs := glyphTextCount(nested.Root, ")"); glyphs != 1 {
		t.Fatalf("nested left operand retained %d closing delimiters, want one pair", glyphs)
	}
	inner, ok := nestedFraction.Numerator.(*ir.Row)
	if !ok || inner.Span != source.NewRange(1, 6) {
		t.Fatalf("nested numerator = %#v, want outer body span", nestedFraction.Numerator)
	}

	denominatorSrc := `a/(b+c)`
	denominatorResult := parse(t, denominatorSrc)
	requireNoDiagnostics(t, denominatorSrc, denominatorResult)
	denominatorFraction := find[*ir.Fraction](t, denominatorResult.Root)
	denominator, ok := denominatorFraction.Denominator.(*ir.Row)
	if !ok || denominator.Span != source.NewRange(3, 6) {
		t.Fatalf("denominator = %#v, want row spanning b+c", denominatorFraction.Denominator)
	}
	if denominatorFraction.Span != source.NewRange(0, len(denominatorSrc)) {
		t.Fatalf("denominator fraction span = %#v, want full expression", denominatorFraction.Span)
	}
	for _, delimiter := range []string{"(", ")"} {
		if glyphs := glyphTextCount(denominatorResult.Root, delimiter); glyphs != 0 {
			t.Fatalf("denominator retained %d %q delimiters", glyphs, delimiter)
		}
	}
}

func TestLiteralDelimiterClosingSpanMatchesVisibleToken(t *testing.T) {
	t.Parallel()

	const src = `f'(x) + (A)`
	result := parse(t, src)
	requireNoDiagnostics(t, src, result)
	var closing []*ir.Glyph
	ir.Walk(result.Root, func(node ir.Node) bool {
		if glyph, ok := node.(*ir.Glyph); ok && glyph.Text == ")" {
			closing = append(closing, glyph)
		}
		return true
	})
	if len(closing) != 2 {
		t.Fatalf("closing delimiters = %d, want 2", len(closing))
	}
	if closing[0].Span != source.NewRange(4, 5) ||
		closing[1].Span != source.NewRange(10, 11) {
		t.Fatalf("closing spans = %v, %v", closing[0].Span, closing[1].Span)
	}
}

func TestUnclosedDelimiterDoesNotSynthesizeClosingGlyph(t *testing.T) {
	t.Parallel()

	const src = "(α"
	result := Parse(src, ir.Inline, Options{})
	if result.Root == nil {
		t.Fatal("Parse() returned a nil root")
	}
	if err := ir.Validate(&ir.Formula{Root: result.Root}, source.DefaultLimits()); err != nil {
		t.Fatalf("unclosed delimiter produced invalid IR: %v", err)
	}
	if glyphs := glyphTextCount(result.Root, ")"); glyphs != 0 {
		t.Fatalf("unclosed delimiter synthesized %d closing glyphs", glyphs)
	}
}

func TestNamedSpacesLowerBeforeSymbolLookup(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		kind   ir.SpaceKind
		amount ir.Length
	}{
		{"thin", ir.SpaceExplicit, builtinEmLength(1.0 / 6.0)},
		{"med", ir.SpaceExplicit, builtinEmLength(2.0 / 9.0)},
		{"thick", ir.SpaceExplicit, builtinEmLength(5.0 / 18.0)},
		{"quad", ir.SpaceExplicit, builtinEmLength(1)},
		{"wide", ir.SpaceExplicit, builtinEmLength(2)},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			result := parse(t, test.name)
			requireNoDiagnostics(t, test.name, result)
			space := find[*ir.Space](t, result.Root)
			if space.Amount != test.amount || space.Type != test.kind ||
				space.Class != ir.ClassSpace {
				t.Fatalf("%s = %#v, want class-space %v/%v", test.name, space, test.kind, test.amount)
			}
		})
	}

	result := parse(t, "space")
	requireNoDiagnostics(t, "space", result)
	if spaces := countNodes[*ir.Space](result.Root); spaces != 0 {
		t.Fatalf("space lowered to %d neutral spaces, want a source glyph", spaces)
	}
	glyph := find[*ir.Glyph](t, result.Root)
	if glyph.Text != " " || glyph.Class != ir.ClassSpace || glyph.Span != source.NewRange(0, 5) {
		t.Fatalf("space glyph = %#v, want class-space source glyph", glyph)
	}
}

func TestUnionAndIntersectionRespectGeneratedVariants(t *testing.T) {
	t.Parallel()

	tests := []struct {
		src   string
		text  string
		class ir.MathClass
	}{
		{"union", "∪", ir.ClassBinary},
		{"union.big", "⋃", ir.ClassLarge},
		{"inter", "∩", ir.ClassBinary},
		{"inter.big", "⋂", ir.ClassLarge},
		{"and.big", "⋀", ir.ClassLarge},
		{"or.big", "⋁", ir.ClassLarge},
	}
	for _, test := range tests {
		test := test
		t.Run(test.src, func(t *testing.T) {
			t.Parallel()
			result := parse(t, test.src)
			requireNoDiagnostics(t, test.src, result)
			glyph := find[*ir.Glyph](t, result.Root)
			if glyph.Text != test.text || glyph.Class != test.class {
				t.Fatalf("%s = %q/%v, want %q/%v", test.src, glyph.Text, glyph.Class, test.text, test.class)
			}
		})
	}
}

func TestPrimeRunsPrecomposeThroughFour(t *testing.T) {
	t.Parallel()

	precomposed := []string{"′", "″", "‴", "⁗"}
	for count, want := range precomposed {
		src := "f" + repeated("'", count+1)
		result := parse(t, src)
		requireNoDiagnostics(t, src, result)
		scripts := find[*ir.Scripts](t, result.Root)
		prime, ok := scripts.TopRight.(*ir.Glyph)
		if !ok || prime.Text != want {
			t.Fatalf("%s top-right = %#v, want %q", src, scripts.TopRight, want)
		}
		if prime.Span != source.NewRange(1, len(src)) {
			t.Fatalf("%s prime span = %#v, want complete run", src, prime.Span)
		}
	}

	src := `f'''''`
	result := parse(t, src)
	requireNoDiagnostics(t, src, result)
	scripts := find[*ir.Scripts](t, result.Root)
	primes, ok := scripts.TopRight.(*ir.Row)
	if !ok || len(primes.Items) != 5 {
		t.Fatalf("five primes = %#v, want five-glyph row", scripts.TopRight)
	}
	for i, item := range primes.Items {
		glyph, ok := item.(*ir.Glyph)
		if !ok || glyph.Text != "′" {
			t.Fatalf("five primes item %d = %#v, want single prime", i, item)
		}
	}

	explicit := parse(t, `f primes(2)`)
	requireNoDiagnostics(t, "explicit primes", explicit)
	if got := nodeText(explicit.Root); got != "f″" {
		t.Fatalf("explicit two primes = %q, want f with double prime", got)
	}

	zero := parse(t, `f primes(0)`)
	requireNoDiagnostics(t, "zero primes", zero)
	if got := nodeText(zero.Root); got != "f" {
		t.Fatalf("zero primes = %q, want no prime glyph", got)
	}

	negative := parse(t, `f primes(-1)`)
	var invalidCount bool
	for _, diagnostic := range negative.Diagnostics {
		if diagnostic.Code == source.CodeInvalidValue && diagnostic.Severity == source.SeverityError {
			invalidCount = true
		}
	}
	if !invalidCount {
		t.Fatalf("negative prime diagnostics = %#v, want invalid-value", negative.Diagnostics)
	}
	if _, ok := findPlaceholder(negative.Root, "invalid prime count"); !ok {
		t.Fatalf("negative prime root = %#v, want visible fallback", negative.Root)
	}
}

func TestAccentStretchAndStructuralBraceMetadata(t *testing.T) {
	t.Parallel()

	for _, src := range []string{`hat(x)`, `dot.triple(x)`, `arrow(f)`} {
		result := parse(t, src)
		requireNoDiagnostics(t, src, result)
		accent := find[*ir.Accent](t, result.Root)
		if accent.Stretchy || !accent.NaturalFit {
			t.Fatalf("%s accent policy = stretchy %v natural-fit %v", src, accent.Stretchy, accent.NaturalFit)
		}
	}

	for _, src := range []string{`hat(a b c)`, `hat(sum)`, `hat(integral)`, `hat(op("sin"))`, `arrow(A B)`, `accent(C D, arrow.l.r)`} {
		result := parse(t, src)
		requireNoDiagnostics(t, src, result)
		accent := find[*ir.Accent](t, result.Root)
		if accent.Stretchy || !accent.NaturalFit {
			t.Fatalf("%s accent policy = stretchy %v natural-fit %v", src, accent.Stretchy, accent.NaturalFit)
		}
	}

	brace := parse(t, `overbrace(a+b)`)
	requireNoDiagnostics(t, "plain structural brace", brace)
	decoration := find[*ir.Accent](t, brace.Root)
	if !decoration.Stretchy || !decoration.NaturalFit ||
		decoration.Class != ir.ClassInner || decoration.Limits != ir.LimitsAlways {
		t.Fatalf("structural brace = %#v, want stretchy inner limits", decoration)
	}

	annotated := parse(t, `overbrace(a+b, n)`)
	requireNoDiagnostics(t, "annotated structural brace", annotated)
	scripts := find[*ir.Scripts](t, annotated.Root)
	if scripts.Class != ir.ClassInner || scripts.Limits != ir.LimitsAlways ||
		scripts.Role != ir.ScriptRoleBraceLabel {
		t.Fatalf("brace annotation scripts = %#v, want inner/always/brace-label", scripts)
	}
}

func TestLimitWrappersRecordExplicitPlacementWithoutForcingAuto(t *testing.T) {
	t.Parallel()

	overset := parse(t, `attach(limits(=), t: "def")`)
	requireNoDiagnostics(t, "overset", overset)
	outer := firstScripts(t, overset.Root)
	inner, ok := outer.BaseItem.(*ir.Scripts)
	if !ok {
		t.Fatalf("overset base = %#v, want limits wrapper", outer.BaseItem)
	}
	equals, ok := inner.BaseItem.(*ir.Glyph)
	if !ok || equals.Limits != ir.LimitsAlways {
		t.Fatalf("overset relation limits = %#v, want always", inner.BaseItem)
	}
	if inner.Limits != ir.LimitsAlways || outer.Limits != ir.LimitsAlways {
		t.Fatalf("overset wrapper limits = inner %v outer %v", inner.Limits, outer.Limits)
	}
	if inner.Role != ir.ScriptRoleOrdinary || outer.Role != ir.ScriptRoleOverlay {
		t.Fatalf("overset roles = inner %v outer %v", inner.Role, outer.Role)
	}

	xarrow := parse(t, `A stretch(->)^"over"_"under" B`)
	requireNoDiagnostics(t, "xarrow", xarrow)
	arrowScripts := firstScripts(t, xarrow.Root)
	arrow, ok := arrowScripts.BaseItem.(*ir.Glyph)
	if !ok || arrow.Limits != ir.LimitsAlways || arrowScripts.Limits != ir.LimitsAlways {
		t.Fatalf("xarrow limits = base %#v wrapper %v", arrowScripts.BaseItem, arrowScripts.Limits)
	}

	sideset := parse(t, `attach(limits(sum), tl: b, bl: a, tr: d, br: c)`)
	requireNoDiagnostics(t, "sideset", sideset)
	sidesetOuter := firstScripts(t, sideset.Root)
	sidesetInner, ok := sidesetOuter.BaseItem.(*ir.Scripts)
	if !ok || sidesetOuter.Limits != ir.LimitsAuto || sidesetInner.Limits != ir.LimitsAuto {
		t.Fatalf("sideset wrappers = outer %#v inner %#v", sidesetOuter, sidesetInner)
	}
	if sidesetOuter.Role != ir.ScriptRoleLimits || sidesetInner.Role != ir.ScriptRoleLimits {
		t.Fatalf("sideset roles = outer %v inner %v", sidesetOuter.Role, sidesetInner.Role)
	}
	sum, ok := sidesetInner.BaseItem.(*ir.Glyph)
	if !ok || sum.Limits != ir.LimitsAlways {
		t.Fatalf("sideset sum limits = %#v, want always", sidesetInner.BaseItem)
	}

	inline := parse(t, `limits(A, inline: #false)_1^2`)
	requireNoDiagnostics(t, "inline limits false", inline)
	inlineScripts := firstScripts(t, inline.Root)
	base, ok := inlineScripts.BaseItem.(*ir.Glyph)
	if !ok || inlineScripts.Limits != ir.LimitsDisplay || base.Limits != ir.LimitsDisplay {
		t.Fatalf("display-only limits = wrapper %v base %v, want display", inlineScripts.Limits, base.Limits)
	}
}

func TestNestedCornerAttachmentsKeepEffectiveLimits(t *testing.T) {
	t.Parallel()

	tests := []struct {
		src   string
		limit ir.LimitsPlacement
	}{
		{`attach(attach(limits(A), tl:b), t:x)`, ir.LimitsAlways},
		{`attach(attach(scripts(sum), tl:b), t:x)`, ir.LimitsNever},
	}
	for _, test := range tests {
		result := parse(t, test.src)
		requireNoDiagnostics(t, test.src, result)
		outer := firstScripts(t, result.Root)
		if outer.Limits != test.limit {
			t.Fatalf("%s outer limits = %v, want %v", test.src, outer.Limits, test.limit)
		}
		if got := effectiveNodeLimits(outer.BaseItem); got != test.limit {
			t.Fatalf("%s inner effective limits = %v, want %v", test.src, got, test.limit)
		}
	}
}

func firstScripts(t *testing.T, root ir.Node) *ir.Scripts {
	t.Helper()
	var found *ir.Scripts
	ir.Walk(root, func(node ir.Node) bool {
		if scripts, ok := node.(*ir.Scripts); ok && found == nil {
			found = scripts
			return false
		}
		return true
	})
	if found == nil {
		t.Fatal("no scripts node")
	}
	return found
}

func glyphTextCount(root ir.Node, text string) int {
	count := 0
	ir.Walk(root, func(node ir.Node) bool {
		if glyph, ok := node.(*ir.Glyph); ok && glyph.Text == text {
			count++
		}
		return true
	})
	return count
}

func findPlaceholder(root ir.Node, reason string) (*ir.Placeholder, bool) {
	var found *ir.Placeholder
	ir.Walk(root, func(node ir.Node) bool {
		placeholder, ok := node.(*ir.Placeholder)
		if ok && placeholder.Reason == reason {
			found = placeholder
			return false
		}
		return true
	})
	return found, found != nil
}

func repeated(text string, count int) string {
	out := ""
	for range count {
		out += text
	}
	return out
}
