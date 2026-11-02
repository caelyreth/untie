package typst

import (
	"testing"

	"github.com/caelyreth/untie/ir"
	"github.com/caelyreth/untie/source"
)

func TestLinkDestinationAndBodyValidation(t *testing.T) {
	t.Parallel()

	valid := []struct {
		name   string
		source string
	}{
		{name: "positional body", source: `#link("https://example.org", $x^2$)`},
		{name: "named body", source: `#link(dest: "https://example.org", body: $x^2$)`},
		{name: "omitted URL body", source: `#link("https://example.org")`},
	}
	for _, test := range valid {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			result := parse(t, test.source)
			assertLinkDiagnostics(t, result.Diagnostics, source.CodeUnsupported)
		})
	}

	invalid := []struct {
		name   string
		source string
		code   source.Code
	}{
		{name: "missing destination", source: `#link()`, code: source.CodeMissingArgument},
		{name: "missing destination with body", source: `#link(body: $x$)`, code: source.CodeMissingArgument},
		{name: "empty destination", source: `#link("", $x$)`, code: source.CodeInvalidValue},
		{name: "numeric destination", source: `#link(1, $x$)`, code: source.CodeInvalidValue},
		{name: "numeric body", source: `#link("https://example.org", 1)`, code: source.CodeInvalidValue},
		{name: "none body", source: `#link("https://example.org", body: none)`, code: source.CodeInvalidValue},
		{name: "missing explicit body", source: `#link("https://example.org", body:)`, code: source.CodeMissingArgument},
		{name: "extra positional", source: `#link("https://example.org", $x$, $y$)`, code: source.CodeSyntax},
		{name: "unknown named", source: `#link("https://example.org", unknown: $x$)`, code: source.CodeSyntax},
	}
	for _, test := range invalid {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			result := parse(t, test.source)
			assertLinkDiagnostics(t, result.Diagnostics, test.code)
		})
	}
}

func TestOmittedLinkBodyRendersDestinationText(t *testing.T) {
	t.Parallel()

	const src = `#link("https://example.org/a%20b")`
	result := parse(t, src)
	assertLinkDiagnostics(t, result.Diagnostics, source.CodeUnsupported)
	var text *ir.Text
	ir.Walk(result.Root, func(node ir.Node) bool {
		if candidate, ok := node.(*ir.Text); ok {
			text = candidate
		}
		return true
	})
	if text == nil {
		t.Fatalf("link root = %T, want destination text", result.Root)
	}
	if text.Text != "https://example.org/a%20b" {
		t.Fatalf("link body = %q", text.Text)
	}
}

func assertLinkDiagnostics(t *testing.T, diagnostics []source.Diagnostic, want source.Code) {
	t.Helper()
	if want == source.CodeUnsupported {
		if len(diagnostics) != 1 || diagnostics[0].Code != want {
			t.Fatalf("link diagnostics = %v, want one unsupported warning", diagnostics)
		}
		return
	}
	found := false
	for _, diagnostic := range diagnostics {
		if diagnostic.Code == want {
			found = true
		}
		if want != source.CodeUnsupported && diagnostic.Code == source.CodeUnsupported {
			t.Fatalf("link diagnostics = %v, unexpected unsupported warning", diagnostics)
		}
	}
	if !found {
		t.Fatalf("link diagnostics = %v, want code %s", diagnostics, want)
	}
}
