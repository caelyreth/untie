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
	if !tag.Manual || tag.Suppressed || tag.Starred || tag.Line != 0 {
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
		if len(result.Tags) != 0 {
			t.Fatalf("environment tags = %+v, want none", result.Tags)
		}
		if !hasDiagnostic(result.Diagnostics, source.CodeUnsupported) {
			t.Fatalf("diagnostics = %v, want unsupported", diagnosticCodes(result.Diagnostics))
		}
	})

	t.Run("tag around multiline root", func(t *testing.T) {
		t.Parallel()
		result := parseTest(t, `\begin{aligned}x&=1\end{aligned}\tag{2}`, Options{})
		if len(result.Tags) != 0 {
			t.Fatalf("multiline tags = %+v, want none", result.Tags)
		}
		if !hasDiagnostic(result.Diagnostics, source.CodeUnsupported) {
			t.Fatalf("diagnostics = %v, want unsupported", diagnosticCodes(result.Diagnostics))
		}
	})
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
