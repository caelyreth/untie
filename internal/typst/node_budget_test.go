package typst

import (
	"testing"

	"github.com/caelyreth/untie/ir"
	"github.com/caelyreth/untie/source"
)

func TestLowerNodeBudgetIsExact(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		src  string
	}{
		{name: "primes", src: `primes(20)`},
		{name: "code primes", src: `#primes(20)`},
		{name: "binomial", src: `binom(1, 2)`},
		{name: "matrix", src: `mat(1, 2; 3, 4)`},
		{name: "vector", src: `vec(1, 2, 3)`},
		{name: "cases", src: `cases(a, b)`},
		{name: "padded matrix", src: `mat(#(1,), #(2, 3))`},
		{name: "differential", src: `dif`},
		{name: "content call", src: `f(x)`},
		{name: "delimited body", src: `(a+b)`},
		{name: "fenced body", src: `abs(a+b)`},
		{name: "lr fence conversion", src: `lr((a+b))`},
		{name: "scriptless attachment unwrap", src: `limits(A)_1`},
		{name: "named fence body", src: `lr(body: (a+b))`},
		{name: "number run", src: `1.25`},
		{name: "combined identifier", src: "a\u0301"},
		{name: "structural accent", src: `overbrace(a+b, n)`},
		{name: "fraction and radical", src: `frac(a,b) + sqrt(c)`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			baseline := lowerAtBudget(t, tt.src, source.DefaultLimits().MaxNodes)
			count := countIRNodes(t, baseline.Root, source.DefaultLimits().MaxNodes)
			if count == 0 {
				t.Fatal("baseline produced an empty tree")
			}

			exact := lowerAtBudget(t, tt.src, count)
			if got := countIRNodes(t, exact.Root, count); got != count {
				t.Fatalf("exact budget produced %d nodes, want %d", got, count)
			}
			if hasDiagnosticCode(exact.Diagnostics, source.CodeLimitExceeded) {
				t.Fatalf("exact budget diagnostics = %#v, want no node-limit error", exact.Diagnostics)
			}
			if err := ir.Validate(&ir.Formula{Root: exact.Root}, limitsWithNodes(count)); err != nil {
				t.Fatalf("exact-budget tree is invalid: %v", err)
			}

			below := lowerAtBudget(t, tt.src, count-1)
			if got := countIRNodes(t, below.Root, count-1); got > count-1 {
				t.Fatalf("below budget produced %d nodes for limit %d", got, count-1)
			}
			if !hasDiagnosticCode(below.Diagnostics, source.CodeLimitExceeded) {
				t.Fatalf("below budget diagnostics = %#v, want node-limit error", below.Diagnostics)
			}
			if err := ir.Validate(&ir.Formula{Root: below.Root}, limitsWithNodes(count-1)); err != nil {
				t.Fatalf("below-budget fallback is invalid: %v", err)
			}
		})
	}
}

func TestLowerNodeBudgetStopsLargeSyntheticTrees(t *testing.T) {
	t.Parallel()

	result := lowerAtBudget(t, `#primes(100000)`, 10)
	if got := countIRNodes(t, result.Root, 10); got > 10 {
		t.Fatalf("large primes produced %d nodes for limit 10", got)
	}
	if !hasDiagnosticCode(result.Diagnostics, source.CodeLimitExceeded) {
		t.Fatalf("diagnostics = %#v, want node-limit error", result.Diagnostics)
	}
	if err := ir.Validate(&ir.Formula{Root: result.Root}, limitsWithNodes(10)); err != nil {
		t.Fatalf("large-primes fallback is invalid: %v", err)
	}
}

func TestParseNodeBudgetFallbackIsValidForMinimumLimits(t *testing.T) {
	t.Parallel()

	for _, maxNodes := range []int{1, 2} {
		limits := limitsWithNodes(maxNodes)
		result := Parse(`#primes(100000)`, ir.Inline, Options{Limits: limits})
		if got := countIRNodes(t, result.Root, maxNodes); got > maxNodes {
			t.Fatalf("MaxNodes=%d produced %d nodes", maxNodes, got)
		}
		if !hasDiagnosticCode(result.Diagnostics, source.CodeLimitExceeded) {
			t.Fatalf("MaxNodes=%d diagnostics = %#v, want node-limit error", maxNodes, result.Diagnostics)
		}
		if err := ir.Validate(&ir.Formula{Root: result.Root}, limits); err != nil {
			t.Fatalf("MaxNodes=%d produced invalid IR: %v", maxNodes, err)
		}
	}
}

func lowerAtBudget(t *testing.T, src string, maxNodes int) Result {
	t.Helper()
	limits := limitsWithNodes(maxNodes)
	root, _ := parseSource(src, ir.Inline, source.DefaultLimits())
	return lower(root, src, limits)
}

func limitsWithNodes(maxNodes int) source.Limits {
	limits := source.DefaultLimits()
	limits.MaxNodes = maxNodes
	return limits
}

func countIRNodes(t *testing.T, root ir.Node, maxNodes int) int {
	t.Helper()
	count := 0
	if err := ir.WalkWithLimits(root, limitsWithNodes(maxNodes), func(ir.Node) bool {
		count++
		return true
	}); err != nil {
		t.Fatalf("walk IR: %v", err)
	}
	return count
}

func hasDiagnosticCode(diagnostics []source.Diagnostic, code source.Code) bool {
	for _, diagnostic := range diagnostics {
		if diagnostic.Code == code {
			return true
		}
	}
	return false
}
