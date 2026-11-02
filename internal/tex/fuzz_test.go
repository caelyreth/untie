package tex

import (
	"testing"

	"github.com/caelyreth/untie/ir"
	"github.com/caelyreth/untie/source"
)

func FuzzParseNoPanicAndValidIR(f *testing.F) {
	f.Add("x")
	f.Add(`\frac{1}{2}`)
	f.Add(`\def\a#1{#1^2}\a{x}`)
	f.Add(`\begin{matrix}a&b\\c&d\end{matrix}`)
	f.Add(string([]byte{0xff, '{', '}'}))

	f.Fuzz(func(t *testing.T, src string) {
		if len(src) > 4_096 {
			return
		}
		limits := source.DefaultLimits()
		limits.MaxSourceBytes = 4_096
		limits.MaxTokens = 8_192
		limits.MaxNodes = 8_192
		limits.MaxMacroExpansions = 1_024
		limits.MaxExpandedTokens = 8_192
		result := Parse(src, ir.Inline, Options{Limits: limits})
		if err := ir.Validate(&ir.Formula{Root: result.Root}, limits); err != nil {
			t.Fatalf("invalid IR for %q: %v", src, err)
		}
	})
}
