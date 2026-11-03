package tex

import (
	"testing"

	"github.com/caelyreth/untie/ir"
	"github.com/caelyreth/untie/source"
)

func TestImplementedLayoutConstructsAreAccepted(t *testing.T) {
	t.Parallel()

	tests := []struct {
		source string
		kind   ir.NodeKind
	}{
		{source: `\hat{x}`, kind: ir.KindAccent},
		{source: `\cancel{x}`, kind: ir.KindCancel},
		{source: `\begin{aligned}x&=1\end{aligned}`, kind: ir.KindMultiline},
		{source: `\begin{array}{|l|}x\end{array}`, kind: ir.KindTable},
		{source: `\boxed{x}`, kind: ir.KindBox},
		{source: `\phantom{x}`, kind: ir.KindPhantom},
	}
	for _, tc := range tests {
		tc := tc
		t.Run(tc.source, func(t *testing.T) {
			t.Parallel()
			result := parseTest(t, tc.source, Options{})
			for _, diagnostic := range result.Diagnostics {
				if diagnostic.Severity == source.SeverityError {
					t.Fatalf("unexpected error diagnostics: %v", result.Diagnostics)
				}
			}
			first := rowItems(t, result.Root)[0]
			if first.NodeBase().Kind() != tc.kind {
				t.Fatalf("first kind = %v, want %v", first.NodeBase().Kind(), tc.kind)
			}
		})
	}
}

func TestUnsupportedLayoutConstructsAreExplicit(t *testing.T) {
	t.Parallel()

	for _, src := range []string{`\vcenter{x}`} {
		result := parseTest(t, src, Options{})
		if !hasDiagnostic(result.Diagnostics, source.CodeUnsupported) {
			t.Errorf("%s diagnostics = %v, want unsupported", src, diagnosticCodes(result.Diagnostics))
		}
	}

	result := parseTest(t, `x\nobreak y`, Options{})
	foundNoBreak := false
	for _, diagnostic := range result.Diagnostics {
		if diagnostic.Code == source.CodeUnsupported &&
			diagnostic.Message == "line-break suppression is not represented in the neutral IR" {
			foundNoBreak = true
			break
		}
	}
	if !foundNoBreak {
		t.Errorf("nobreak diagnostics = %v, want explicit suppression boundary", diagnosticCodes(result.Diagnostics))
	}
}

func TestHrefRendersBodyAsTextOnly(t *testing.T) {
	t.Parallel()

	const src = `\href{https://example.com}{x^2}`
	result := parseTest(t, src, Options{})
	var scripts *ir.Scripts
	ir.Walk(result.Root, func(node ir.Node) bool {
		if candidate, ok := node.(*ir.Scripts); ok {
			scripts = candidate
		}
		return true
	})
	if scripts == nil {
		t.Fatalf("href body root = %T, want nested scripts", result.Root)
	}
	if got := string(scripts.NodeBase().Span.Slice(src)); got != "x^2" {
		t.Fatalf("href body span = %q, want %q", got, "x^2")
	}
	foundWarning := false
	for _, diagnostic := range result.Diagnostics {
		if diagnostic.Code == source.CodeUnsupported &&
			diagnostic.Message == "hyperlinks are outside the formula IR; link destination was discarded" {
			foundWarning = true
			break
		}
	}
	if !foundWarning {
		t.Fatalf("href diagnostics = %v, want discarded destination warning", result.Diagnostics)
	}
}

func TestURLRendersVisibleTextWithoutDestinationMetadata(t *testing.T) {
	t.Parallel()

	const src = `\url{https://example.com/a#b}`
	result := parseTest(t, src, Options{})
	var text *ir.Text
	ir.Walk(result.Root, func(node ir.Node) bool {
		if candidate, ok := node.(*ir.Text); ok {
			text = candidate
		}
		return true
	})
	if text == nil {
		t.Fatalf("url body root = %T, want text", result.Root)
	}
	if got := string(text.NodeBase().Span.Slice(src)); got != "https://example.com/a#b" {
		t.Fatalf("url body span = %q, want visible URL", got)
	}
	foundWarning := false
	for _, diagnostic := range result.Diagnostics {
		if diagnostic.Code == source.CodeUnsupported &&
			diagnostic.Message == "hyperlinks are outside the formula IR; link destination was discarded" {
			foundWarning = true
			break
		}
	}
	if !foundWarning {
		t.Fatalf("url diagnostics = %v, want discarded destination warning", result.Diagnostics)
	}
}
