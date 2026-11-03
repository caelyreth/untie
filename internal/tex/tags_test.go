package tex

import (
	"strings"
	"testing"

	"github.com/caelyreth/untie/ir"
	"github.com/caelyreth/untie/source"
)

func TestDisplayManualTagsBecomeFormulaMetadata(t *testing.T) {
	t.Parallel()

	result := parseTest(t, `\tag{\frac{1}{2}}x`, Options{})
	for _, diagnostic := range result.Diagnostics {
		if diagnostic.Severity == source.SeverityError {
			t.Fatalf("unexpected error diagnostic: %+v", diagnostic)
		}
	}
	if len(result.Tags) != 1 {
		t.Fatalf("tags = %+v, want one", result.Tags)
	}
	tag := result.Tags[0]
	if !tag.Manual || tag.Suppressed || tag.Starred || tag.Line != ir.FormulaTagLine {
		t.Fatalf("tag metadata = %+v", tag)
	}
	if tag.Span != source.NewRange(0, 17) || tag.Origin != tag.Span {
		t.Fatalf("tag spans = %v/%v, want [0,17)", tag.Span, tag.Origin)
	}
	if _, ok := tag.Body.(*ir.Row); !ok {
		t.Fatalf("tag body = %T, want row", tag.Body)
	}
	if got := collectGlyphText(tag.Body); got != "12" {
		t.Fatalf("tag glyphs = %q, want 12", got)
	}
	if got := collectGlyphText(result.Root); got != "x" {
		t.Fatalf("root glyphs = %q, want tag removed from root", got)
	}
}

func TestTagStarAndSuppressionMetadata(t *testing.T) {
	t.Parallel()

	result := parseTest(t, `\tag*{A}\nonumber x`, Options{})
	if len(result.Tags) != 2 {
		t.Fatalf("tags = %+v, want manual and suppression", result.Tags)
	}
	manual, suppression := result.Tags[0], result.Tags[1]
	if !manual.Manual || !manual.Starred || manual.Suppressed {
		t.Fatalf("manual tag = %+v", manual)
	}
	if suppression.Manual || !suppression.Suppressed || suppression.Body != nil {
		t.Fatalf("suppression tag = %+v", suppression)
	}
	if got := collectGlyphText(result.Root); got != "x" {
		t.Fatalf("root glyphs = %q, want x", got)
	}
}

func TestTopLevelTagAfterDeclarativeStyle(t *testing.T) {
	t.Parallel()

	result := parseTest(t, `\displaystyle x\tag{1}`, Options{})
	if len(result.Tags) != 1 || !result.Tags[0].Manual {
		t.Fatalf("tags = %+v, want one manual tag", result.Tags)
	}
	for _, diagnostic := range result.Diagnostics {
		if diagnostic.Severity == source.SeverityError {
			t.Fatalf("unexpected diagnostic: %+v", diagnostic)
		}
	}
}

func TestMacroExpandedTagUsesContiguousInvocationMetadata(t *testing.T) {
	t.Parallel()

	src := `\def\tagme{\tag{1}}x\tagme`
	result := parseTest(t, src, Options{})
	if len(result.Tags) != 1 {
		t.Fatalf("tags = %+v, want one", result.Tags)
	}
	start := strings.LastIndex(src, `\tagme`)
	want := source.NewRange(start, len(src))
	if result.Tags[0].Span != want || result.Tags[0].Origin != want {
		t.Fatalf("tag span/origin = %v/%v, want %v", result.Tags[0].Span, result.Tags[0].Origin, want)
	}
}

func TestTagFailurePaths(t *testing.T) {
	t.Parallel()

	t.Run("multiple tags", func(t *testing.T) {
		t.Parallel()
		result := parseTest(t, `\tag{1}\tag{2}x`, Options{})
		if len(result.Tags) != 1 {
			t.Fatalf("tags = %+v, want first tag only", result.Tags)
		}
		if !hasDiagnostic(result.Diagnostics, source.CodeInvalidValue) {
			t.Fatalf("diagnostics = %v, want invalid-value", diagnosticCodes(result.Diagnostics))
		}
	})

	t.Run("empty tag", func(t *testing.T) {
		t.Parallel()
		result := parseTest(t, `\tag{}x`, Options{})
		if len(result.Tags) != 1 {
			t.Fatalf("tags = %+v, want visible empty-tag placeholder", result.Tags)
		}
		if _, ok := result.Tags[0].Body.(*ir.ErrorNode); !ok {
			t.Fatalf("tag body = %T, want error node", result.Tags[0].Body)
		}
		if !hasDiagnostic(result.Diagnostics, source.CodeInvalidValue) {
			t.Fatalf("diagnostics = %v, want invalid-value", diagnosticCodes(result.Diagnostics))
		}
	})

	t.Run("missing tag", func(t *testing.T) {
		t.Parallel()
		result := parseTest(t, `\tag`, Options{})
		if len(result.Tags) != 1 {
			t.Fatalf("tags = %+v, want visible missing-tag placeholder", result.Tags)
		}
		if _, ok := result.Tags[0].Body.(*ir.Placeholder); !ok {
			t.Fatalf("tag body = %T, want placeholder", result.Tags[0].Body)
		}
		if !hasDiagnostic(result.Diagnostics, source.CodeMissingArgument) {
			t.Fatalf("diagnostics = %v, want missing-argument", diagnosticCodes(result.Diagnostics))
		}
	})

	t.Run("inline", func(t *testing.T) {
		t.Parallel()
		result := Parse(`x\tag{1}`, ir.Inline, Options{})
		if len(result.Tags) != 0 {
			t.Fatalf("inline tags = %+v, want none", result.Tags)
		}
		if !hasDiagnostic(result.Diagnostics, source.CodeUnsupported) {
			t.Fatalf("diagnostics = %v, want unsupported", diagnosticCodes(result.Diagnostics))
		}
	})

	t.Run("nested group", func(t *testing.T) {
		t.Parallel()
		result := parseTest(t, `{x\tag{1}}`, Options{})
		if len(result.Tags) != 0 {
			t.Fatalf("nested tags = %+v, want none", result.Tags)
		}
		if !hasDiagnostic(result.Diagnostics, source.CodeUnsupported) {
			t.Fatalf("diagnostics = %v, want unsupported", diagnosticCodes(result.Diagnostics))
		}
	})

	t.Run("environment tag", func(t *testing.T) {
		t.Parallel()
		result := parseTest(t, `\begin{aligned}x&=1\tag{2}\end{aligned}`, Options{})
		if len(result.Tags) != 1 || result.Tags[0].Line != 0 {
			t.Fatalf("environment tags = %+v, want one row tag", result.Tags)
		}
		for _, diagnostic := range result.Diagnostics {
			if diagnostic.Severity == source.SeverityError {
				t.Fatalf("environment tag diagnostics = %v", result.Diagnostics)
			}
		}
	})

	t.Run("tag around multiline root", func(t *testing.T) {
		t.Parallel()
		result := parseTest(t, `\begin{aligned}x&=1\end{aligned}\tag{2}`, Options{})
		if len(result.Tags) != 1 || !result.Tags[0].Manual {
			t.Fatalf("tags = %+v, want a top-level tag on the multiline value", result.Tags)
		}
		for _, diagnostic := range result.Diagnostics {
			if diagnostic.Severity == source.SeverityError {
				t.Fatalf("unexpected diagnostic: %+v", diagnostic)
			}
		}
	})
}

func TestMultilineEnvironmentTagsCarrySourceRow(t *testing.T) {
	t.Parallel()

	result := parseTest(t, `\begin{align}x&=1\tag{A}\\y&=2\tag*{B}\\z&=3\nonumber\end{align}`, Options{})
	for _, diagnostic := range result.Diagnostics {
		if diagnostic.Severity == source.SeverityError {
			t.Fatalf("multiline tag diagnostics = %v", result.Diagnostics)
		}
	}
	if len(result.Tags) != 3 {
		t.Fatalf("tags = %+v, want two manual and one suppression", result.Tags)
	}
	if result.Tags[0].Line != 0 || !result.Tags[0].Manual || result.Tags[0].Starred {
		t.Fatalf("row 0 tag = %+v", result.Tags[0])
	}
	if result.Tags[1].Line != 1 || !result.Tags[1].Manual || !result.Tags[1].Starred {
		t.Fatalf("row 1 tag = %+v", result.Tags[1])
	}
	if result.Tags[2].Line != 2 || !result.Tags[2].Suppressed {
		t.Fatalf("row 2 suppression = %+v", result.Tags[2])
	}
}

func TestMultilineTagConflictsStayRowLocal(t *testing.T) {
	t.Parallel()

	tests := []string{
		`\begin{align}x&=1\tag{A}\tag{B}\\y&=2\end{align}`,
		`\begin{align}x&=1\tag{A}\nonumber\\y&=2\end{align}`,
	}
	for _, src := range tests {
		result := parseTest(t, src, Options{})
		if !hasDiagnostic(result.Diagnostics, source.CodeInvalidValue) {
			t.Errorf("%s diagnostics = %v, want invalid-value", src, diagnosticCodes(result.Diagnostics))
		}
	}
}

func TestNestedMultilineTagsRemainExplicitlyUnsupported(t *testing.T) {
	t.Parallel()

	result := parseTest(t, `\begin{equation}\begin{aligned}x&=1\tag{A}\end{aligned}\end{equation}`, Options{})
	if !hasDiagnostic(result.Diagnostics, source.CodeUnsupported) {
		t.Fatalf("diagnostics = %v, want unsupported nested row tag", diagnosticCodes(result.Diagnostics))
	}
	if len(result.Tags) != 0 {
		t.Fatalf("nested tags = %+v, want none", result.Tags)
	}
}

func TestTagsCannotBePromotedFromSyntacticArguments(t *testing.T) {
	t.Parallel()

	parsedSources := []string{
		`x^\tag{1}`,
		`x^\nonumber`,
		`\sqrt\tag{1}`,
		`\frac\tag{1}{2}`,
		`\overset\tag{1}{x}`,
		`\sqrt[\tag{1}]{x}`,
	}
	for _, src := range parsedSources {
		src := src
		t.Run(src, func(t *testing.T) {
			t.Parallel()
			result := parseTest(t, src, Options{})
			if len(result.Tags) != 0 {
				t.Fatalf("tags = %+v, want no promoted tag", result.Tags)
			}
			if !hasDiagnostic(result.Diagnostics, source.CodeUnsupported) {
				t.Fatalf("diagnostics = %v, want unsupported", diagnosticCodes(result.Diagnostics))
			}
		})
	}

	rawSources := []string{
		`\href{\tag{1}}{x}`,
		`\textcolor{\tag{1}}{x}`,
		`\big\tag{1}`,
	}
	for _, src := range rawSources {
		src := src
		t.Run(src, func(t *testing.T) {
			t.Parallel()
			result := parseTest(t, src, Options{})
			if len(result.Tags) != 0 {
				t.Fatalf("tags = %+v, want no promoted tag", result.Tags)
			}
		})
	}
}

func TestTagsCannotBePromotedThroughMacroExpandedArguments(t *testing.T) {
	t.Parallel()

	sources := []string{
		`\def\mytag{\tag{1}}x^\mytag`,
		`\def\mytag{\tag{1}}\sqrt\mytag`,
		`\def\mytag{\tag{1}}\sqrt[\mytag]{x}`,
		`\def\mytag{\tag{1}}\frac\mytag{2}`,
		`\def\mytag{\nonumber}x^\mytag`,
	}
	for _, src := range sources {
		src := src
		t.Run(src, func(t *testing.T) {
			t.Parallel()
			result := parseTest(t, src, Options{})
			if len(result.Tags) != 0 {
				t.Fatalf("tags = %+v, want no promoted tag", result.Tags)
			}
			if !hasDiagnostic(result.Diagnostics, source.CodeUnsupported) {
				t.Fatalf("diagnostics = %v, want unsupported", diagnosticCodes(result.Diagnostics))
			}
		})
	}
}

func TestNestedMultilineValuesDoNotInvalidateTopLevelTags(t *testing.T) {
	t.Parallel()

	sources := []string{
		`\sum_{\substack{i=1\\j=2}} a_{ij}\tag{1}`,
		`x+\substack{a\\b}\tag{1}`,
	}
	for _, src := range sources {
		src := src
		t.Run(src, func(t *testing.T) {
			t.Parallel()
			result := parseTest(t, src, Options{})
			if len(result.Tags) != 1 || !result.Tags[0].Manual {
				t.Fatalf("tags = %+v, want one top-level tag", result.Tags)
			}
		})
	}
}

func TestCommandValuedEmptyTagIsDiagnosed(t *testing.T) {
	t.Parallel()

	result := parseTest(t, `\tag\relax`, Options{})
	if len(result.Tags) != 1 {
		t.Fatalf("tags = %+v, want visible invalid tag", result.Tags)
	}
	if _, ok := result.Tags[0].Body.(*ir.Placeholder); !ok {
		t.Fatalf("tag body = %T, want placeholder", result.Tags[0].Body)
	}
	if !hasDiagnostic(result.Diagnostics, source.CodeMissingArgument) {
		t.Fatalf("diagnostics = %v, want missing-argument", diagnosticCodes(result.Diagnostics))
	}
}

func TestTagLoweringSharesFormulaNodeBudget(t *testing.T) {
	t.Parallel()

	result := parseTest(t, `x\tag{1}`, Options{Limits: source.Limits{MaxNodes: 2}})
	if len(result.Tags) != 0 {
		t.Fatalf("tags = %+v, want tag omitted under shared node budget", result.Tags)
	}
	if !hasDiagnostic(result.Diagnostics, source.CodeLimitExceeded) {
		t.Fatalf("diagnostics = %v, want limit-exceeded", diagnosticCodes(result.Diagnostics))
	}
}
