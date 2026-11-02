package conformance

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"slices"

	"github.com/caelyreth/untie/internal/fixture"
	"github.com/caelyreth/untie/internal/typst"
	"github.com/caelyreth/untie/ir"
	"github.com/caelyreth/untie/source"
)

const (
	typletPairedCorpus   = "typlet-paired"
	typletFeaturesCorpus = "typlet-features"
	typletCodeCorpus     = "typlet-code"
	typletSymbolsCorpus  = "typlet-symbols"
	typletSyntaxCorpus   = "typlet-syntax"
)

var typstCorpora = []string{
	typletPairedCorpus,
	typletFeaturesCorpus,
	typletCodeCorpus,
	typletSymbolsCorpus,
	typletSyntaxCorpus,
}

var typstCorpusCounts = map[string]int{
	typletPairedCorpus:   81,
	typletFeaturesCorpus: 29,
	typletCodeCorpus:     16,
	typletSymbolsCorpus:  30,
	typletSyntaxCorpus:   15,
}

// BuildTypstReport evaluates every imported selected Typlet Typst corpus.
func BuildTypstReport(ctx context.Context, options SuiteOptions) (*Report, error) {
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
	for _, name := range typstCorpora {
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
		explicit = ExplicitTypstLimits()
	}
	frontend := options.Frontend
	if frontend == nil {
		frontend = ParseTypst
	}
	evaluator := Evaluator{Frontend: frontend, ExplicitLimits: explicit, Timeout: options.CaseTimeout}
	report := &Report{
		Dialect: DialectTypst,
		Limits: LimitProfiles{
			Default:  limitValues(source.DefaultLimits()),
			Explicit: limitValues(explicit.WithDefaults()),
		},
	}

	for _, name := range typstCorpora {
		corpus, err := fixture.LoadCorpus(options.FixtureRoot, specs[name])
		if err != nil {
			return nil, err
		}
		report.Corpora = append(report.Corpora, CorpusSummary{Name: name, Count: len(corpus.Cases)})
		for _, testCase := range corpus.Cases {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
			input, err := typstInput(name, testCase, policy)
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

// ParseTypst adapts the Typst frontend to the dialect-neutral executor.
func ParseTypst(ctx context.Context, request Request) (FrontendResult, error) {
	if err := ctx.Err(); err != nil {
		return FrontendResult{}, err
	}
	result := typst.Parse(request.Source, request.Mode, typst.Options{
		Limits: request.Limits,
	})
	return FrontendResult{Root: result.Root, Diagnostics: result.Diagnostics}, nil
}

func typstInput(corpus string, testCase fixture.Case, policy *Policy) (Input, error) {
	mode := ir.Inline
	if testCase.Display {
		mode = ir.Display
	}
	input := Input{
		Corpus:   corpus,
		ID:       testCase.ID,
		Index:    testCase.Index,
		Source:   testCase.Source,
		Preamble: testCase.Preamble,
		Mode:     mode,
		Expect:   ExpectParse,
		Dialect:  DialectTypst,
	}
	if testCase.Category != "" {
		input.Families = append(input.Families, "category:"+testCase.Category)
	}
	if testCase.Preamble != "" {
		input.Families = append(input.Families, "feature:preamble")
	}

	for _, diagnostic := range testCase.Diagnostics {
		if diagnostic.At != "src" || diagnostic.Severity != "error" {
			continue
		}
		input.Expect = ExpectDiagnostic
		codes, ok := policy.expectedCodes(corpus, testCase.ID)
		if !ok {
			return Input{}, fmt.Errorf("typst error case %q has no reviewed diagnostic expectation", testCase.ID)
		}
		input.ExpectCode = codes
		if diagnostic.HasSpan {
			span := diagnostic.Span
			input.ExpectSpan = &span
		}
		break
	}
	return input, nil
}

// CheckTypstCorpusSet rejects partial or unexpectedly expanded Typst suites.
func CheckTypstCorpusSet(report *Report) error {
	if report == nil {
		return errors.New("nil report")
	}
	if report.Dialect != DialectTypst {
		return fmt.Errorf("dialect is %q, want %q", report.Dialect, DialectTypst)
	}
	if len(report.Corpora) != len(typstCorpora) {
		return fmt.Errorf("report has %d corpora, want %d", len(report.Corpora), len(typstCorpora))
	}
	for _, corpus := range report.Corpora {
		want, ok := typstCorpusCounts[corpus.Name]
		if !ok {
			return fmt.Errorf("unexpected corpus %q", corpus.Name)
		}
		if corpus.Count != want {
			return fmt.Errorf("%s has %d cases, want %d", corpus.Name, corpus.Count, want)
		}
	}
	return nil
}

// TypstCaseIDs returns the stable sorted case keys in a Typst report.
func TypstCaseIDs(report *Report) []string {
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
