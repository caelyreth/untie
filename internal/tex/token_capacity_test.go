package tex

import (
	"testing"

	"github.com/caelyreth/untie/source"
)

func TestTokenCapacityHintCoversLexerOutput(t *testing.T) {
	t.Parallel()

	tests := []string{
		"",
		"x",
		`x^2+\frac{1}{2}+\sqrt{y}`,
		"a b\t c\n d",
		`\alpha@beta \@ \%`,
		"before% ignored\n after",
		`\url{https://example.com/a b}`,
		"α+β",
		"\\",
		"x\\",
	}
	for _, src := range tests {
		limits := source.DefaultLimits()
		lexer := newLexer(src, limits, false, func(source.Diagnostic) {})
		tokens := lexer.appendAll(nil)
		hint := tokenCapacityHint(src, limits.MaxTokens)
		if hint < len(tokens) {
			t.Errorf("tokenCapacityHint(%q) = %d, lexer emitted %d tokens", src, hint, len(tokens))
		}
		if hint > limits.MaxTokens+1 {
			t.Errorf("tokenCapacityHint(%q) = %d exceeds configured limit", src, hint)
		}
	}
}

func TestTokenCapacityHintHonorsTokenLimit(t *testing.T) {
	t.Parallel()

	const maxTokens = 2
	if got := tokenCapacityHint("a+b+c+d", maxTokens); got != maxTokens+1 {
		t.Fatalf("tokenCapacityHint = %d, want capped capacity %d", got, maxTokens+1)
	}
}
