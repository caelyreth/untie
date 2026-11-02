package typst

import (
	"testing"

	"github.com/caelyreth/untie/ir"
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
