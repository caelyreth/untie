package typst

import (
	"math"
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

func TestLexerScientificNotation(t *testing.T) {
	t.Parallel()

	tests := []struct {
		src  string
		want float64
	}{
		{src: "1e5", want: 1e5},
		{src: "1E+5", want: 1e5},
		{src: "1.5e-3", want: 1.5e-3},
		{src: "1e-400", want: 0},
		{src: "5e-324", want: 5e-324},
	}
	for _, test := range tests {
		tokens, diagnostics := lex(test.src, source.DefaultLimits())
		if len(diagnostics) != 0 {
			t.Fatalf("lex(%q) diagnostics = %#v", test.src, diagnostics)
		}
		if len(tokens) != 2 || tokens[0].kind != tokNumber || tokens[0].text != test.src {
			t.Fatalf("lex(%q) tokens = %#v", test.src, tokens)
		}
		value, ok := parseNumericLiteral(tokens[0].text)
		if !ok || value.kind != valueFloat || value.float != test.want || math.IsInf(value.float, 0) || math.IsNaN(value.float) {
			t.Fatalf("parseNumericLiteral(%q) = %#v, %v; want finite %g", test.src, value, ok, test.want)
		}
	}
}

func TestLexerScientificNotationDiagnosticsAndSuffixes(t *testing.T) {
	t.Parallel()

	for _, src := range []string{"1e", "0e", "1.2E+"} {
		tokens, diagnostics := lex(src, source.DefaultLimits())
		if len(tokens) != 2 || tokens[0].kind != tokInvalidNumber || tokens[0].text != src {
			t.Fatalf("lex(%q) tokens = %#v", src, tokens)
		}
		if len(diagnostics) != 1 || diagnostics[0].Code != source.CodeSyntax ||
			diagnostics[0].Range != source.NewRange(0, len(src)) {
			t.Fatalf("lex(%q) diagnostics = %#v, want malformed exponent", src, diagnostics)
		}
	}

	tokens, diagnostics := lex("1e400", source.DefaultLimits())
	if len(diagnostics) != 1 || diagnostics[0].Code != source.CodeInvalidValue {
		t.Fatalf("overflow diagnostics = %#v", diagnostics)
	}
	if len(tokens) != 2 || tokens[0].kind != tokInvalidNumber || tokens[0].text != "1e400" {
		t.Fatalf("overflow tokens = %#v", tokens)
	}

	for _, src := range []string{"e5", "e5em"} {
		tokens, diagnostics := lex(src, source.DefaultLimits())
		if len(diagnostics) != 0 {
			t.Fatalf("lex(%q) diagnostics = %#v", src, diagnostics)
		}
		if len(tokens) != 2 || tokens[0].kind != tokIdent || tokens[0].text != src {
			t.Fatalf("lex(%q) tokens = %#v, want identifier", src, tokens)
		}
	}

	for _, src := range []string{"1em", "1ex", "1e5em"} {
		tokens, diagnostics := lex(src, source.DefaultLimits())
		if len(diagnostics) != 0 {
			t.Fatalf("lex(%q) diagnostics = %#v", src, diagnostics)
		}
		if len(tokens) != 2 || tokens[0].kind != tokNumber || tokens[0].text != src {
			t.Fatalf("lex(%q) tokens = %#v, want numeric suffix", src, tokens)
		}
	}
}

func TestLexerRejectsInvalidUTF8InStrings(t *testing.T) {
	t.Parallel()

	tests := []struct {
		src     string
		literal string
		at      int
	}{
		{src: "\"a\xffb\"", literal: "ab", at: 2},
		{src: "\"\\\xffb\"", literal: "b", at: 2},
	}
	for _, test := range tests {
		tokens, diagnostics := lex(test.src, source.DefaultLimits())
		if len(tokens) != 2 || tokens[0].kind != tokString {
			t.Fatalf("lex(%q) tokens = %#v", test.src, tokens)
		}
		if tokens[0].literal != test.literal {
			t.Fatalf("lex(%q) literal = %q, want %q", test.src, tokens[0].literal, test.literal)
		}
		if len(diagnostics) != 1 || diagnostics[0].Code != source.CodeSyntax ||
			diagnostics[0].Range != source.NewRange(test.at, test.at+1) {
			t.Fatalf("lex(%q) diagnostics = %#v, want invalid byte at %d:%d", test.src, diagnostics, test.at, test.at+1)
		}
	}
}
