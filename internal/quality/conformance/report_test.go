package conformance

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/caelyreth/untie/ir"
	"github.com/caelyreth/untie/source"
)

func TestSourceFamilies(t *testing.T) {
	t.Parallel()

	families, commands, environments := sourceFamilies(`\frac{\begin{matrix}a&b\\c&d\end{matrix}}{\sqrt{x}}`)
	for _, want := range []string{
		`command:\frac`,
		`command:\begin`,
		`command:\end`,
		`command:\sqrt`,
		`environment:matrix`,
		"feature:alignment",
		"feature:group",
		"feature:row-break",
	} {
		if !slices.Contains(families, want) {
			t.Errorf("families %v do not contain %q", families, want)
		}
	}
	for _, want := range []string{`\frac`, `\begin`, `\end`, `\sqrt`} {
		if !slices.Contains(commands, want) {
			t.Errorf("commands %v do not contain %q", commands, want)
		}
	}
	if !slices.Equal(environments, []string{"matrix"}) {
		t.Errorf("environments = %v", environments)
	}
}

func TestEvaluatorClassifications(t *testing.T) {
	t.Parallel()

	validRoot := func() ir.Node {
		span := source.NewRange(0, 1)
		return ir.NewRow(span, ir.NewGlyph(span, "x"))
	}
	tests := []struct {
		name       string
		expect     ExpectationKind
		expectCode []source.Code
		frontend   FrontendFunc
		want       Status
	}{
		{
			name:   "pass",
			expect: ExpectParse,
			frontend: func(context.Context, Request) (FrontendResult, error) {
				return FrontendResult{Root: validRoot()}, nil
			},
			want: StatusPass,
		},
		{
			name:       "diagnostic",
			expect:     ExpectDiagnostic,
			expectCode: []source.Code{source.CodeMissingArgument},
			frontend: func(context.Context, Request) (FrontendResult, error) {
				return FrontendResult{Root: validRoot(), Diagnostics: []source.Diagnostic{{
					Range: source.NewRange(0, 1), Severity: source.SeverityError,
					Code: source.CodeMissingArgument, Message: "missing",
				}}}, nil
			},
			want: StatusDiagnostic,
		},
		{
			name:   "unsupported",
			expect: ExpectParse,
			frontend: func(context.Context, Request) (FrontendResult, error) {
				return FrontendResult{Root: validRoot(), Diagnostics: []source.Diagnostic{{
					Range: source.NewRange(0, 1), Severity: source.SeverityError,
					Code: source.CodeUnsupported, Message: "unsupported",
				}}}, nil
			},
			want: StatusUnsupported,
		},
		{
			name:   "unexpected diagnostic",
			expect: ExpectParse,
			frontend: func(context.Context, Request) (FrontendResult, error) {
				return FrontendResult{Root: validRoot(), Diagnostics: []source.Diagnostic{{
					Range: source.NewRange(0, 1), Severity: source.SeverityError,
					Code: source.CodeUnknownCommand, Message: "unknown",
				}}}, nil
			},
			want: StatusBug,
		},
		{
			name:   "wrong expected code",
			expect: ExpectDiagnostic,
			expectCode: []source.Code{
				source.CodeInvalidValue,
			},
			frontend: func(context.Context, Request) (FrontendResult, error) {
				return FrontendResult{Root: validRoot(), Diagnostics: []source.Diagnostic{{
					Range: source.NewRange(0, 1), Severity: source.SeverityError,
					Code: source.CodeUnknownCommand, Message: "unknown",
				}}}, nil
			},
			want: StatusBug,
		},
		{
			name:   "panic",
			expect: ExpectParse,
			frontend: func(context.Context, Request) (FrontendResult, error) {
				panic("boom")
			},
			want: StatusBug,
		},
	}
	for _, testCase := range tests {
		testCase := testCase
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			report, err := (Evaluator{
				Frontend: testCase.frontend, ExplicitLimits: ExplicitTeXLimits(),
			}).Evaluate(t.Context(), Input{
				Corpus: "test", ID: testCase.name, Index: 1, Source: "x",
				Mode: ir.Inline, Expect: testCase.expect, ExpectCode: testCase.expectCode,
			})
			if err != nil {
				t.Fatal(err)
			}
			if report.Status != testCase.want {
				t.Fatalf("status = %q, want %q; detail: %s", report.Status, testCase.want, report.Detail)
			}
		})
	}
}

func TestEvaluatorRejectsInvalidAndLimitDependentIR(t *testing.T) {
	t.Parallel()

	invalid := func(context.Context, Request) (FrontendResult, error) {
		return FrontendResult{Root: ir.NewFraction(source.NewRange(0, 1), nil, nil)}, nil
	}
	report, err := (Evaluator{Frontend: invalid, ExplicitLimits: ExplicitTeXLimits()}).Evaluate(t.Context(), Input{
		Corpus: "test", ID: "invalid", Index: 1, Source: "x", Mode: ir.Inline, Expect: ExpectParse,
	})
	if err != nil {
		t.Fatal(err)
	}
	if report.Status != StatusBug || !strings.Contains(report.Detail, "IR contract") {
		t.Fatalf("report = %#v", report)
	}

	limitDependent := func(_ context.Context, request Request) (FrontendResult, error) {
		text := "a"
		if request.Limits.MaxSourceBytes != source.DefaultLimits().MaxSourceBytes {
			text = "b"
		}
		return FrontendResult{Root: ir.NewRow(source.NewRange(0, len(request.Source)), ir.NewGlyph(source.NewRange(0, 1), text))}, nil
	}
	report, err = (Evaluator{Frontend: limitDependent, ExplicitLimits: ExplicitTeXLimits()}).Evaluate(t.Context(), Input{
		Corpus: "test", ID: "limits", Index: 1, Source: "x", Mode: ir.Inline, Expect: ExpectParse,
	})
	if err != nil {
		t.Fatal(err)
	}
	if report.Status != StatusBug || !strings.Contains(report.Detail, "IR differ") {
		t.Fatalf("report = %#v", report)
	}
}

func TestEvaluatorRetriesTransientFrontendFailure(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32
	report, err := (Evaluator{
		Frontend: func(context.Context, Request) (FrontendResult, error) {
			if calls.Add(1) == 1 {
				return FrontendResult{}, context.DeadlineExceeded
			}
			span := source.NewRange(0, 1)
			return FrontendResult{Root: ir.NewRow(span, ir.NewGlyph(span, "x"))}, nil
		},
		ExplicitLimits: ExplicitTeXLimits(),
	}).Evaluate(t.Context(), Input{
		Corpus: "test", ID: "retry", Index: 1, Source: "x", Mode: ir.Inline, Expect: ExpectParse,
	})
	if err != nil {
		t.Fatal(err)
	}
	if report.Status != StatusPass {
		t.Fatalf("status = %q; detail: %s", report.Status, report.Detail)
	}
	if calls.Load() != 4 {
		t.Fatalf("frontend calls = %d, want one failed attempt plus three successful evaluations", calls.Load())
	}
}

func TestEvaluatorRejectsWrongDiagnosticSpan(t *testing.T) {
	t.Parallel()

	span := source.NewRange(0, 1)
	report, err := (Evaluator{
		Frontend: func(context.Context, Request) (FrontendResult, error) {
			return FrontendResult{
				Root: ir.NewRow(source.NewRange(0, 3), ir.NewGlyph(source.NewRange(0, 1), "foo")),
				Diagnostics: []source.Diagnostic{{
					Range:    source.NewRange(0, 3),
					Severity: source.SeverityError,
					Code:     source.CodeUnknownVariable,
					Message:  "unknown variable",
				}},
			}, nil
		},
		ExplicitLimits: ExplicitTypstLimits(),
	}).Evaluate(t.Context(), Input{
		Corpus: "test", ID: "span", Index: 1, Source: "foo", Mode: ir.Inline,
		Dialect: DialectTypst, Expect: ExpectDiagnostic,
		ExpectCode: []source.Code{source.CodeUnknownVariable}, ExpectSpan: &span,
	})
	if err != nil {
		t.Fatal(err)
	}
	if report.Status != StatusBug || !strings.Contains(report.Detail, "source span") {
		t.Fatalf("report = %#v", report)
	}
}

func TestReportRejectsUnexplainedCases(t *testing.T) {
	t.Parallel()

	report := Report{
		SchemaVersion: reportSchemaVersion,
		Dialect:       DialectTeX,
		Corpora:       []CorpusSummary{{Name: "corpus", Count: 1}},
		Summary:       Summary{Total: 1, Bug: 1},
		Families:      []FamilySummary{{Family: "literal", Total: 1}},
		Cases: []CaseReport{{
			ID: "case", Corpus: "corpus", Index: 1, Status: StatusBug,
			Families: []string{"literal"}, Detail: "failure",
		}},
	}
	if err := report.Validate(); err == nil ||
		!strings.Contains(err.Error(), "unexplained") ||
		!strings.Contains(err.Error(), `detail "failure"`) {
		t.Fatalf("error = %v", err)
	}
}

func TestPolicyRejectsBroadExclusionAndStaleRules(t *testing.T) {
	t.Parallel()

	policy := Policy{
		SchemaVersion: policySchemaVersion,
		Exclusions: []PolicyRule{{
			Corpus: "corpus", Codes: []source.Code{source.CodeUnsupported}, Reason: "too broad",
		}},
	}
	if err := policy.Validate(); err == nil {
		t.Fatal("code-only scope exclusion was accepted")
	}

	path := filepath.Join(t.TempDir(), "policy.json")
	if err := os.WriteFile(path, []byte(`{
		"schemaVersion": 1,
		"knownGaps": [{"corpus":"corpus","ids":["case"],"reason":"known"}]
	}`), 0o644); err != nil {
		t.Fatal(err)
	}
	loaded, err := LoadPolicy(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := loaded.unused(); err == nil {
		t.Fatal("unused known gap was accepted")
	}
}

func TestLoadReportRejectsUnknownFields(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "report.json")
	if err := os.WriteFile(path, []byte(`{"schemaVersion":1,"dialect":"tex","unknown":true}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadReport(path); err == nil || !strings.Contains(err.Error(), "unknown field") {
		t.Fatalf("error = %v", err)
	}
}

func TestProcessFrontendAdapter(t *testing.T) {
	t.Parallel()

	result, err := testProcessFrontend()(t.Context(), Request{
		Source: `\frac{1}{2}`, Mode: ir.Inline, Limits: ExplicitTeXLimits(),
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Observation == nil || !result.Observation.Valid || len(result.Observation.CanonicalJSON) == 0 {
		t.Fatalf("unexpected observation: %#v", result.Observation)
	}
	if len(result.Diagnostics) != 0 {
		t.Fatalf("diagnostics: %v", result.Diagnostics)
	}
}

func TestEnsureJSONEOF(t *testing.T) {
	t.Parallel()

	valid := json.NewDecoder(strings.NewReader("{}"))
	var value any
	if err := valid.Decode(&value); err != nil {
		t.Fatal(err)
	}
	if err := ensureJSONEOF(valid); err != nil {
		t.Fatal(err)
	}
	extra := json.NewDecoder(strings.NewReader("{} {}"))
	if err := extra.Decode(&value); err != nil {
		t.Fatal(err)
	}
	if err := ensureJSONEOF(extra); err == nil {
		t.Fatal("multiple JSON values were accepted")
	}
}
