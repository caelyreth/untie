package typst

import (
	"testing"

	"github.com/caelyreth/untie/ir"
	"github.com/caelyreth/untie/source"
)

func FuzzParseNeverPanics(f *testing.F) {
	seeds := []string{
		"", "x", "x_i^2", "1/2", "sqrt(x)", "mat(1,2;3,4)",
		`text(fill: #red, "x")`, "#let x = 1", "/*", "\xff",
	}
	for _, seed := range seeds {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, src string) {
		if len(src) > 64*1024 {
			t.Skip()
		}
		result := Parse(src, ir.Inline, Options{
			Limits: source.Limits{
				MaxSourceBytes:    64 * 1024,
				MaxTokens:         10_000,
				MaxNesting:        64,
				MaxNodes:          10_000,
				MaxCallDepth:      16,
				MaxTableCells:     256,
				MaxTextBytes:      16 * 1024,
				MaxExpandedTokens: 64 * 1024,
			},
		})
		if result.Root == nil {
			t.Fatal("nil root")
		}
		if err := ir.Validate(&ir.Formula{Root: result.Root}, source.Limits{
			MaxNodes:     20_000,
			MaxNesting:   128,
			MaxTextBytes: 32 * 1024,
		}); err != nil {
			t.Fatalf("invalid IR: %v", err)
		}
	})
}
