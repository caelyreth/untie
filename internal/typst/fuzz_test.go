package typst

import (
	"testing"
	"unicode/utf8"

	"github.com/caelyreth/untie/ir"
	"github.com/caelyreth/untie/source"
)

func FuzzParseNeverPanics(f *testing.F) {
	seeds := []string{
		"", "x", "x_i^2", "1/2", "sqrt(x)", "mat(1,2;3,4)",
		`text(fill: #red, "x")`, "#let x = 1", "/*", "\xff",
		`integral f(x) dif x`, `#text(fill: red, $x$)`,
		`#sym.arrow.r + #math.frac(1, 2)`, `math.frac(1, 2)`,
		`class("relation", a+b)`, `bold(a+b)`, `cases(reverse: #true, a, b)`,
		`mat(#(1, 2), #(3, 4))`, `mat(..#((1, 2), (3, 4)))`,
	}
	for _, seed := range seeds {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, src string) {
		if len(src) > 64*1024 {
			t.Skip()
		}
		limits := source.Limits{
			MaxSourceBytes:    64 * 1024,
			MaxTokens:         10_000,
			MaxNesting:        64,
			MaxNodes:          10_000,
			MaxCallDepth:      16,
			MaxTableCells:     256,
			MaxTextBytes:      16 * 1024,
			MaxExpandedTokens: 64 * 1024,
		}
		result := Parse(src, ir.Inline, Options{
			Limits: limits,
		})
		if result.Root == nil {
			t.Fatal("nil root")
		}
		if err := ir.Validate(&ir.Formula{Root: result.Root}, limits); err != nil {
			t.Fatalf("invalid IR: %v", err)
		}
		for _, diagnostic := range result.Diagnostics {
			if !diagnosticRangeOnUTF8Boundaries(diagnostic.Range, src) {
				t.Fatalf("diagnostic range %#v is outside or splits source", diagnostic.Range)
			}
		}
	})
}

func diagnosticRangeOnUTF8Boundaries(r source.Range, src string) bool {
	if r.IsDetached() {
		return true
	}
	if !r.IsValidFor(len(src)) {
		return false
	}
	boundaries := make(map[int]bool, len(src)+1)
	boundaries[0] = true
	for offset := 0; offset < len(src); {
		_, size := utf8.DecodeRuneInString(src[offset:])
		offset += size
		boundaries[offset] = true
	}
	return boundaries[r.Start] && boundaries[r.End]
}
