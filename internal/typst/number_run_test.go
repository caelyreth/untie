package typst

import (
	"testing"

	"github.com/caelyreth/untie/ir"
	"github.com/caelyreth/untie/source"
)

func TestDecimalNumbersLowerToNumberGlyphRuns(t *testing.T) {
	t.Parallel()

	result := parse(t, `9.81 + 2.`)
	requireNoDiagnostics(t, "decimal number runs", result)
	var glyphs []*ir.Glyph
	ir.Walk(result.Root, func(node ir.Node) bool {
		if glyph, ok := node.(*ir.Glyph); ok && (glyph.Text == "." || (glyph.Text >= "0" && glyph.Text <= "9")) {
			glyphs = append(glyphs, glyph)
		}
		return true
	})
	if len(glyphs) != 6 {
		t.Fatalf("number glyphs = %d, want 6", len(glyphs))
	}
	want := []string{"9", ".", "8", "1", "2", "."}
	for i, text := range want {
		if glyphs[i].Text != text {
			t.Fatalf("glyph %d = %q, want %q", i, glyphs[i].Text, text)
		}
		if glyphs[i].Class != ir.ClassNormal {
			t.Fatalf("glyph %d class = %v, want normal", i, glyphs[i].Class)
		}
	}
	if glyphs[0].Span != source.NewRange(0, 1) ||
		glyphs[1].Span != source.NewRange(1, 2) ||
		glyphs[2].Span != source.NewRange(2, 3) ||
		glyphs[3].Span != source.NewRange(3, 4) {
		t.Fatalf("decimal spans = %v, %v, %v, %v", glyphs[0].Span, glyphs[1].Span, glyphs[2].Span, glyphs[3].Span)
	}
}

func TestScientificNumbersParseAndLower(t *testing.T) {
	t.Parallel()

	for _, src := range []string{"1e5", "1E+5", "1.5e-3", "1e-400", "5e-324", "1e5em"} {
		result := parse(t, src)
		requireNoDiagnostics(t, src, result)
		if got := nodeText(result.Root); got != src {
			t.Fatalf("math %q lowered to %q, want source glyph run", src, got)
		}
	}

	tests := []struct {
		src  string
		want string
	}{
		{src: "#1e5", want: "100000"},
		{src: "#1E+5", want: "100000"},
		{src: "#1.5e-3", want: "0.0015"},
		{src: "#1e-400", want: "0"},
		{src: "#5e-324", want: "5e-324"},
	}
	for _, test := range tests {
		result := parse(t, test.src)
		requireNoDiagnostics(t, test.src, result)
		if got := nodeText(result.Root); got != test.want {
			t.Fatalf("%q lowered to %q, want %q", test.src, got, test.want)
		}
	}
}

func TestScientificNumberDiagnostics(t *testing.T) {
	t.Parallel()

	for _, src := range []string{"1e", "1.2E+"} {
		result := parse(t, src)
		if len(result.Diagnostics) != 1 || result.Diagnostics[0].Code != source.CodeSyntax {
			t.Fatalf("%q diagnostics = %#v, want malformed exponent", src, result.Diagnostics)
		}
	}

	overflow := parse(t, "1e400")
	if len(overflow.Diagnostics) != 1 || overflow.Diagnostics[0].Code != source.CodeInvalidValue {
		t.Fatalf("overflow diagnostics = %#v, want non-finite number", overflow.Diagnostics)
	}

	for _, src := range []string{"#1e", "#1e400"} {
		result := parse(t, src)
		if len(result.Diagnostics) != 1 {
			t.Fatalf("%q diagnostics = %#v, want one lexer diagnostic", src, result.Diagnostics)
		}
	}

	identifier := parse(t, "e5")
	if len(identifier.Diagnostics) != 1 || identifier.Diagnostics[0].Code != source.CodeUnknownVariable {
		t.Fatalf("e5 diagnostics = %#v, want ordinary identifier resolution", identifier.Diagnostics)
	}
}

func TestInvalidUTF8InStringIsDiagnosedWithoutReplacementText(t *testing.T) {
	t.Parallel()

	src := "\"a\xffb\""
	result := parse(t, src)
	if len(result.Diagnostics) != 1 || result.Diagnostics[0].Code != source.CodeSyntax ||
		result.Diagnostics[0].Range != source.NewRange(2, 3) {
		t.Fatalf("diagnostics = %#v, want invalid byte at 2:3", result.Diagnostics)
	}
	if got := nodeText(result.Root); got != "ab" {
		t.Fatalf("lowered text = %q, want %q", got, "ab")
	}
}
