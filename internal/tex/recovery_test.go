package tex

import (
	"testing"

	"github.com/caelyreth/untie/ir"
	"github.com/caelyreth/untie/source"
)

func TestMalformedInputRecoversWithValidIR(t *testing.T) {
	t.Parallel()

	tests := []struct {
		source string
		code   source.Code
		kind   ir.NodeKind
	}{
		{"{a", source.CodeUnclosedGroup, ir.KindRow},
		{"x^", source.CodeMissingArgument, ir.KindScripts},
		{`\frac{1}`, source.CodeMissingArgument, ir.KindFraction},
		{`\sqrt[`, source.CodeUnclosedGroup, ir.KindRadical},
		{`\left(x`, source.CodeUnclosedGroup, ir.KindFenced},
		{`\unknown`, source.CodeUnknownCommand, ir.KindError},
		{"x_1_2", source.CodeUnexpectedToken, ir.KindScripts},
	}
	for _, tc := range tests {
		tc := tc
		t.Run(tc.source, func(t *testing.T) {
			t.Parallel()
			result := parseTest(t, tc.source, Options{})
			if !hasDiagnostic(result.Diagnostics, tc.code) {
				t.Fatalf("diagnostics = %v, want %s", diagnosticCodes(result.Diagnostics), tc.code)
			}
			first := rowItems(t, result.Root)[0]
			if first.NodeBase().Kind() != tc.kind {
				t.Errorf("first kind = %v, want %v", first.NodeBase().Kind(), tc.kind)
			}
		})
	}
	result := parseTest(t, "x}", Options{})
	if !hasDiagnostic(result.Diagnostics, source.CodeUnexpectedToken) {
		t.Fatalf("diagnostics = %v, want unexpected-token", diagnosticCodes(result.Diagnostics))
	}
}

func TestInvalidUTF8ProducesDiagnosticAndNoPanic(t *testing.T) {
	t.Parallel()

	result := parseTest(t, string([]byte{'x', 0xff, '+', '1'}), Options{})
	if !hasDiagnostic(result.Diagnostics, source.CodeSyntax) {
		t.Fatalf("diagnostics = %v, want syntax", diagnosticCodes(result.Diagnostics))
	}
}

func TestNestingLimitStopsDeepGroups(t *testing.T) {
	t.Parallel()

	limits := source.DefaultLimits()
	limits.MaxNesting = 6
	result := parseTest(t, `{{{{{{{{{{x}}}}}}}}}}`, Options{Limits: limits})
	if !hasDiagnostic(result.Diagnostics, source.CodeLimitExceeded) {
		t.Fatalf("diagnostics = %v, want limit diagnostic", diagnosticCodes(result.Diagnostics))
	}
}

func TestSourceByteLimit(t *testing.T) {
	t.Parallel()

	limits := source.DefaultLimits()
	limits.MaxSourceBytes = 2
	result := parseTest(t, "x+y", Options{Limits: limits})
	if !hasDiagnostic(result.Diagnostics, source.CodeLimitExceeded) {
		t.Fatalf("diagnostics = %v, want limit diagnostic", diagnosticCodes(result.Diagnostics))
	}
}

func TestSourceLimitFallbackObeysNodeBudget(t *testing.T) {
	t.Parallel()

	limits := source.DefaultLimits()
	limits.MaxSourceBytes = 1
	limits.MaxNodes = 1
	result := Parse("xy", ir.Display, Options{Limits: limits})
	if err := ir.Validate(&ir.Formula{Root: result.Root}, limits); err != nil {
		t.Fatalf("source-limit fallback violates limits: %v", err)
	}
	if result.Root.NodeBase().Kind() != ir.KindPlaceholder {
		t.Fatalf("fallback kind = %v, want placeholder", result.Root.NodeBase().Kind())
	}
}

func TestDiagnosticsAreDeterministicallyOrdered(t *testing.T) {
	t.Parallel()

	result := parseTest(t, `\unknown{x}\frac{1}`, Options{})
	for i := 1; i < len(result.Diagnostics); i++ {
		previous := result.Diagnostics[i-1]
		current := result.Diagnostics[i]
		if previous.Range.Start > current.Range.Start {
			t.Fatalf("diagnostics are not source ordered: %v", result.Diagnostics)
		}
	}
}

func TestLimitsRequireOperator(t *testing.T) {
	t.Parallel()

	result := parseTest(t, `x\limits_1`, Options{})
	if !hasDiagnostic(result.Diagnostics, source.CodeUnexpectedToken) {
		t.Fatalf("diagnostics = %v, want unexpected-token", diagnosticCodes(result.Diagnostics))
	}
}

func TestUnsupportedConstructsAreExplicit(t *testing.T) {
	t.Parallel()

	tests := []string{
		`\vcenter{x}`,
		`\mathclap{x}`,
	}
	for _, src := range tests {
		src := src
		t.Run(src, func(t *testing.T) {
			t.Parallel()
			result := parseTest(t, src, Options{})
			if !hasDiagnostic(result.Diagnostics, source.CodeUnsupported) {
				t.Fatalf("diagnostics = %v, want unsupported", diagnosticCodes(result.Diagnostics))
			}
		})
	}
}
