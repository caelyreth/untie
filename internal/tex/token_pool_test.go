package tex

import (
	"testing"

	"github.com/caelyreth/untie/ir"
	"github.com/caelyreth/untie/source"
)

func TestExpanderReleaseClearsPooledTokens(t *testing.T) {
	t.Parallel()

	e := newExpander(`x+\alpha`, source.DefaultLimits(), nil, func(source.Diagnostic) {})
	slot := e.tokenSlot
	tokens := e.tokens
	if slot == nil || len(tokens) == 0 {
		t.Fatal("expander did not acquire a token buffer")
	}
	e.release()
	for i, item := range tokens {
		if item != (token{}) {
			t.Fatalf("released token %d retained data: %+v", i, item)
		}
	}
	if e.tokens != nil || e.tokenSlot != nil || len(*slot) != 0 {
		t.Fatal("expander retained its released token buffer")
	}
}

func TestParseAfterTokenBufferReusePreservesResult(t *testing.T) {
	t.Parallel()

	first := Parse(`x^2+1`, ir.Inline, Options{})
	if len(first.Diagnostics) != 0 {
		t.Fatalf("first diagnostics = %+v", first.Diagnostics)
	}
	second := Parse(`\frac{a}{b}`, ir.Inline, Options{})
	if len(second.Diagnostics) != 0 {
		t.Fatalf("second diagnostics = %+v", second.Diagnostics)
	}
	if first.Root == nil || second.Root == nil || first.Root == second.Root {
		t.Fatal("reused token storage aliased parse results")
	}
}
