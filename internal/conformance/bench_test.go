package conformance

import (
	"testing"

	"github.com/caelyreth/untie/internal/tex"
	"github.com/caelyreth/untie/ir"
	"github.com/caelyreth/untie/source"
)

func BenchmarkParseTeXRepresentative(b *testing.B) {
	cases := []struct {
		name string
		src  string
		mode ir.Mode
	}{
		{"symbols", `x+\alpha-\beta=\gamma`, ir.Inline},
		{"scripts", `x_i^2+\sum_{i=0}^{n}a_i`, ir.Display},
		{"fraction", `\frac{1+x^2}{1-x^2}`, ir.Display},
		{"radical", `\sqrt[3]{\frac a b}`, ir.Display},
		{"matrix", `\begin{matrix}a&b\\c&d\end{matrix}`, ir.Display},
		{"text", `\text{velocity} + v_0 t`, ir.Inline},
		{"macro", `\def\sqr#1{#1^2}\sqr{x}+\sqr{y}`, ir.Display},
		{"mixed", `\left(\frac{\sum_i x_i}{\sqrt{n}}\right)^2`, ir.Display},
	}
	limits := ExplicitTeXLimits()
	for _, testCase := range cases {
		testCase := testCase
		b.Run(testCase.name, func(b *testing.B) {
			warmup := tex.Parse(testCase.src, testCase.mode, tex.Options{Limits: limits})
			if err := ir.Validate(&ir.Formula{Root: warmup.Root}, limits); err != nil {
				b.Fatalf("invalid benchmark input: %v", err)
			}
			if len(warmup.Diagnostics) != 0 {
				b.Fatalf("benchmark input diagnostics: %v", warmup.Diagnostics)
			}
			b.ReportAllocs()
			b.SetBytes(int64(len(testCase.src)))
			b.ResetTimer()
			for b.Loop() {
				result := tex.Parse(testCase.src, testCase.mode, tex.Options{Limits: limits})
				if result.Root == nil {
					b.Fatal("nil root")
				}
			}
		})
	}
}

func BenchmarkParseTeXCorpusMix(b *testing.B) {
	sources := []string{
		`n!`,
		`a\!b`,
		`\def\sqr#1{#1^2}\sqr{y}`,
		`\begin{matrix} a & b \\ c & d \end{matrix}`,
		`\frac{\partial^2 u}{\partial x \partial y}`,
		`\displaystyle\sum_{1\le i\le j\le n} x_{ij}`,
	}
	limits := source.DefaultLimits()
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		for _, src := range sources {
			result := tex.Parse(src, ir.Display, tex.Options{Limits: limits})
			if result.Root == nil {
				b.Fatal("nil root")
			}
		}
	}
}
