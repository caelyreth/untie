package tex

import (
	"testing"

	"github.com/caelyreth/untie/ir"
	"github.com/caelyreth/untie/source"
)

func TestLayoutUnsupportedConstructsAreDiagnosedAndPreserved(t *testing.T) {
	t.Parallel()

	tests := []struct {
		source string
		kind   ir.NodeKind
	}{
		{source: `\hat{x}`, kind: ir.KindAccent},
		{source: `\cancel{x}`, kind: ir.KindCancel},
		{source: `\begin{aligned}x&=1\end{aligned}`, kind: ir.KindMultiline},
		{source: `\begin{array}{|l|}x\end{array}`, kind: ir.KindTable},
	}
	for _, tc := range tests {
		tc := tc
		t.Run(tc.source, func(t *testing.T) {
			t.Parallel()
			result := parseTest(t, tc.source, Options{})
			if !hasDiagnostic(result.Diagnostics, source.CodeUnsupported) {
				t.Fatalf("diagnostics = %v, want unsupported", diagnosticCodes(result.Diagnostics))
			}
			first := rowItems(t, result.Root)[0]
			if first.NodeBase().Kind() != tc.kind {
				t.Fatalf("first kind = %v, want %v", first.NodeBase().Kind(), tc.kind)
			}
		})
	}
}

func TestUnimplementedBoxAndPhantomAreNotSilentlyAccepted(t *testing.T) {
	t.Parallel()

	for _, src := range []string{`\boxed{x}`, `\phantom{x}`} {
		result := parseTest(t, src, Options{})
		if !hasDiagnostic(result.Diagnostics, source.CodeUnknownCommand) {
			t.Errorf("%s diagnostics = %v, want unknown-command", src, diagnosticCodes(result.Diagnostics))
		}
	}

	result := parseTest(t, `x\nobreak y`, Options{})
	if !hasDiagnostic(result.Diagnostics, source.CodeUnsupported) {
		t.Errorf("nobreak diagnostics = %v, want unsupported", diagnosticCodes(result.Diagnostics))
	}
}
