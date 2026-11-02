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

	for _, src := range []string{`\vcenter{x}`, `\mathclap{x}`, `\ce{H2O}`} {
		result := parseTest(t, src, Options{})
		if !hasDiagnostic(result.Diagnostics, source.CodeUnsupported) {
			t.Errorf("%s diagnostics = %v, want unsupported", src, diagnosticCodes(result.Diagnostics))
		}
	}

	result := parseTest(t, `x\nobreak y`, Options{})
	if !hasDiagnostic(result.Diagnostics, source.CodeUnsupported) {
		t.Errorf("nobreak diagnostics = %v, want unsupported", diagnosticCodes(result.Diagnostics))
	}
}
