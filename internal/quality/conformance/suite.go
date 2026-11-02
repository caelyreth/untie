package conformance

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"slices"
	"time"

	"github.com/caelyreth/untie/internal/quality/fixture"
	"github.com/caelyreth/untie/internal/tex"
	"github.com/caelyreth/untie/ir"
	"github.com/caelyreth/untie/source"
)

const (
	ratexCorpus = "ratex-core"
	katexCorpus = "katex-cases"
)

// SuiteOptions selects tracked inputs and reviewed disposition data.
type SuiteOptions struct {
	FixtureRoot    string
	PolicyPath     string
	Frontend       FrontendFunc
	ExplicitLimits source.Limits
	CaseTimeout    time.Duration
}

// BuildTeXReport evaluates the tracked RaTeX and selected KaTeX corpora.
func BuildTeXReport(ctx context.Context, options SuiteOptions) (*Report, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	manifestPath := filepath.Join(options.FixtureRoot, "manifests", "corpora.json")
	manifest, err := fixture.LoadCorpusManifest(manifestPath)
	if err != nil {
		return nil, err
	}
	specs := make(map[string]fixture.CorpusSpec, len(manifest.Corpora))
	for _, spec := range manifest.Corpora {
		specs[spec.Name] = spec
	}
	for _, name := range []string{ratexCorpus, katexCorpus} {
		if _, ok := specs[name]; !ok {
			return nil, fmt.Errorf("corpus manifest has no %q", name)
		}
	}

	var policy *Policy
	if options.PolicyPath != "" {
		policy, err = LoadPolicy(options.PolicyPath)
		if err != nil {
			return nil, err
		}
	}
	explicit := options.ExplicitLimits
	if isZeroLimits(explicit) {
		explicit = ExplicitTeXLimits()
	}
	frontend := options.Frontend
	if frontend == nil {
		frontend = ParseTeX
	}
	evaluator := Evaluator{Frontend: frontend, ExplicitLimits: explicit, Timeout: options.CaseTimeout}
	report := &Report{
		Dialect: DialectTeX,
		Limits: LimitProfiles{
			Default:  limitValues(source.DefaultLimits()),
			Explicit: limitValues(explicit.WithDefaults()),
		},
	}

	for _, name := range []string{ratexCorpus, katexCorpus} {
		corpus, err := fixture.LoadCorpus(options.FixtureRoot, specs[name])
		if err != nil {
			return nil, err
		}
		report.Corpora = append(report.Corpora, CorpusSummary{Name: name, Count: len(corpus.Cases)})
		for _, testCase := range corpus.Cases {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
			input, err := teXInput(name, testCase, policy)
			if err != nil {
				return nil, err
			}
			result, err := evaluator.Evaluate(ctx, input)
			if err != nil {
				return nil, fmt.Errorf("%s/%s: %w", name, testCase.ID, err)
			}
			if policy != nil {
				policy.apply(&result)
			}
			report.Cases = append(report.Cases, result)
		}
	}
	if policy != nil {
		if err := policy.unused(); err != nil {
			return nil, err
		}
	}
	finalizeReport(report)
	return report, nil
}

// ParseTeX adapts the TeX frontend to the dialect-neutral executor. TeX
// preambles are not part of the imported corpus and are intentionally ignored.
func ParseTeX(_ context.Context, request Request) (FrontendResult, error) {
	result := tex.Parse(request.Source, request.Mode, tex.Options{Limits: request.Limits})
	return FrontendResult{Root: result.Root, Tags: result.Tags, Diagnostics: result.Diagnostics}, nil
}

func teXInput(corpus string, testCase fixture.Case, policy *Policy) (Input, error) {
	input := Input{
		Corpus: corpus,
		ID:     testCase.ID,
		Index:  testCase.Index,
		Source: testCase.Source,
		Mode:   ir.Display,
		Expect: ExpectParse,
	}
	switch corpus {
	case ratexCorpus:
		return input, nil
	case katexCorpus:
		input.Mode = ir.Inline
		var meta fixture.KaTeXCase
		if len(testCase.Meta) == 0 {
			return Input{}, fmt.Errorf("KaTeX case %q has no metadata", testCase.ID)
		}
		if err := json.Unmarshal(testCase.Meta, &meta); err != nil {
			return Input{}, fmt.Errorf("KaTeX case %q metadata: %w", testCase.ID, err)
		}
		if meta.ID != testCase.ID || meta.Source != testCase.Source {
			return Input{}, fmt.Errorf("KaTeX case %q metadata does not match fixture record", testCase.ID)
		}
		switch meta.Kind {
		case "parse":
			input.Expect = ExpectParse
		case "error":
			input.Expect = ExpectDiagnostic
			codes, ok := policy.expectedCodes(corpus, testCase.ID)
			if !ok {
				return Input{}, fmt.Errorf("KaTeX error case %q has no reviewed diagnostic expectation", testCase.ID)
			}
			input.ExpectCode = codes
		default:
			return Input{}, fmt.Errorf("KaTeX case %q has unsupported kind %q", testCase.ID, meta.Kind)
		}
		return input, nil
	default:
		return Input{}, fmt.Errorf("unsupported TeX corpus %q", corpus)
	}
}

// TeXCaseIDs returns a stable copy of all evaluated case keys.
func TeXCaseIDs(report *Report) []string {
	if report == nil {
		return nil
	}
	keys := make([]string, len(report.Cases))
	for i, result := range report.Cases {
		keys[i] = result.Key()
	}
	slices.Sort(keys)
	return keys
}

func isZeroLimits(limits source.Limits) bool {
	return limits == (source.Limits{})
}

// CheckCorpusSet verifies that a report contains exactly the expected TeX
// corpora. It avoids treating a partial report as a successful suite.
func CheckCorpusSet(report *Report) error {
	if report == nil {
		return errors.New("nil report")
	}
	if report.Dialect != DialectTeX {
		return fmt.Errorf("dialect is %q, want %q", report.Dialect, DialectTeX)
	}
	if len(report.Corpora) != 2 {
		return fmt.Errorf("report has %d corpora, want 2", len(report.Corpora))
	}
	for _, corpus := range report.Corpora {
		switch corpus.Name {
		case ratexCorpus:
			if corpus.Count != 1075 {
				return fmt.Errorf("%s has %d cases, want 1075", corpus.Name, corpus.Count)
			}
		case katexCorpus:
			if corpus.Count != 36 {
				return fmt.Errorf("%s has %d cases, want 36", corpus.Name, corpus.Count)
			}
		default:
			return fmt.Errorf("unexpected corpus %q", corpus.Name)
		}
	}
	return nil
}
