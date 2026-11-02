package typst

import (
	"testing"

	"github.com/caelyreth/untie/source"
)

func TestLexerMathTokensAndByteSpans(t *testing.T) {
	t.Parallel()

	src := "α_i <= β /* note */ + \"x\\n\" + \\u{1F600} + \\/"
	tokens, diagnostics := lex(src, source.DefaultLimits())
	if len(diagnostics) != 0 {
		t.Fatalf("lex diagnostics = %#v", diagnostics)
	}
	wantText := []string{"α", "_", "i", "≤", "β", "+", `"x\n"`, "+", "😀", "+", "/", ""}
	if len(tokens) != len(wantText) {
		t.Fatalf("tokens = %#v", tokens)
	}
	for i, want := range wantText {
		if tokens[i].literal != "" {
			if !tokenLiteralMatches(tokens[i], want) {
				t.Errorf("token %d literal = %q, want %q", i, tokens[i].literal, want)
			}
		} else if !tokenTextMatches(tokens[i], want) {
			t.Errorf("token %d text = %q, want %q", i, tokens[i].text, want)
		}
		if !tokens[i].span.IsValid() || tokens[i].span.Start < 0 || tokens[i].span.End > len(src) {
			t.Errorf("token %d span = %#v", i, tokens[i].span)
		}
	}
	underscore := tokens[1].span
	if got := string(underscore.Slice(src)); got != "_" {
		t.Fatalf("underscore slice = %q", got)
	}
}

func tokenLiteralMatches(tok token, want string) bool {
	return tok.literal == want || (want == `"x\n"` && tok.literal == "x\n")
}

func tokenTextMatches(tok token, want string) bool {
	return tok.text == want || (tok.kind == tokEOF && want == "")
}

func TestLexerReportsBadEscapesAndComments(t *testing.T) {
	t.Parallel()

	inputs := []string{`\u{110000}`, `\u{zz}`, `"unterminated`, `/* open`}
	for _, input := range inputs {
		_, diagnostics := lex(input, source.DefaultLimits())
		if len(diagnostics) == 0 {
			t.Errorf("lex(%q) diagnostics = nil", input)
		}
	}
}

func TestLexerTokenLimit(t *testing.T) {
	t.Parallel()

	tokens, diagnostics := lex("a + b + c", source.Limits{MaxTokens: 2})
	if len(tokens) != 3 || tokens[len(tokens)-1].kind != tokEOF {
		t.Fatalf("tokens = %#v", tokens)
	}
	if len(diagnostics) != 1 || diagnostics[0].Code != source.CodeLimitExceeded {
		t.Fatalf("diagnostics = %#v", diagnostics)
	}
}
