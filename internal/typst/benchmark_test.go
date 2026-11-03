package typst

import (
	"testing"

	"github.com/caelyreth/untie/ir"
	"github.com/caelyreth/untie/source"
)

var (
	benchmarkAST    *astNode
	benchmarkTokens []token
)

func BenchmarkParseCoreFormula(b *testing.B) {
	src := `sum_(i=0)^n mat(1, 2; 3, 4) + sqrt(frac(a+b, c)) + lr([hat(x)_i^2])`
	b.ReportAllocs()
	for b.Loop() {
		result := Parse(src, ir.Display, Options{})
		if result.Root == nil {
			b.Fatal("nil root")
		}
	}
}

func BenchmarkLexCoreFormula(b *testing.B) {
	src := `sum_(i=0)^n mat(1, 2; 3, 4) + sqrt(frac(a+b, c)) + lr([hat(x)_i^2])`
	limits := source.DefaultLimits()
	b.ReportAllocs()
	for b.Loop() {
		var diagnostics []source.Diagnostic
		benchmarkTokens, diagnostics = lex(src, limits)
		if len(diagnostics) != 0 {
			b.Fatal(diagnostics)
		}
	}
}

func BenchmarkParseASTCoreFormula(b *testing.B) {
	src := `sum_(i=0)^n mat(1, 2; 3, 4) + sqrt(frac(a+b, c)) + lr([hat(x)_i^2])`
	limits := source.DefaultLimits()
	b.ReportAllocs()
	for b.Loop() {
		var diagnostics []source.Diagnostic
		benchmarkAST, diagnostics = parseSource(src, ir.Display, limits)
		if len(diagnostics) != 0 {
			b.Fatal(diagnostics)
		}
	}
}
