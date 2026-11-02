package typst

import (
	"testing"

	"github.com/caelyreth/untie/ir"
	"github.com/caelyreth/untie/source"
)

func TestModuleNamesAreResolvedInTheirScope(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		src      string
		code     source.Code
		wantSpan source.Range
	}{
		{"bare math module", `math.frac(1, 2, style: "skewed")`, source.CodeUnknownVariable, source.NewRange(0, 4)},
		{"bare sym module", `sym.arrow.r`, source.CodeUnknownVariable, source.NewRange(0, 3)},
		{"unknown math field", `#math.nope`, source.CodeUnknownVariable, source.NewRange(6, 10)},
		{"unknown code root", `#foo.bar`, source.CodeUnknownVariable, source.NewRange(1, 4)},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			result := parse(t, test.src)
			if len(result.Diagnostics) != 1 {
				t.Fatalf("diagnostics = %#v, want one", result.Diagnostics)
			}
			diagnostic := result.Diagnostics[0]
			if diagnostic.Code != test.code || diagnostic.Range != test.wantSpan {
				t.Fatalf("diagnostic = %#v, want %s at %#v", diagnostic, test.code, test.wantSpan)
			}
		})
	}

	for _, src := range []string{
		`#math.frac(1, 2, style: "skewed")`,
		`#std.math.frac(1, 2)`,
		`#sym.arrow.r`,
		`#std.sym.alpha`,
	} {
		result := parse(t, src)
		requireNoDiagnostics(t, src, result)
	}
}

func TestBareCodeKeywordsRemainUnknownInMathMode(t *testing.T) {
	t.Parallel()

	for _, name := range []string{"none", "auto", "true", "false", "rgb"} {
		result := parse(t, name)
		if len(result.Diagnostics) != 1 || result.Diagnostics[0].Code != source.CodeUnknownVariable {
			t.Fatalf("%s diagnostics = %#v, want unknown-variable", name, result.Diagnostics)
		}
	}

	for _, src := range []string{"#none", "#auto", "#true", "#false", `#rgb(255, 0, 0)`} {
		result := parse(t, src)
		requireNoDiagnostics(t, src, result)
	}
}

func TestCallableSymbolsFollowTypstWrapperRules(t *testing.T) {
	t.Parallel()

	for _, src := range []string{
		`paren.l(x)`,
		`bracket.l(x)`,
		`chevron.l(x)`,
		`bar.v(x)`,
		`bar.v.double(x)`,
		`angle.l(x)`,
	} {
		result := parse(t, src)
		requireNoDiagnostics(t, src, result)
		fenced := find[*ir.Fenced](t, result.Root)
		if fenced.Open == nil || fenced.Close == nil {
			t.Fatalf("%s fences = %#v", src, fenced)
		}
	}

	result := parse(t, `paren.r(x)`)
	requireNoDiagnostics(t, "closing delimiter call", result)
	if got := nodeText(result.Root); got != ")(x)" {
		t.Fatalf("closing delimiter call = %q, want non-function display", got)
	}
}

func TestAttachAndCasesPreserveNeutralMetadata(t *testing.T) {
	t.Parallel()

	tests := []struct {
		src    string
		class  ir.MathClass
		limits ir.LimitsPlacement
	}{
		{`attach(limits(sum), tl: b, bl: a, tr: d, br: c)`, ir.ClassLarge, ir.LimitsAlways},
		{`attach(limits(=), t: "def")`, ir.ClassRelation, ir.LimitsAlways},
		{`op("argmax", limits: #true)_x`, ir.ClassLarge, ir.LimitsAlways},
	}
	for _, test := range tests {
		result := parse(t, test.src)
		requireNoDiagnostics(t, test.src, result)
		scripts := find[*ir.Scripts](t, result.Root)
		if scripts.Class != test.class || scripts.Limits != test.limits {
			t.Fatalf("%s scripts metadata = class %v limits %v", test.src, scripts.Class, scripts.Limits)
		}
	}

	for _, test := range []struct {
		src   string
		open  string
		close string
	}{
		{`cases(a, b)`, "{", ""},
		{`cases(reverse: #true, a, b)`, "", "}"},
		{`cases(delim: "[", a, b)`, "[", ""},
		{`cases(reverse: #true, delim: "[", a, b)`, "", "]"},
	} {
		result := parse(t, test.src)
		requireNoDiagnostics(t, test.src, result)
		fenced := find[*ir.Fenced](t, result.Root)
		if nodeText(fenced.Open) != test.open || nodeText(fenced.Close) != test.close || !fenced.Balanced {
			t.Fatalf("%s delimiters = %q/%q balanced=%v", test.src,
				nodeText(fenced.Open), nodeText(fenced.Close), fenced.Balanced)
		}
	}
}

func TestDifferentialAndBuiltinSpacingUseExactMathUnits(t *testing.T) {
	t.Parallel()

	result := parse(t, `integral f(x) dif x + Dif`)
	requireNoDiagnostics(t, "differential", result)
	var differentials []*ir.Row
	ir.Walk(result.Root, func(node ir.Node) bool {
		row, ok := node.(*ir.Row)
		if !ok || len(row.Items) != 2 {
			return true
		}
		space, spaceOK := row.Items[0].(*ir.Space)
		glyph, glyphOK := row.Items[1].(*ir.Glyph)
		if spaceOK && glyphOK && (glyph.Text == "d" || glyph.Text == "D") {
			if space.Weak && space.Amount == builtinMuLength(3) &&
				glyph.Class == ir.ClassUnary && glyph.Variant == ir.VariantNormal {
				differentials = append(differentials, row)
			}
		}
		return true
	})
	if len(differentials) != 2 {
		t.Fatalf("differentials = %d, want 2", len(differentials))
	}

	spacing := map[string]int64{
		"thin":  3,
		"med":   4,
		"thick": 5,
		"quad":  18,
		"wide":  36,
	}
	for name, want := range spacing {
		result := parse(t, name)
		requireNoDiagnostics(t, name, result)
		space := find[*ir.Space](t, result.Root)
		if space.Amount != builtinMuLength(float64(want)) {
			t.Fatalf("%s amount = %#v, want %d mu", name, space.Amount, want)
		}
	}
}
