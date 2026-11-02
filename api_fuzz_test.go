package untie

import (
	"testing"

	"github.com/caelyreth/untie/ir"
	"github.com/caelyreth/untie/source"
)

func FuzzParseNoPanicAndValidIR(f *testing.F) {
	for _, seed := range []string{
		"",
		"x^2",
		`\frac{1}{2}`,
		`\sqrt[3]{x+y}`,
		`\left(\frac{a}{b}\right)^n`,
		"sum_(i=1)^n i",
		"mat(1, 2; 3, 4)",
		`\def\x{\x}\x`,
		"#let x = 1; #x",
	} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, src string) {
		for _, dialect := range []Dialect{TeX, Typst} {
			expr := Parse(src, dialect, Inline, ParseOptions{
				Limits: source.Limits{
					MaxSourceBytes:     1 << 14,
					MaxTokens:          20_000,
					MaxNesting:         64,
					MaxNodes:           20_000,
					MaxMacroExpansions: 20_000,
					MaxExpandedTokens:  100_000,
					MaxCallDepth:       32,
					MaxTableCells:      1_000,
					MaxAssemblyRepeats: 128,
					MaxTextBytes:       10_000,
					MaxOutputItems:     20_000,
					MaxPathSegments:    100_000,
					MaxDimension:       1 << 32,
				},
			})
			if expr == nil {
				t.Fatal("Parse() = nil")
			}
			if err := ir.Validate(expr.Root(), source.Limits{}); err != nil {
				t.Fatalf("invalid IR for %q: %v", src, err)
			}
			first := expr.Key()
			second := Parse(src, dialect, Inline, ParseOptions{}).Key()
			if first == 0 || second == 0 {
				t.Fatal("zero expression key")
			}
		}
	})
}
