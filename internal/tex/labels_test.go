package tex

import (
	"strings"
	"testing"

	"github.com/caelyreth/untie/ir"
	"github.com/caelyreth/untie/source"
)

func TestEquationLabelsAreFormulaMetadataWithoutOutput(t *testing.T) {
	t.Parallel()

	const src = `a\label{eq:a}+\frac{x}{\label{eq:b}}`
	result := parseTest(t, src, Options{})
	if len(result.Labels) != 2 {
		t.Fatalf("labels = %+v, want two", result.Labels)
	}
	for i, name := range []string{"eq:a", "eq:b"} {
		start := strings.Index(src, `\label{`+name+`}`)
		want := source.NewRange(start, start+len(`\label{`+name+`}`))
		if result.Labels[i].Name != name || result.Labels[i].Span != want || result.Labels[i].Origin != want {
			t.Fatalf("label %d = %+v, want name %q span/origin %v", i, result.Labels[i], name, want)
		}
	}
	if text := collectURLTestText(result.Root); strings.Contains(text, "label") {
		t.Fatalf("label metadata leaked visible text %q", text)
	}

	nested := parseTest(t, `x^{\label{eq:script}}`, Options{})
	if len(nested.Labels) != 1 || nested.Labels[0].Name != "eq:script" {
		t.Fatalf("nested script labels = %+v", nested.Labels)
	}
}

func TestDuplicateEquationLabelKeepsFirstAndReportsSecond(t *testing.T) {
	t.Parallel()

	const src = `\label{eq:a}x\label{eq:a}`
	result := parseTest(t, src, Options{})
	if len(result.Labels) != 1 || result.Labels[0].Name != "eq:a" {
		t.Fatalf("labels = %+v, want first eq:a only", result.Labels)
	}
	if !hasDiagnostic(result.Diagnostics, source.CodeDuplicateLabel) {
		t.Fatalf("diagnostics = %+v, want duplicate-label", result.Diagnostics)
	}
	second := strings.LastIndex(src, `\label{eq:a}`)
	for _, diagnostic := range result.Diagnostics {
		if diagnostic.Code == source.CodeDuplicateLabel && diagnostic.Range != source.NewRange(second, len(src)) {
			t.Fatalf("duplicate diagnostic range = %v, want [%d,%d)", diagnostic.Range, second, len(src))
		}
	}
}

func TestEqrefResolvesOpaqueHostTextAndEscapedLabel(t *testing.T) {
	t.Parallel()

	const src = `\label{eq\_a}+x\eqref{eq\_a}`
	result := parseTest(t, src, Options{References: map[string]string{
		"eq_a": `3\frac{1}{2}`,
	}})
	if len(result.Labels) != 1 || result.Labels[0].Name != "eq_a" {
		t.Fatalf("labels = %+v, want escaped eq_a", result.Labels)
	}
	text := findText(result.Root)
	if text == nil {
		t.Fatal("\\eqref produced no text node")
	}
	if text.Text != `(3\frac{1}{2})` {
		t.Fatalf("\\eqref text = %q, want opaque host text in parentheses", text.Text)
	}
	if text.Font != ir.FontText || text.Variant != ir.VariantNormal || len(text.Clusters) != 0 {
		t.Fatalf("\\eqref text contract = font:%v variant:%v clusters:%+v", text.Font, text.Variant, text.Clusters)
	}
	start := strings.Index(src, `\eqref{eq\_a}`)
	if text.Span != source.NewRange(start, len(src)) || text.Origin != text.Span {
		t.Fatalf("\\eqref span/origin = %v/%v, want %v", text.Span, text.Origin, source.NewRange(start, len(src)))
	}
}

func TestEqrefUnresolvedAndMalformedEntriesRecoverVisibly(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		src        string
		references map[string]string
		limits     source.Limits
		code       source.Code
	}{
		{name: "missing entry", src: `a+\eqref{missing}+b`, code: source.CodeUnresolvedReference},
		{name: "empty value", src: `\eqref{eq:a}`, references: map[string]string{"eq:a": ""}, code: source.CodeInvalidValue},
		{name: "invalid UTF-8", src: `\eqref{eq:a}`, references: map[string]string{"eq:a": string([]byte{0xff})}, code: source.CodeInvalidValue},
		{name: "control character", src: `\eqref{eq:a}`, references: map[string]string{"eq:a": "1\n"}, code: source.CodeInvalidValue},
		{name: "text limit", src: `\eqref{eq:a}`, references: map[string]string{"eq:a": "12"}, limits: source.Limits{MaxTextBytes: 3}, code: source.CodeLimitExceeded},
		{name: "empty name", src: `\eqref{}`, code: source.CodeInvalidValue},
		{name: "missing argument", src: `\eqref`, code: source.CodeMissingArgument},
		{name: "unclosed group", src: `\eqref{eq:a`, code: source.CodeUnclosedGroup},
	}
	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			result := parseTest(t, tc.src, Options{References: tc.references, Limits: tc.limits})
			if !hasDiagnostic(result.Diagnostics, tc.code) {
				t.Fatalf("diagnostics = %+v, want %s", result.Diagnostics, tc.code)
			}
			found := false
			ir.Walk(result.Root, func(node ir.Node) bool {
				if _, ok := node.(*ir.Placeholder); ok {
					found = true
				}
				return true
			})
			if !found {
				t.Fatalf("root has no visible placeholder: %T", result.Root)
			}
		})
	}

	recovered := parseTest(t, `a+\eqref{missing}+b`, Options{})
	if got := collectURLTestText(recovered.Root); got != "a++b" {
		t.Fatalf("surrounding formula text = %q, want a++b", got)
	}
}

func TestEquationLabelsEnforceMetadataLimits(t *testing.T) {
	t.Parallel()

	nodeLimited := parseTest(t, `x\label{eq:a}`, Options{Limits: source.Limits{MaxNodes: 2}})
	if len(nodeLimited.Labels) != 0 || !hasDiagnostic(nodeLimited.Diagnostics, source.CodeLimitExceeded) {
		t.Fatalf("node-limited labels = %+v diagnostics = %+v", nodeLimited.Labels, nodeLimited.Diagnostics)
	}
	nameLimited := parseTest(t, `x\label{abcd}`, Options{Limits: source.Limits{MaxTextBytes: 3}})
	if len(nameLimited.Labels) != 0 || !hasDiagnostic(nameLimited.Diagnostics, source.CodeLimitExceeded) {
		t.Fatalf("text-limited labels = %+v diagnostics = %+v", nameLimited.Labels, nameLimited.Diagnostics)
	}
}

func TestMacroExpandedLabelAndReferenceUseInvocationOrigin(t *testing.T) {
	t.Parallel()

	const labelSrc = `\def\lab{\label{eq:a}}\lab`
	labelResult := parseTest(t, labelSrc, Options{})
	call := strings.LastIndex(labelSrc, `\lab`)
	want := source.NewRange(call, len(labelSrc))
	if len(labelResult.Labels) != 1 || labelResult.Labels[0].Span != want || labelResult.Labels[0].Origin != want {
		t.Fatalf("macro label = %+v, want invocation %v", labelResult.Labels, want)
	}

	const refSrc = `\def\refa{\eqref{eq:a}}\refa`
	refResult := parseTest(t, refSrc, Options{References: map[string]string{"eq:a": "8"}})
	text := findText(refResult.Root)
	call = strings.LastIndex(refSrc, `\refa`)
	want = source.NewRange(call, len(refSrc))
	if text == nil || text.Text != "(8)" || text.Span != want || text.Origin != want {
		t.Fatalf("macro reference = %+v, want invocation %v", text, want)
	}
}

func TestLabelEscapesDecodeToHostFacingNames(t *testing.T) {
	t.Parallel()

	tests := map[string]string{
		`eq\_a`:   "eq_a",
		`eq\%a`:   "eq%a",
		`eq\#a`:   "eq#a",
		`eq\{a\}`: "eq{a}",
		`eq\\a`:   `eq\a`,
		`eq\ a`:   "eq a",
		`eq\:a`:   `eq\:a`,
	}
	for input, want := range tests {
		if got := unescapeLabelName(input); got != want {
			t.Errorf("unescapeLabelName(%q) = %q, want %q", input, got, want)
		}
	}
}
