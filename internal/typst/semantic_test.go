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
