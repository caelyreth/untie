package tex

import (
	"testing"

	"github.com/caelyreth/untie/ir"
	"github.com/caelyreth/untie/source"
)

func TestMacroExpansionAndOriginSpans(t *testing.T) {
	t.Parallel()

	src := `\def\foo{x}\foo`
	result := parseTest(t, src, Options{})
	items := rowItems(t, result.Root)
	if len(items) != 1 {
		t.Fatalf("got %d items, want 1", len(items))
	}
	glyph := requireKind[*ir.Glyph](t, items[0])
	if glyph.Text != "x" {
		t.Fatalf("glyph = %q", glyph.Text)
	}
	if glyph.Span != source.NewRange(9, 10) {
		t.Errorf("definition span = %v, want [9,10)", glyph.Span)
	}
	if glyph.Origin != source.NewRange(11, 15) {
		t.Errorf("invocation origin = %v, want [11,15)", glyph.Origin)
	}
}

func TestCompoundMacroOutputRetainsInvocationOrigin(t *testing.T) {
	t.Parallel()

	src := `\def\half{\frac{1}{2}}\half`
	result := parseTest(t, src, Options{})
	fraction := requireKind[*ir.Fraction](t, rowItems(t, result.Root)[0])
	invocation := source.NewRange(len(src)-len(`\half`), len(src))
	if fraction.Origin != invocation {
		t.Errorf("fraction origin = %v, want %v", fraction.Origin, invocation)
	}
	if fraction.Span == invocation {
		t.Error("fraction definition span was replaced by its invocation origin")
	}
}

func TestMacroArgumentsAndNesting(t *testing.T) {
	t.Parallel()

	src := `\def\wrap#1{[#1]}\wrap{x^2}`
	result := parseTest(t, src, Options{})
	if len(result.Diagnostics) != 0 {
		t.Fatalf("diagnostics: %v", result.Diagnostics)
	}
	items := rowItems(t, result.Root)
	if len(items) != 3 {
		t.Fatalf("got %d items, want 3", len(items))
	}
	if glyph := requireKind[*ir.Glyph](t, items[0]); glyph.Text != "[" {
		t.Errorf("first glyph = %q", glyph.Text)
	}
	if items[1].NodeBase().Kind() != ir.KindScripts {
		t.Errorf("argument was not parsed as an expression: %T", items[1])
	}
	if glyph := requireKind[*ir.Glyph](t, items[2]); glyph.Text != "]" {
		t.Errorf("closing glyph = %q", glyph.Text)
	}
}

func TestNewCommandFormsAndScoping(t *testing.T) {
	t.Parallel()

	src := `\newcommand{\sq}[1]{#1^2}\sq{x}{\renewcommand{\sq}[1]{#1^3}\sq{y}}\sq{z}`
	result := parseTest(t, src, Options{})
	if len(result.Diagnostics) != 0 {
		t.Fatalf("diagnostics: %v", result.Diagnostics)
	}

	var scripts []*ir.Scripts
	ir.Walk(result.Root, func(n ir.Node) bool {
		if s, ok := n.(*ir.Scripts); ok {
			scripts = append(scripts, s)
		}
		return true
	})
	if len(scripts) != 3 {
		t.Fatalf("got %d script nodes, want 3", len(scripts))
	}
}

func TestProvideCommandDoesNotRedefine(t *testing.T) {
	t.Parallel()

	result := parseTest(t, `\newcommand{\foo}{1}\providecommand{\foo}{2}\foo`, Options{})
	if len(result.Diagnostics) != 0 {
		t.Fatalf("diagnostics: %v", result.Diagnostics)
	}
	items := rowItems(t, result.Root)
	if len(items) != 1 {
		t.Fatalf("got %d items, want 1", len(items))
	}
	if glyph := requireKind[*ir.Glyph](t, items[0]); glyph.Text != "1" {
		t.Errorf("providecommand replaced an existing macro: %q", glyph.Text)
	}
}

func TestHostMacros(t *testing.T) {
	t.Parallel()

	result := parseTest(t, `\pair{x}{y}`, Options{
		Macros: map[string]Macro{
			"pair": {Body: `(#1,#2)`, Args: 2},
		},
	})
	if len(result.Diagnostics) != 0 {
		t.Fatalf("diagnostics: %v", result.Diagnostics)
	}
	items := rowItems(t, result.Root)
	if len(items) != 5 {
		t.Fatalf("got %d items, want 5", len(items))
	}
}

func TestMacroArgumentRetainsSourceSpan(t *testing.T) {
	t.Parallel()

	src := `\def\id#1{#1}\id{ab}`
	result := parseTest(t, src, Options{})
	items := rowItems(t, result.Root)
	if len(items) != 2 {
		t.Fatalf("got %d items, want 2", len(items))
	}
	if got := items[0].NodeBase().Span; got != source.NewRange(17, 18) {
		t.Errorf("first argument span = %v, want [17,18)", got)
	}
	if got := items[1].NodeBase().Span; got != source.NewRange(18, 19) {
		t.Errorf("second argument span = %v, want [18,19)", got)
	}
}

func TestHostMacroNormalizationIsDeterministic(t *testing.T) {
	t.Parallel()

	opts := Options{Macros: map[string]Macro{
		`\same`: {Body: "x"},
		"same":  {Body: "y"},
	}}
	first := parseTest(t, `\same`, opts)
	second := parseTest(t, `\same`, opts)
	if !hasDiagnostic(first.Diagnostics, source.CodeInvalidValue) {
		t.Fatalf("diagnostics = %v, want duplicate-name diagnostic", diagnosticCodes(first.Diagnostics))
	}
	firstGlyph := requireKind[*ir.Glyph](t, rowItems(t, first.Root)[0])
	secondGlyph := requireKind[*ir.Glyph](t, rowItems(t, second.Root)[0])
	if firstGlyph.Text != secondGlyph.Text {
		t.Errorf("nondeterministic duplicate resolution: %q vs %q", firstGlyph.Text, secondGlyph.Text)
	}
}

func TestMacroRecursionStopsAtLimit(t *testing.T) {
	t.Parallel()

	limits := source.DefaultLimits()
	limits.MaxCallDepth = 8
	limits.MaxMacroExpansions = 100
	result := parseTest(t, `\def\loop{\loop}\loop`, Options{Limits: limits})
	if !hasDiagnostic(result.Diagnostics, source.CodeLimitExceeded) {
		t.Fatalf("diagnostics = %v, want limit diagnostic", diagnosticCodes(result.Diagnostics))
	}
}

func TestMacroMissingArgumentIsDiagnosed(t *testing.T) {
	t.Parallel()

	result := parseTest(t, `\def\foo#1{#1}\foo`, Options{})
	if !hasDiagnostic(result.Diagnostics, source.CodeMissingArgument) {
		t.Fatalf("diagnostics = %v, want missing-argument", diagnosticCodes(result.Diagnostics))
	}
}

func TestMacroExpansionBudgetStopsGrowth(t *testing.T) {
	t.Parallel()

	limits := source.DefaultLimits()
	limits.MaxMacroExpansions = 4
	limits.MaxExpandedTokens = 16
	result := parseTest(t, `\def\a{\a\a}\a`, Options{Limits: limits})
	if !hasDiagnostic(result.Diagnostics, source.CodeLimitExceeded) {
		t.Fatalf("diagnostics = %v, want limit diagnostic", diagnosticCodes(result.Diagnostics))
	}
}

func TestNewCommandRejectsExistingBuiltin(t *testing.T) {
	t.Parallel()

	result := parseTest(t, `\newcommand{\sum}{x}`, Options{})
	if !hasDiagnostic(result.Diagnostics, source.CodeInvalidValue) {
		t.Fatalf("diagnostics = %v, want invalid-value", diagnosticCodes(result.Diagnostics))
	}
}

func TestUnsupportedMacroDefinitionsDoNotBecomeApproximations(t *testing.T) {
	t.Parallel()

	result := parseTest(t, `\edef\foo{x}\foo`, Options{})
	if !hasDiagnostic(result.Diagnostics, source.CodeUnsupported) {
		t.Fatalf("diagnostics = %v, want unsupported", diagnosticCodes(result.Diagnostics))
	}
	if !hasDiagnostic(result.Diagnostics, source.CodeUnknownCommand) {
		t.Fatalf("diagnostics = %v, want ignored definition to leave the command undefined", diagnosticCodes(result.Diagnostics))
	}
}
