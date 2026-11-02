package tex

import (
	"strings"
	"testing"

	"github.com/caelyreth/untie/ir"
	"github.com/caelyreth/untie/source"
)

func TestURLPreservesRawDestinationText(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		src        string
		want       string
		wantSource string
	}{
		{
			name: "percent escape",
			src:  `\url{https://example.org/a%20b?x=1#fragment}`,
			want: `https://example.org/a%20b?x=1#fragment`,
		},
		{
			name: "tilde and hash",
			src:  `\url{https://example.org/~user/#section}`,
			want: `https://example.org/~user/#section`,
		},
		{
			name: "balanced braces",
			src:  `\url{https://example.org/{a{b}}}`,
			want: `https://example.org/{a{b}}`,
		},
		{
			name:       "escaped braces",
			src:        `\url{https://example.org/\{a\{b\}\}}`,
			want:       `https://example.org/{a{b}}`,
			wantSource: `https://example.org/\{a\{b\}\}`,
		},
		{
			name: "newline",
			src:  "\\url{https://example.org/a\nb}",
			want: "https://example.org/a\nb",
		},
		{
			name: "UTF-8 path",
			src:  `\url{https://例え.test/路径}`,
			want: `https://例え.test/路径`,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			result := parseTest(t, test.src, Options{})
			assertOnlyLinkDiagnostics(t, result.Diagnostics)
			text := firstText(t, result.Root)
			if text.Text != test.want {
				t.Fatalf("url text = %q, want %q", text.Text, test.want)
			}
			wantSource := test.wantSource
			if wantSource == "" {
				wantSource = test.want
			}
			if got := string(text.NodeBase().Span.Slice(test.src)); got != wantSource {
				t.Fatalf("url span = %q, want %q", got, wantSource)
			}
		})
	}
}

func TestURLPercentDoesNotConsumeFollowingFormula(t *testing.T) {
	t.Parallel()

	const src = `\url{https://example.org/a%20b}+x`
	result := parseTest(t, src, Options{})
	assertOnlyLinkDiagnostics(t, result.Diagnostics)
	items := rowItems(t, result.Root)
	if len(items) != 3 {
		t.Fatalf("root items = %d, want URL, plus, and x", len(items))
	}
	if text := requireKind[*ir.Text](t, items[0]); text.Text != "https://example.org/a%20b" {
		t.Fatalf("url text = %q", text.Text)
	}
}

func TestURLEmptyAndSingleTokenArguments(t *testing.T) {
	t.Parallel()

	empty := parseTest(t, `\url{}`, Options{})
	assertOnlyLinkDiagnostics(t, empty.Diagnostics)
	if text := findText(empty.Root); text != nil {
		t.Fatalf("empty url produced text %q", text.Text)
	}
	if empty.Root.Kind() != ir.KindRow {
		t.Fatalf("empty url root = %v, want row", empty.Root.Kind())
	}

	single := parseTest(t, `\url xy`, Options{})
	assertOnlyLinkDiagnostics(t, single.Diagnostics)
	items := rowItems(t, single.Root)
	if len(items) != 2 {
		t.Fatalf("single-token items = %d, want URL and trailing y", len(items))
	}
	if text := requireKind[*ir.Text](t, items[0]); text.Text != "x" {
		t.Fatalf("single-token url = %q, want x", text.Text)
	}
	if got := string(items[0].NodeBase().Span.Slice(`\url xy`)); got != "x" {
		t.Fatalf("single-token span = %q, want x", got)
	}
}

func TestURLUnclosedAndMissingArguments(t *testing.T) {
	t.Parallel()

	unclosed := parseTest(t, `\url{https://example.org/{a}`, Options{})
	if !hasDiagnostic(unclosed.Diagnostics, source.CodeUnclosedGroup) {
		t.Fatalf("unclosed diagnostics = %v", unclosed.Diagnostics)
	}
	if hasDiagnostic(unclosed.Diagnostics, source.CodeUnsupported) {
		t.Fatalf("unclosed URL emitted link warning: %v", unclosed.Diagnostics)
	}

	missing := parseTest(t, `\url`, Options{})
	if !hasDiagnostic(missing.Diagnostics, source.CodeMissingArgument) {
		t.Fatalf("missing diagnostics = %v", missing.Diagnostics)
	}
	if hasDiagnostic(missing.Diagnostics, source.CodeUnsupported) {
		t.Fatalf("missing URL emitted link warning: %v", missing.Diagnostics)
	}
}

func TestURLUnknownMacroRemainsLiteralText(t *testing.T) {
	t.Parallel()

	const src = `\url{https://example.org/\unknown}`
	result := parseTest(t, src, Options{})
	assertOnlyLinkDiagnostics(t, result.Diagnostics)
	text := firstText(t, result.Root)
	if text.Text != `https://example.org/\unknown` {
		t.Fatalf("unknown macro URL = %q", text.Text)
	}
}

func TestHrefArgumentBoundaries(t *testing.T) {
	t.Parallel()

	valid := []string{
		`\href{}{x}`,
		`\href{https://example.org/a%20b}{x}`,
		`\href{https://example.org/{a}}{x}`,
		`\href x{y}`,
		`\href{x}{}`,
	}
	for _, src := range valid {
		result := parseTest(t, src, Options{})
		assertOnlyLinkDiagnostics(t, result.Diagnostics)
	}

	missingDest := parseTest(t, `\href`, Options{})
	if !hasDiagnostic(missingDest.Diagnostics, source.CodeMissingArgument) ||
		hasDiagnostic(missingDest.Diagnostics, source.CodeUnsupported) {
		t.Fatalf("missing href diagnostics = %v", missingDest.Diagnostics)
	}

	missingBody := parseTest(t, `\href{https://example.org}`, Options{})
	if !hasDiagnostic(missingBody.Diagnostics, source.CodeMissingArgument) ||
		hasDiagnostic(missingBody.Diagnostics, source.CodeUnsupported) {
		t.Fatalf("missing href body diagnostics = %v", missingBody.Diagnostics)
	}

	unclosedDest := parseTest(t, `\href{https://example.org/{a}`, Options{})
	if !hasDiagnostic(unclosedDest.Diagnostics, source.CodeUnclosedGroup) ||
		hasDiagnostic(unclosedDest.Diagnostics, source.CodeUnsupported) {
		t.Fatalf("unclosed href destination diagnostics = %v", unclosedDest.Diagnostics)
	}

	unclosedBody := parseTest(t, `\href{https://example.org}{x`, Options{})
	if !hasDiagnostic(unclosedBody.Diagnostics, source.CodeUnclosedGroup) ||
		hasDiagnostic(unclosedBody.Diagnostics, source.CodeUnsupported) {
		t.Fatalf("unclosed href body diagnostics = %v", unclosedBody.Diagnostics)
	}

	extra := parseTest(t, `\href{a}{b}{c}`, Options{})
	assertOnlyLinkDiagnostics(t, extra.Diagnostics)
	if got := collectURLTestText(extra.Root); got != "bc" {
		t.Fatalf("extra href argument text = %q, want bc", got)
	}
}

func assertOnlyLinkDiagnostics(t *testing.T, diagnostics []source.Diagnostic) {
	t.Helper()
	if len(diagnostics) != 1 || diagnostics[0].Code != source.CodeUnsupported {
		t.Fatalf("link diagnostics = %v, want one unsupported warning", diagnostics)
	}
}

func firstText(t *testing.T, root ir.Node) *ir.Text {
	t.Helper()
	text := findText(root)
	if text == nil {
		t.Fatalf("found no text in %T", root)
	}
	return text
}

func findText(root ir.Node) *ir.Text {
	var found *ir.Text
	ir.Walk(root, func(node ir.Node) bool {
		if text, ok := node.(*ir.Text); ok && found == nil {
			found = text
		}
		return true
	})
	return found
}

func collectURLTestText(root ir.Node) string {
	var out strings.Builder
	ir.Walk(root, func(node ir.Node) bool {
		switch node := node.(type) {
		case *ir.Text:
			out.WriteString(node.Text)
		case *ir.Glyph:
			out.WriteString(node.Text)
		}
		return true
	})
	return out.String()
}

func TestURLTextRejectsNoInvalidSpan(t *testing.T) {
	t.Parallel()

	const src = `\url{https://example.org/路径}`
	result := parseTest(t, src, Options{})
	text := firstText(t, result.Root)
	if strings.Contains(text.Text, "\uFFFD") {
		t.Fatalf("url text contains replacement rune: %q", text.Text)
	}
}

func TestDefinedURLCommandKeepsMacroCatcodes(t *testing.T) {
	t.Parallel()

	const src = `\def\url#1{[#1]}\url{ab}`
	result := parseTest(t, src, Options{})
	if len(result.Diagnostics) != 0 {
		t.Fatalf("defined URL diagnostics = %v", result.Diagnostics)
	}
	if got := collectURLTestText(result.Root); got != "[ab]" {
		t.Fatalf("defined URL body = %q, want [ab]", got)
	}
}
