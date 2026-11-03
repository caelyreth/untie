package tex

import (
	"testing"

	"github.com/caelyreth/untie/source"
)

var benchmarkExpander *expander

func BenchmarkNewExpanderCoreFormula(b *testing.B) {
	src := `x^2+\frac{1}{2}+\sqrt{y}`
	limits := source.DefaultLimits()
	diagnostic := func(source.Diagnostic) {}
	b.ReportAllocs()
	for b.Loop() {
		benchmarkExpander = newExpander(src, limits, nil, diagnostic)
	}
}
