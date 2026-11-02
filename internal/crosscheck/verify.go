package crosscheck

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/caelyreth/untie"
	"github.com/caelyreth/untie/internal/fixture"
	"github.com/caelyreth/untie/ir"
	"github.com/caelyreth/untie/source"
)

// Options configure one paired-corpus verification.
type Options struct {
	FixtureRoot      string
	KnownDifferences string
}

// LoadPairedCorpus loads the exact 81-case Typlet paired corpus and frames.
func LoadPairedCorpus(fixtureRoot string) (*fixture.Corpus, error) {
	manifestPath := filepath.Join(fixtureRoot, "manifests", "corpora.json")
	manifest, err := fixture.LoadCorpusManifest(manifestPath)
	if err != nil {
		return nil, err
	}
	for _, spec := range manifest.Corpora {
		if spec.Name == "typlet-paired" {
			return fixture.LoadCorpus(fixtureRoot, spec)
		}
	}
	return nil, errors.New("corpora manifest has no typlet-paired spec")
}

// Verify loads and verifies all 81 paired cases.
func Verify(ctx context.Context, opts Options) (Report, error) {
	results, previous, err := verifyObserved(ctx, opts)
	if err != nil {
		return Report{}, err
	}
	for i := range results {
		result := &results[i]
		if expected, ok := previous[result.ID]; ok {
			if expected.Kind != result.Kind {
				result.Detail = fmt.Sprintf("baseline expects %s: %s; observed %s: %s",
					expected.Kind, expected.Reason, result.Kind, result.Detail)
				result.Kind = Unexplained
				result.KnownReason = "Observed result differs from the reviewed baseline."
			} else {
				result.KnownReason = expected.Reason
			}
		} else if result.Kind != Pass {
			result.Kind = Unexplained
			result.KnownReason = "Observed non-passing result has no reviewed baseline entry."
		}
	}
	report := summarize(results)
	if err := report.Validate(); err != nil {
		var unexpected []string
		for _, result := range report.Cases {
			if result.Kind == Unexplained {
				unexpected = append(unexpected, result.ID+": "+result.Detail)
			}
		}
		if len(unexpected) > 0 {
			return report, fmt.Errorf("%w: %s", err, strings.Join(unexpected, "; "))
		}
		return report, err
	}
	return report, nil
}

// Refresh recomputes observed dispositions while requiring explicit reasons
// for every new non-passing kind and for every kind transition.
func Refresh(ctx context.Context, opts Options, reasons map[string]string) (Report, error) {
	results, _, err := verifyObserved(ctx, opts)
	if err != nil {
		return Report{}, err
	}
	previous, err := LoadReport(opts.KnownDifferences)
	if err != nil {
		return Report{}, err
	}
	return ReviewedRefresh(results, previous, reasons)
}

func verifyObserved(ctx context.Context, opts Options) ([]CaseResult, map[string]KnownDifference, error) {
	if ctx == nil {
		return nil, nil, errors.New("nil context")
	}
	corpus, err := LoadPairedCorpus(opts.FixtureRoot)
	if err != nil {
		return nil, nil, err
	}
	if corpus.Spec.ExpectedCount != 81 {
		return nil, nil, fmt.Errorf("paired corpus has %d cases, want 81", corpus.Spec.ExpectedCount)
	}
	known, err := loadKnownDifferences(opts.KnownDifferences)
	if err != nil {
		return nil, nil, err
	}
	fontBytes, err := os.ReadFile(filepath.Join(opts.FixtureRoot, "fonts", "NewCMMath-Book.otf"))
	if err != nil {
		return nil, nil, err
	}
	harness, err := newLayoutHarness(fontBytes)
	if err != nil {
		return nil, nil, err
	}
	results := make([]CaseResult, 0, len(corpus.Cases))
	for _, testCase := range corpus.Cases {
		if err := ctx.Err(); err != nil {
			return nil, nil, err
		}
		results = append(results, verifyCase(ctx, harness, testCase))
	}
	return results, known, nil
}

func verifyCase(ctx context.Context, harness *layoutHarness, testCase fixture.Case) CaseResult {
	result := CaseResult{ID: testCase.ID, Category: testCase.Category}
	mode := ir.Inline
	if testCase.Display {
		mode = ir.Display
	}
	size := 16.0
	if testCase.Fixture != nil && testCase.Fixture.Frame != nil {
		converted, err := fixture.FrameToDIP(*testCase.Fixture.Frame)
		if err != nil {
			result.Kind, result.Detail = FixtureMismatch, "decode frame: "+err.Error()
			return result
		}
		size = converted.FontSize
	}
	typstExpr := untie.ParseTypst(testCase.Source, mode, untie.ParseOptions{
		Preamble: testCase.Preamble, Limits: source.DefaultLimits(),
	})
	typstSide, failure := parsedSide("Typst", typstExpr)
	if failure != nil {
		result.Kind, result.Detail = failure.kind, failure.detail
		return result
	}
	typstBox, typstLayoutErr := harness.layout(ctx, typstSide.root.Root, mode, size)

	hasTeX := strings.TrimSpace(testCase.TeX) != ""
	if !hasTeX {
		switch {
		case len(typstSide.diagnostics) > 0:
			result.Kind = SemanticIRMismatch
			result.Detail = "Typst diagnostics: " + strings.Join(typstSide.diagnostics, "; ")
		case typstLayoutErr != nil:
			result.Kind = LayoutMismatch
			result.Detail = "Typst layout: " + typstLayoutErr.Error()
		default:
			result.Kind = DocumentedPairDifference
			result.Detail = "Typst-only case has no TeX source"
		}
		return result
	}

	texExpr := untie.ParseTeX(testCase.TeX, mode, untie.ParseOptions{Limits: source.DefaultLimits()})
	texSide, failure := parsedSide("TeX", texExpr)
	if failure != nil {
		result.Kind, result.Detail = failure.kind, failure.detail
		return result
	}
	texBox, texLayoutErr := harness.layout(ctx, texSide.root.Root, mode, size)

	typstSemantic, err := normalizeFormula(typstSide.root, mode)
	if err != nil {
		result.Kind, result.Detail = SemanticIRMismatch, "normalize Typst IR: "+err.Error()
		return result
	}
	texSemantic, err := normalizeFormula(texSide.root, mode)
	if err != nil {
		result.Kind, result.Detail = SemanticIRMismatch, "normalize TeX IR: "+err.Error()
		return result
	}
	semanticDifference := CompareSemantic(texSemantic, typstSemantic)
	var layoutDifference string
	if typstLayoutErr == nil && texLayoutErr == nil {
		layoutDifference = CompareOutputs(texBox, typstBox)
	}
	switch {
	case len(typstSide.diagnostics) > 0:
		result.Kind, result.Detail = SemanticIRMismatch, "Typst diagnostics: "+strings.Join(typstSide.diagnostics, "; ")
		return result
	case len(texSide.diagnostics) > 0:
		result.Kind, result.Detail = SemanticIRMismatch, "TeX diagnostics: "+strings.Join(texSide.diagnostics, "; ")
		return result
	case semanticDifference != nil:
		result.Kind, result.Detail = SemanticIRMismatch, semanticDifference.String()
		return result
	case typstLayoutErr != nil:
		result.Kind, result.Detail = LayoutMismatch, "Typst layout: "+typstLayoutErr.Error()
		return result
	case texLayoutErr != nil:
		result.Kind, result.Detail = LayoutMismatch, "TeX layout: "+texLayoutErr.Error()
		return result
	case layoutDifference != "":
		result.Kind, result.Detail = LayoutMismatch, layoutDifference
		return result
	}
	if testCase.Fixture == nil || testCase.Fixture.Frame == nil {
		result.Kind, result.Detail = FixtureMismatch, "paired case has no static frame"
		return result
	}
	if difference := CompareOutputToFixture(typstBox, *testCase.Fixture.Frame); difference != "" {
		result.Kind, result.Detail = FixtureMismatch, "Typst: "+difference
		return result
	}
	if difference := CompareOutputToFixture(texBox, *testCase.Fixture.Frame); difference != "" {
		result.Kind, result.Detail = FixtureMismatch, "TeX: "+difference
		return result
	}
	result.Kind = Pass
	return result
}

type parsed struct {
	root        *ir.Formula
	diagnostics []string
}

type sideFailure struct {
	kind   Kind
	detail string
}

func parsedSide(dialect string, expr *untie.Expr) (parsed, *sideFailure) {
	root, err := expr.RootSnapshot()
	if err != nil {
		return parsed{}, &sideFailure{kind: SemanticIRMismatch, detail: dialect + " IR validation: " + err.Error()}
	}
	if err := ir.ValidateSource(root, expr.Source(), source.DefaultLimits()); err != nil {
		return parsed{}, &sideFailure{kind: SemanticIRMismatch, detail: dialect + " source-aware IR validation: " + err.Error()}
	}
	return parsed{root: root, diagnostics: errorDiagnostics(expr.Diagnostics())}, nil
}

func errorDiagnostics(diagnostics []untie.Diagnostic) []string {
	var out []string
	for _, diagnostic := range diagnostics {
		if diagnostic.Severity != source.SeverityError {
			continue
		}
		out = append(out, fmt.Sprintf("%s: %s", diagnostic.Code, diagnostic.Message))
	}
	return out
}
