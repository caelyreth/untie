package daily

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/caelyreth/untie"
)

const defaultCaseTimeout = 2 * time.Second

// Runner evaluates cases with one reusable public Engine.
type Runner struct {
	Engine      *untie.Engine
	CaseTimeout time.Duration
}

// NewRunner returns a corpus runner. A non-positive timeout uses two seconds.
func NewRunner(engine *untie.Engine, timeout time.Duration) (*Runner, error) {
	if engine == nil {
		return nil, errors.New("daily: nil engine")
	}
	if timeout <= 0 {
		timeout = defaultCaseTimeout
	}
	return &Runner{Engine: engine, CaseTimeout: timeout}, nil
}

// CaseResult records one observed expectation result.
type CaseResult struct {
	ID              string   `json:"id"`
	Category        string   `json:"category"`
	EquivalentGroup string   `json:"equivalentGroup,omitempty"`
	SourceSpecific  string   `json:"sourceSpecific,omitempty"`
	Dialect         string   `json:"dialect"`
	Mode            string   `json:"mode"`
	Outcome         string   `json:"outcome"`
	Pass            bool     `json:"pass"`
	Diagnostics     []string `json:"diagnostics,omitempty"`
	Items           int      `json:"items,omitempty"`
	Hits            int      `json:"hits,omitempty"`
	Lines           int      `json:"lines,omitempty"`
	Issue           string   `json:"issue,omitempty"`
	Detail          string   `json:"detail,omitempty"`
}

// Report is the deterministic result of one daily-corpus run.
type Report struct {
	SchemaVersion int               `json:"schemaVersion"`
	Summary       Summary           `json:"summary"`
	Categories    []CategorySummary `json:"categories"`
	Cases         []CaseResult      `json:"cases"`
}

// Summary contains exact result counts.
type Summary struct {
	Total        int `json:"total"`
	Pass         int `json:"pass"`
	Fail         int `json:"fail"`
	Rendered     int `json:"rendered"`
	Recovered    int `json:"recovered"`
	MissingGlyph int `json:"missingGlyph"`
	LayoutError  int `json:"layoutError"`
	Failure      int `json:"failure"`
	KnownIssues  int `json:"knownIssues"`
}

// CategorySummary contains exact per-category counts.
type CategorySummary struct {
	Category string `json:"category"`
	Total    int    `json:"total"`
	Pass     int    `json:"pass"`
	Fail     int    `json:"fail"`
}

const (
	outcomeRender       = "render"
	outcomeRecovered    = "recover"
	outcomeMissingGlyph = "missing-glyph"
	outcomeLayoutError  = "layout-error"
	outcomeFailure      = "failure"
)

// Run evaluates every case. It stops early only when ctx is canceled.
func (r *Runner) Run(ctx context.Context, cases []Case) (Report, error) {
	if r == nil || r.Engine == nil {
		return Report{}, errors.New("daily: nil runner or engine")
	}
	if ctx == nil {
		return Report{}, errors.New("daily: nil context")
	}
	timeout := r.CaseTimeout
	if timeout <= 0 {
		timeout = defaultCaseTimeout
	}
	report := Report{SchemaVersion: SchemaVersion, Cases: make([]CaseResult, 0, len(cases))}
	for _, testCase := range cases {
		if err := ctx.Err(); err != nil {
			return Report{}, err
		}
		result := r.runBounded(ctx, testCase, timeout)
		report.Cases = append(report.Cases, result)
	}
	report.finalize()
	return report, nil
}

func (r *Runner) runBounded(ctx context.Context, testCase Case, timeout time.Duration) CaseResult {
	ch := make(chan CaseResult, 1)
	caseCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	go func() {
		ch <- r.runRecovered(caseCtx, testCase)
	}()
	select {
	case result := <-ch:
		return result
	case <-caseCtx.Done():
		detail := "case timed out after " + timeout.String()
		if err := ctx.Err(); err != nil {
			detail = "corpus canceled: " + err.Error()
		}
		return CaseResult{
			ID: testCase.ID, Category: testCase.Category,
			EquivalentGroup: testCase.EquivalentGroup, SourceSpecific: testCase.SourceSpecific,
			Dialect: testCase.Dialect, Mode: testCase.Mode, Issue: testCase.Issue,
			Outcome: outcomeFailure, Pass: false, Detail: detail,
		}
	}
}

func (r *Runner) runRecovered(ctx context.Context, testCase Case) (result CaseResult) {
	result = CaseResult{
		ID: testCase.ID, Category: testCase.Category,
		EquivalentGroup: testCase.EquivalentGroup, SourceSpecific: testCase.SourceSpecific,
		Dialect: testCase.Dialect, Mode: testCase.Mode, Outcome: outcomeFailure, Issue: testCase.Issue,
	}
	defer func() {
		if recovered := recover(); recovered != nil {
			result.Pass = false
			result.Detail = fmt.Sprintf("panic: %v", recovered)
		}
	}()

	dialect, mode, err := parseMode(testCase)
	if err != nil {
		result.Detail = err.Error()
		return result
	}
	expr := untie.Parse(testCase.Source, dialect, mode, untie.ParseOptions{
		Limits: testCase.Limits.untieLimits(),
	})
	if expr == nil || expr.Key() == 0 {
		result.Detail = "parse returned a nil or unkeyed expression"
		return result
	}
	result.Diagnostics = diagnosticCodes(expr.Diagnostics())
	if problem := checkParseExpectation(testCase.Expect, result.Diagnostics); problem != "" {
		result.Detail = problem
		return result
	}
	root, err := expr.RootSnapshot()
	if err != nil {
		result.Detail = "root snapshot: " + err.Error()
		return result
	}
	if root == nil || root.Root == nil {
		result.Detail = "root snapshot is empty"
		return result
	}

	alt, altErr := expr.AltContext(ctx)
	if err := alt.Validate(); err != nil {
		result.Detail = "alt validation: " + err.Error()
		return result
	}
	altExpectation := testCase.Expect.Alt
	if altExpectation == "" {
		altExpectation = altOK
	}
	switch altExpectation {
	case altOK:
		if altErr != nil {
			result.Detail = "alt error: " + altErr.Error()
			return result
		}
	case altError:
		if altErr == nil {
			result.Detail = "alt succeeded, expected an error"
			return result
		}
		if problem := containsAll(altErr.Error(), testCase.Expect.AltErrorContains); problem != "" {
			result.Detail = "alt error mismatch: " + problem
			return result
		}
	}

	style := untie.Style{Size: 16}
	if testCase.Style != nil {
		if testCase.Style.Size > 0 {
			style.Size = testCase.Style.Size
		}
		style.MaxWidth = testCase.Style.MaxWidth
	}
	box, layoutErr := r.Engine.LayoutContext(ctx, expr, style)
	switch testCase.Expect.Layout {
	case layoutOK:
		if layoutErr != nil {
			result.Detail = "layout error: " + layoutErr.Error()
			return result
		}
		if box == nil {
			result.Detail = "layout returned a nil box"
			return result
		}
		if err := box.Validate(); err != nil {
			result.Detail = "box validation: " + err.Error()
			return result
		}
		if err := box.Alt.Validate(); err != nil {
			result.Detail = "box alt validation: " + err.Error()
			return result
		}
		if altExpectation == altOK && box.Alt != alt {
			result.Detail = "Box.Alt differs from Expr.Alt"
			return result
		}
		if len(box.Items) == 0 {
			result.Detail = "rendered box has no items"
			return result
		}
		if len(box.Hits) == 0 {
			result.Detail = "rendered box has no source hits"
			return result
		}
		for i, hit := range box.Hits {
			if hit.Source.Start < 0 || hit.Source.End > len(testCase.Source) ||
				hit.Source.Start > hit.Source.End {
				result.Detail = fmt.Sprintf("hit %d range %v is outside source", i, hit.Source)
				return result
			}
		}
		if mode == untie.Inline && len(box.Lines) != 0 {
			result.Detail = "inline box unexpectedly contains lines"
			return result
		}
		if style.MaxWidth > 0 {
			minLines := testCase.Expect.MinLines
			if minLines < 1 {
				minLines = 1
			}
			if len(box.Lines) < minLines {
				result.Detail = fmt.Sprintf("box lines %d, want at least %d", len(box.Lines), minLines)
				return result
			}
		} else if len(box.Lines) != 0 {
			result.Detail = "unconstrained box unexpectedly contains lines"
			return result
		}
		result.Items = len(box.Items)
		result.Hits = len(box.Hits)
		result.Lines = len(box.Lines)
		result.Outcome = outcomeRender
		if testCase.Expect.Parse == parseDiagnostic {
			result.Outcome = outcomeRecovered
		}
		result.Pass = true
	case layoutMissingGlyph, layoutError:
		if layoutErr == nil || box != nil {
			result.Detail = "layout succeeded, expected an error"
			return result
		}
		if testCase.Expect.Layout == layoutMissingGlyph && !strings.Contains(layoutErr.Error(), "missing glyph") {
			result.Detail = "layout error is not a missing-glyph error: " + layoutErr.Error()
			return result
		}
		if problem := containsAll(layoutErr.Error(), testCase.Expect.LayoutErrorContains); problem != "" {
			result.Detail = "layout error mismatch: " + problem
			return result
		}
		result.Outcome = outcomeLayoutError
		if testCase.Expect.Layout == layoutMissingGlyph {
			result.Outcome = outcomeMissingGlyph
		}
		result.Pass = true
	}
	return result
}

func parseMode(testCase Case) (untie.Dialect, untie.Mode, error) {
	var dialect untie.Dialect
	switch testCase.Dialect {
	case "tex":
		dialect = untie.TeX
	case "typst":
		dialect = untie.Typst
	default:
		return 0, 0, fmt.Errorf("unknown dialect %q", testCase.Dialect)
	}
	var mode untie.Mode
	switch testCase.Mode {
	case "inline":
		mode = untie.Inline
	case "display":
		mode = untie.Display
	default:
		return 0, 0, fmt.Errorf("unknown mode %q", testCase.Mode)
	}
	return dialect, mode, nil
}

func diagnosticCodes(diagnostics []untie.Diagnostic) []string {
	var codes []string
	for _, diagnostic := range diagnostics {
		codes = append(codes, string(diagnostic.Code))
	}
	slices.Sort(codes)
	return codes
}

func checkParseExpectation(expectation Expectation, actual []string) string {
	switch expectation.Parse {
	case parseOK:
		if len(actual) != 0 {
			return "parse diagnostics: " + strings.Join(actual, ", ")
		}
	case parseDiagnostic:
		want := slices.Clone(expectation.DiagnosticCodes)
		slices.Sort(want)
		if !slices.Equal(actual, want) {
			return fmt.Sprintf("parse diagnostics %v, want %v", actual, want)
		}
	default:
		return "invalid parse expectation"
	}
	return ""
}

func containsAll(text string, fragments []string) string {
	for _, fragment := range fragments {
		if fragment == "" {
			return "empty required fragment"
		}
		if !strings.Contains(text, fragment) {
			return fmt.Sprintf("%q does not contain %q", text, fragment)
		}
	}
	return ""
}

func (r *Report) finalize() {
	r.Summary = Summary{Total: len(r.Cases)}
	categories := make(map[string]*CategorySummary)
	for _, result := range r.Cases {
		if result.Pass {
			r.Summary.Pass++
		} else {
			r.Summary.Fail++
		}
		switch result.Outcome {
		case outcomeRender:
			r.Summary.Rendered++
		case outcomeRecovered:
			r.Summary.Recovered++
		case outcomeMissingGlyph:
			r.Summary.MissingGlyph++
		case outcomeLayoutError:
			r.Summary.LayoutError++
		case outcomeFailure:
			r.Summary.Failure++
		}
		if result.Issue != "" {
			r.Summary.KnownIssues++
		}
		summary := categories[result.Category]
		if summary == nil {
			summary = &CategorySummary{Category: result.Category}
			categories[result.Category] = summary
		}
		summary.Total++
		if result.Pass {
			summary.Pass++
		} else {
			summary.Fail++
		}
	}
	r.Categories = r.Categories[:0]
	for _, summary := range categories {
		r.Categories = append(r.Categories, *summary)
	}
	sort.Slice(r.Categories, func(i, j int) bool {
		return r.Categories[i].Category < r.Categories[j].Category
	})
	sort.Slice(r.Cases, func(i, j int) bool {
		return r.Cases[i].ID < r.Cases[j].ID
	})
}

// Validate checks report conservation and stable ordering.
func (r Report) Validate() error {
	if r.SchemaVersion != SchemaVersion {
		return fmt.Errorf("daily: report schema %d, want %d", r.SchemaVersion, SchemaVersion)
	}
	if r.Summary.Total != len(r.Cases) {
		return fmt.Errorf("daily: summary total %d, actual %d", r.Summary.Total, len(r.Cases))
	}
	if r.Summary.Pass+r.Summary.Fail != r.Summary.Total {
		return errors.New("daily: pass/fail counts do not conserve total")
	}
	if r.Summary.Rendered+r.Summary.Recovered+r.Summary.MissingGlyph+r.Summary.LayoutError+r.Summary.Failure != r.Summary.Total {
		return errors.New("daily: outcomes do not conserve total")
	}
	pass, fail, known := 0, 0, 0
	actualCategories := make(map[string]CategorySummary)
	for i, result := range r.Cases {
		if i > 0 && r.Cases[i-1].ID >= result.ID {
			return errors.New("daily: report cases are not strictly sorted")
		}
		switch result.Outcome {
		case outcomeRender, outcomeRecovered, outcomeMissingGlyph, outcomeLayoutError, outcomeFailure:
		default:
			return fmt.Errorf("daily: case %q has unknown outcome %q", result.ID, result.Outcome)
		}
		if result.Pass {
			pass++
		} else {
			fail++
			if result.Detail == "" {
				return fmt.Errorf("daily: failed case %q has no detail", result.ID)
			}
		}
		if result.Issue != "" {
			known++
		}
		category := actualCategories[result.Category]
		category.Category = result.Category
		category.Total++
		if result.Pass {
			category.Pass++
		} else {
			category.Fail++
		}
		actualCategories[result.Category] = category
	}
	if pass != r.Summary.Pass || fail != r.Summary.Fail || known != r.Summary.KnownIssues {
		return errors.New("daily: case counts disagree with summary")
	}
	categoryTotal, categoryPass, categoryFail := 0, 0, 0
	for i, category := range r.Categories {
		if i > 0 && r.Categories[i-1].Category >= category.Category {
			return errors.New("daily: categories are not strictly sorted")
		}
		if category.Total != category.Pass+category.Fail {
			return fmt.Errorf("daily: category %q does not conserve counts", category.Category)
		}
		if actual := actualCategories[category.Category]; actual != category {
			return fmt.Errorf("daily: category %q summary %+v, actual %+v", category.Category, category, actual)
		}
		delete(actualCategories, category.Category)
		categoryTotal += category.Total
		categoryPass += category.Pass
		categoryFail += category.Fail
	}
	if len(actualCategories) != 0 {
		return fmt.Errorf("daily: %d case categories are absent from summary", len(actualCategories))
	}
	if categoryTotal != r.Summary.Total || categoryPass != r.Summary.Pass || categoryFail != r.Summary.Fail {
		return errors.New("daily: category totals disagree with summary")
	}
	return nil
}

// MarshalCanonical returns deterministic report JSON after validation.
func (r Report) MarshalCanonical() ([]byte, error) {
	if err := r.Validate(); err != nil {
		return nil, err
	}
	return json.Marshal(r)
}
