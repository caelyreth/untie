package daily

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"slices"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/caelyreth/untie"
	"github.com/caelyreth/untie/ir"
	"github.com/caelyreth/untie/source"
)

const (
	defaultCaseTimeout = 2 * time.Second
	cancellationGrace  = 50 * time.Millisecond
)

var (
	// ErrCaseTimeout identifies a case worker that exceeded its time budget.
	ErrCaseTimeout = errors.New("daily: case timeout")
	// ErrRunnerFatal identifies a runner whose worker could not be confirmed
	// terminated. A fatal runner must not be reused.
	ErrRunnerFatal = errors.New("daily: fatal runner state")
)

// Evaluator observes one reviewed corpus case.
type Evaluator interface {
	Evaluate(context.Context, Case) CaseResult
}

// EvaluatorFunc adapts a function to Evaluator for focused tests.
type EvaluatorFunc func(context.Context, Case) CaseResult

// Evaluate calls f.
func (f EvaluatorFunc) Evaluate(ctx context.Context, testCase Case) CaseResult {
	return f(ctx, testCase)
}

// Runner evaluates cases sequentially. A timed-out or canceled worker makes the
// Runner fatal because process-local goroutines cannot be force-killed.
type Runner struct {
	evaluator   Evaluator
	CaseTimeout time.Duration

	mu    sync.Mutex
	fatal error
}

// NewRunner prepares one public-Engine evaluator over an immutable font set.
// Every case owns a fresh Engine using that case's limits.
func NewRunner(fonts *untie.FontSet, timeout time.Duration) (*Runner, error) {
	if fonts == nil {
		return nil, errors.New("daily: nil font set")
	}
	return NewRunnerWithEvaluator(engineEvaluator{fonts: fonts}, timeout)
}

// NewRunnerWithEvaluator injects the case evaluator for containment tests.
func NewRunnerWithEvaluator(evaluator Evaluator, timeout time.Duration) (*Runner, error) {
	if evaluator == nil {
		return nil, errors.New("daily: nil evaluator")
	}
	if timeout <= 0 {
		timeout = defaultCaseTimeout
	}
	return &Runner{evaluator: evaluator, CaseTimeout: timeout}, nil
}

// CaseResult records one observed expectation result.
type CaseResult struct {
	ID                 string   `json:"id"`
	Category           string   `json:"category"`
	CoverageGroup      string   `json:"coverageGroup,omitempty"`
	ExpectedDifference string   `json:"expectedDifference,omitempty"`
	SourceSpecific     string   `json:"sourceSpecific,omitempty"`
	Dialect            string   `json:"dialect"`
	Mode               string   `json:"mode"`
	Outcome            string   `json:"outcome"`
	Verdict            string   `json:"verdict"`
	Matched            bool     `json:"matched"`
	Diagnostics        []string `json:"diagnostics,omitempty"`
	Items              int      `json:"items,omitempty"`
	Hits               int      `json:"hits,omitempty"`
	Lines              int      `json:"lines,omitempty"`
	Issue              string   `json:"issue,omitempty"`
	Detail             string   `json:"detail,omitempty"`
}

// Report is the deterministic result of one daily-corpus run.
type Report struct {
	SchemaVersion int               `json:"schemaVersion"`
	Summary       Summary           `json:"summary"`
	Categories    []CategorySummary `json:"categories"`
	Cases         []CaseResult      `json:"cases"`
}

// Summary separates clean passes, matched tracked issues, and unexpected
// failures. Only unexpected failures are permitted by default.
type Summary struct {
	Total              int `json:"total"`
	Pass               int `json:"pass"`
	KnownIssueFailures int `json:"knownIssueFailures"`
	UnexpectedFailures int `json:"unexpectedFailures"`
	Rendered           int `json:"rendered"`
	Recovered          int `json:"recovered"`
	MissingGlyph       int `json:"missingGlyph"`
	LayoutError        int `json:"layoutError"`
	Failure            int `json:"failure"`
}

// CategorySummary contains exact per-category result counts.
type CategorySummary struct {
	Category           string `json:"category"`
	Total              int    `json:"total"`
	Pass               int    `json:"pass"`
	KnownIssueFailures int    `json:"knownIssueFailures"`
	UnexpectedFailures int    `json:"unexpectedFailures"`
}

const (
	outcomeRender       = "render"
	outcomeRecovered    = "recover"
	outcomeMissingGlyph = "missing-glyph"
	outcomeLayoutError  = "layout-error"
	outcomeFailure      = "failure"

	verdictPass       = "pass"
	verdictKnownIssue = "known-issue"
	verdictFailure    = "failure"
)

// Run evaluates every case in order. A context error or ErrCaseTimeout returns
// a finalized partial report. Timeout and incomplete cancellation poison the
// runner so its Engine/evaluator cannot be reused while a worker may still run.
func (r *Runner) Run(ctx context.Context, cases []Case) (Report, error) {
	if r == nil || r.evaluator == nil {
		return Report{}, errors.New("daily: nil runner or evaluator")
	}
	if ctx == nil {
		return Report{}, errors.New("daily: nil context")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.fatal != nil {
		return Report{}, r.fatal
	}

	report := Report{SchemaVersion: SchemaVersion, Cases: make([]CaseResult, 0, len(cases))}
	for _, testCase := range cases {
		if err := ctx.Err(); err != nil {
			report.finalize()
			return report, err
		}
		result, err := r.runIsolated(ctx, testCase)
		report.Cases = append(report.Cases, result)
		if err != nil {
			report.finalize()
			return report, err
		}
	}
	report.finalize()
	return report, nil
}

func (r *Runner) runIsolated(ctx context.Context, testCase Case) (CaseResult, error) {
	caseCtx, cancel := context.WithTimeout(ctx, r.CaseTimeout)
	defer cancel()
	done := make(chan CaseResult, 1)
	go func() {
		done <- r.evaluateRecovered(caseCtx, testCase)
	}()

	select {
	case result := <-done:
		if ctxErr := ctx.Err(); ctxErr != nil {
			return result, ctxErr
		}
		if errors.Is(caseCtx.Err(), context.DeadlineExceeded) {
			return r.timeoutWorker(testCase)
		}
		return result, nil
	case <-caseCtx.Done():
		if ctxErr := ctx.Err(); ctxErr != nil {
			timer := time.NewTimer(cancellationGrace)
			defer timer.Stop()
			select {
			case result := <-done:
				return result, ctxErr
			case <-timer.C:
				return r.incompleteWorker(testCase, ctxErr)
			}
		}
		// Prefer a worker result that raced with the context edge.
		select {
		case result := <-done:
			if errors.Is(caseCtx.Err(), context.DeadlineExceeded) {
				return r.timeoutWorker(testCase)
			}
			return result, nil
		default:
		}
		return r.timeoutWorker(testCase)
	}
}

func (r *Runner) timeoutWorker(testCase Case) (CaseResult, error) {
	result := failedResult(testCase, fmt.Sprintf("case timed out after %s", r.CaseTimeout))
	timeoutErr := fmt.Errorf("%w: case %q exceeded %s", ErrCaseTimeout, testCase.ID, r.CaseTimeout)
	fatalErr := errors.Join(ErrRunnerFatal, timeoutErr)
	r.fatal = fatalErr
	return result, fatalErr
}

func (r *Runner) incompleteWorker(testCase Case, ctxErr error) (CaseResult, error) {
	result := failedResult(testCase, "case canceled before worker termination: "+ctxErr.Error())
	r.fatal = fmt.Errorf("%w: cancellation left case %q running", ErrRunnerFatal, testCase.ID)
	return result, ctxErr
}

func (r *Runner) evaluateRecovered(ctx context.Context, testCase Case) (result CaseResult) {
	defer func() {
		if recovered := recover(); recovered != nil {
			result = failedResult(testCase, fmt.Sprintf("panic: %v", recovered))
		}
	}()
	result = r.evaluator.Evaluate(ctx, testCase)
	result.ID = testCase.ID
	result.Category = testCase.Category
	result.CoverageGroup = testCase.CoverageGroup
	result.ExpectedDifference = testCase.ExpectedDifference
	result.SourceSpecific = testCase.SourceSpecific
	result.Dialect = testCase.Dialect
	result.Mode = testCase.Mode
	result.Issue = testCase.Issue
	if result.Outcome == "" {
		result.Outcome = outcomeFailure
	}
	return result
}

func failedResult(testCase Case, detail string) CaseResult {
	return CaseResult{
		ID: testCase.ID, Category: testCase.Category,
		CoverageGroup: testCase.CoverageGroup, ExpectedDifference: testCase.ExpectedDifference,
		SourceSpecific: testCase.SourceSpecific, Dialect: testCase.Dialect, Mode: testCase.Mode,
		Outcome: outcomeFailure, Matched: false, Issue: testCase.Issue, Detail: detail,
	}
}

type engineEvaluator struct {
	fonts *untie.FontSet
}

func (e engineEvaluator) Evaluate(ctx context.Context, testCase Case) CaseResult {
	result := CaseResult{Outcome: outcomeFailure}
	fail := func(detail string) CaseResult {
		result.Detail = detail
		return result
	}
	dialect, mode, err := parseMode(testCase)
	if err != nil {
		return fail(err.Error())
	}
	limits := testCase.Limits.untieLimits()
	expr := untie.Parse(testCase.Source, dialect, mode, untie.ParseOptions{Limits: limits})
	if expr == nil || expr.Key() == 0 {
		return fail("parse returned a nil or unkeyed expression")
	}
	result.Diagnostics = diagnosticCodes(expr.Diagnostics())
	if problem := checkParseExpectation(testCase.Expect, result.Diagnostics); problem != "" {
		return fail(problem)
	}
	root, err := expr.RootSnapshot()
	if err != nil {
		return fail("root snapshot: " + err.Error())
	}
	if root == nil || root.Root == nil {
		return fail("root snapshot is empty")
	}
	if testCase.Expect.RootKind != "" {
		if got := rootKindName(root.Root); got != testCase.Expect.RootKind {
			return fail(fmt.Sprintf("root kind = %q, want %q", got, testCase.Expect.RootKind))
		}
	}

	alt, altErr := expr.AltContext(ctx)
	if err := ctx.Err(); err != nil {
		return fail(err.Error())
	}
	if err := alt.Validate(); err != nil {
		return fail("alt validation: " + err.Error())
	}
	if alt.Text == "" || alt.Speech == "" {
		return fail("alt text and speech must both be nonempty")
	}
	altExpectation := testCase.Expect.Alt
	if altExpectation == "" {
		altExpectation = altOK
	}
	switch altExpectation {
	case altOK:
		if altErr != nil {
			return fail("alt error: " + altErr.Error())
		}
	case altError:
		if altErr == nil {
			return fail("alt succeeded, expected an error")
		}
		if problem := containsAll(altErr.Error(), testCase.Expect.AltErrorContains); problem != "" {
			return fail("alt error mismatch: " + problem)
		}
	default:
		return fail("invalid alt expectation")
	}
	if testCase.Expect.AltText != "" {
		if alt.Text != testCase.Expect.AltText || alt.Speech != testCase.Expect.AltSpeech {
			return fail(fmt.Sprintf("alt = %q/%q, want %q/%q",
				alt.Text, alt.Speech, testCase.Expect.AltText, testCase.Expect.AltSpeech))
		}
	}

	engine, err := untie.NewEngine(e.fonts, limits)
	if err != nil {
		return fail("new engine: " + err.Error())
	}
	style, err := testCase.Style.layoutStyle()
	if err != nil {
		return fail(err.Error())
	}
	box, layoutErr := engine.LayoutContext(ctx, expr, style)
	if err := ctx.Err(); err != nil {
		return fail(err.Error())
	}
	switch testCase.Expect.Layout {
	case layoutOK:
		if layoutErr != nil {
			return fail("layout error: " + layoutErr.Error())
		}
		if box == nil {
			return fail("layout returned a nil box")
		}
		if err := box.Validate(); err != nil {
			return fail("box validation: " + err.Error())
		}
		if err := box.Alt.Validate(); err != nil {
			return fail("box alt validation: " + err.Error())
		}
		if altExpectation == altOK && !box.Alt.Equal(alt) {
			return fail("Box.Alt differs from Expr.Alt")
		}
		if box.Alt.Text == "" || box.Alt.Speech == "" {
			return fail("Box.Alt text and speech must both be nonempty")
		}
		if len(box.Items) == 0 {
			return fail("rendered box has no items")
		}
		if len(box.Hits) == 0 {
			return fail("rendered box has no source hits")
		}
		if problem := validateHits(root.Root, testCase, box); problem != "" {
			return fail(problem)
		}
		if problem := validateTag(testCase, box); problem != "" {
			return fail(problem)
		}
		if problem := validateLines(testCase, style, box); problem != "" {
			return fail(problem)
		}
		result.Items = len(box.Items)
		result.Hits = len(box.Hits)
		result.Lines = len(box.Lines)
		result.Outcome = outcomeRender
		if testCase.Expect.Parse == parseDiagnostic {
			result.Outcome = outcomeRecovered
		}
		result.Matched = true
	case layoutMissingGlyph:
		if layoutErr != nil {
			return fail("layout error: " + layoutErr.Error())
		}
		if box == nil {
			return fail("layout returned a nil box for a recoverable missing glyph")
		}
		if err := box.Validate(); err != nil {
			return fail("box validation: " + err.Error())
		}
		var missing []untie.Diagnostic
		for _, diagnostic := range box.Diagnostics {
			if diagnostic.Code == untie.CodeMissingGlyph {
				missing = append(missing, diagnostic)
			}
		}
		if len(missing) == 0 {
			return fail("layout box has no missing-glyph diagnostic")
		}
		var messages []string
		for _, diagnostic := range missing {
			messages = append(messages, diagnostic.Message)
		}
		if problem := containsAll(strings.Join(messages, "\n"), testCase.Expect.LayoutErrorContains); problem != "" {
			return fail("layout diagnostic mismatch: " + problem)
		}
		if len(box.Items) == 0 {
			return fail("recovered box has no items")
		}
		result.Items = len(box.Items)
		result.Hits = len(box.Hits)
		result.Lines = len(box.Lines)
		result.Outcome = outcomeMissingGlyph
		result.Matched = true
	case layoutError:
		if layoutErr == nil || box != nil {
			return fail("layout succeeded, expected an error")
		}
		if problem := containsAll(layoutErr.Error(), testCase.Expect.LayoutErrorContains); problem != "" {
			return fail("layout error mismatch: " + problem)
		}
		result.Outcome = outcomeLayoutError
		result.Matched = true
	default:
		return fail("invalid layout expectation")
	}
	if result.Issue != "" {
		result.Detail = "tracked issue: " + result.Issue
	}
	return result
}

func (s *StyleSpec) layoutStyle() (untie.Style, error) {
	style := untie.Style{Size: 16, GenerateAlt: true}
	if s == nil {
		return style, nil
	}
	if s.Size > 0 {
		style.Size = s.Size
	}
	style.MaxWidth = s.MaxWidth
	switch s.Align {
	case "", alignStart:
		style.Align = untie.AlignStart
	case alignCenter:
		style.Align = untie.AlignCenter
	case alignEnd:
		style.Align = untie.AlignEnd
	default:
		return untie.Style{}, fmt.Errorf("unknown alignment %q", s.Align)
	}
	return style, nil
}

func validateHits(root ir.Node, testCase Case, box *untie.Box) string {
	for i, hit := range box.Hits {
		if hit.Source.Start < 0 || hit.Source.End > len(testCase.Source) ||
			hit.Source.Start > hit.Source.End {
			return fmt.Sprintf("hit %d range %v is outside source", i, hit.Source)
		}
		if hit.Source.Len() == 0 {
			// Fence and row boundaries are intentionally zero-width structural
			// hits. They cannot claim visible characters, but they are useful to
			// hosts navigating groups.
			if hit.Kind != untie.HitFence && hit.Kind != untie.HitRow {
				return fmt.Sprintf("hit %d has zero-length source range for visible kind %s",
					i, hitKindName(hit.Kind))
			}
			if hit.Source.Start < 0 || hit.Source.Start > len(testCase.Source) ||
				(hit.Source.Start < len(testCase.Source) && !utf8.RuneStart(testCase.Source[hit.Source.Start])) {
				return fmt.Sprintf("hit %d has invalid zero-width source boundary %v", i, hit.Source)
			}
			continue
		}
	}
	for _, leaf := range meaningfulLeaves(root) {
		if !leafHitCovered(box.Hits, leaf) {
			return fmt.Sprintf("meaningful source range %v has no permitted leaf hit coverage",
				leaf.span)
		}
	}
	for i, expected := range testCase.Expect.Hits {
		start, err := sourceOccurrence(testCase.Source, expected.Text, expected.Occurrence)
		if err != nil {
			return fmt.Sprintf("hit expectation %d: %v", i, err)
		}
		want := source.NewRange(start, start+len(expected.Text))
		found := false
		for _, hit := range box.Hits {
			if hit.Source == want && hitKindName(hit.Kind) == expected.Kind {
				found = true
				break
			}
		}
		if !found {
			return fmt.Sprintf("hit expectation %d: no %s hit for source range %v", i, expected.Kind, want)
		}
	}
	return ""
}

type meaningfulLeaf struct {
	span  source.Range
	kinds hitKindMask
}

type hitKindMask uint16

const leafHitKinds = 1<<untie.HitAtom |
	1<<untie.HitScript |
	1<<untie.HitFractionNumerator |
	1<<untie.HitFractionDenominator |
	1<<untie.HitRadical |
	1<<untie.HitTableCell

func meaningfulLeaves(root ir.Node) []meaningfulLeaf {
	var leaves []meaningfulLeaf
	ir.Walk(root, func(node ir.Node) bool {
		if _, ok := node.(*ir.Phantom); ok {
			return false
		}
		var text string
		kinds := hitKindMask(leafHitKinds)
		switch value := node.(type) {
		case *ir.Glyph:
			text = value.Text
			if delimiterClass(value.Class) {
				kinds |= 1 << untie.HitFence
			}
		case *ir.Operator:
			text = value.Text
		case *ir.Text:
			text = value.Text
		case *ir.ErrorNode:
			text = value.Text
		case *ir.SizedDelimiter:
			text = value.Text
			if delimiterClass(value.Class) {
				kinds |= 1 << untie.HitFence
			}
		default:
			return true
		}
		span := node.NodeBase().Span
		if strings.TrimSpace(text) != "" && span.IsValid() && !span.IsDetached() && span.Len() > 0 {
			leaves = append(leaves, meaningfulLeaf{span: span, kinds: kinds})
		}
		return true
	})
	return leaves
}

func leafHitCovered(hits []untie.Hit, leaf meaningfulLeaf) bool {
	for _, hit := range hits {
		if hit.Source.Len() <= 0 {
			continue
		}
		if hit.Kind == untie.HitFence &&
			(hit.Source.Start < leaf.span.Start || hit.Source.End > leaf.span.End) {
			continue
		}
		if hit.Source.Start < leaf.span.End && hit.Source.End > leaf.span.Start &&
			leaf.kinds&(1<<hit.Kind) != 0 {
			return true
		}
	}
	return false
}

func delimiterClass(class ir.MathClass) bool {
	return class == ir.ClassOpening || class == ir.ClassClosing || class == ir.ClassFence
}

func sourceOccurrence(src, text string, occurrence int) (int, error) {
	if text == "" {
		return 0, errors.New("empty search text")
	}
	offset := 0
	for i := 0; ; i++ {
		index := strings.Index(src[offset:], text)
		if index < 0 {
			return 0, fmt.Errorf("%q occurrence %d is absent", text, occurrence)
		}
		index += offset
		if i == occurrence {
			return index, nil
		}
		offset = index + len(text)
	}
}

func hitKindName(kind untie.HitKind) string {
	switch kind {
	case untie.HitAtom:
		return "atom"
	case untie.HitGroup:
		return "group"
	case untie.HitScript:
		return "script"
	case untie.HitFractionNumerator:
		return "fraction-numerator"
	case untie.HitFractionDenominator:
		return "fraction-denominator"
	case untie.HitRadical:
		return "radical"
	case untie.HitFence:
		return "fence"
	case untie.HitTableCell:
		return "table-cell"
	case untie.HitRow:
		return "row"
	case untie.HitOther:
		return "other"
	default:
		return ""
	}
}

func rootKindName(root ir.Node) string {
	switch root.(type) {
	case *ir.Row:
		return "row"
	case *ir.Placeholder:
		return "placeholder"
	case *ir.ErrorNode:
		return "error"
	default:
		return ""
	}
}

func validateLines(testCase Case, style untie.Style, box *untie.Box) string {
	if style.MaxWidth <= 0 {
		if len(box.Lines) != 0 {
			return "unconstrained box unexpectedly contains lines"
		}
		if testCase.Expect.MinLines != 0 || testCase.Expect.RequireOverflow {
			return "line expectation requires a constrained display style"
		}
		return ""
	}
	if testCase.Expect.RequireFits && box.Overflow > 0 {
		return fmt.Sprintf("constrained box overflow = %v, want a fitting frame", box.Overflow)
	}
	minLines := testCase.Expect.MinLines
	if minLines < 1 {
		minLines = 1
	}
	if len(box.Lines) < minLines {
		return fmt.Sprintf("box lines %d, want at least %d", len(box.Lines), minLines)
	}
	for i, line := range box.Lines {
		frameWidth := style.MaxWidth
		if line.Tag != nil {
			frameWidth -= line.Tag.Width
			if frameWidth < 0 {
				frameWidth = 0
			}
		}
		wantOffset := float32(0)
		if line.Width < frameWidth {
			remaining := frameWidth - line.Width
			switch style.Align {
			case untie.AlignStart:
			case untie.AlignCenter:
				wantOffset = remaining / 2
			case untie.AlignEnd:
				wantOffset = remaining
			default:
				return "unknown layout alignment"
			}
		}
		if !closeHost(line.OffsetX, wantOffset) {
			return fmt.Sprintf("line %d offset-x %v, want %v for align %v",
				i, line.OffsetX, wantOffset, style.Align)
		}
	}
	if testCase.Expect.RequireOverflow {
		if box.Overflow <= 0 {
			return "constrained box overflow is not positive"
		}
		lineOverflow := false
		for _, line := range box.Lines {
			if line.Overflow > 0 {
				lineOverflow = true
				break
			}
		}
		if !lineOverflow {
			return "no constrained line reports overflow"
		}
	}
	return ""
}

func validateTag(testCase Case, box *untie.Box) string {
	if !testCase.Expect.RequireTag {
		return ""
	}
	if box.Tag == nil {
		return "required equation tag is absent"
	}
	if !box.Tag.Manual || box.Tag.Line != untie.FormulaTagLine {
		return fmt.Sprintf("equation tag metadata = manual:%v line:%d",
			box.Tag.Manual, box.Tag.Line)
	}
	if len(box.Lines) != 0 {
		if len(box.Lines) != 1 || box.Lines[0].Tag == nil {
			return "constrained equation tag is not attached to its line"
		}
		if box.Lines[0].Tag.Line != untie.FormulaTagLine {
			return fmt.Sprintf("line tag line = %d, want %d",
				box.Lines[0].Tag.Line, untie.FormulaTagLine)
		}
	}
	return ""
}

func closeHost(left, right float32) bool {
	difference := math.Abs(float64(left - right))
	scale := math.Max(1, math.Max(math.Abs(float64(left)), math.Abs(float64(right))))
	return difference <= 1e-5*scale
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
	for i := range r.Cases {
		result := &r.Cases[i]
		switch {
		case result.Matched && result.Issue == "":
			result.Verdict = verdictPass
			r.Summary.Pass++
		case result.Matched:
			result.Verdict = verdictKnownIssue
			r.Summary.KnownIssueFailures++
			if result.Detail == "" {
				result.Detail = "tracked issue: " + result.Issue
			}
		default:
			result.Verdict = verdictFailure
			r.Summary.UnexpectedFailures++
			if result.Detail == "" {
				result.Detail = "expectation mismatch"
			}
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
		summary := categories[result.Category]
		if summary == nil {
			summary = &CategorySummary{Category: result.Category}
			categories[result.Category] = summary
		}
		summary.Total++
		switch result.Verdict {
		case verdictPass:
			summary.Pass++
		case verdictKnownIssue:
			summary.KnownIssueFailures++
		case verdictFailure:
			summary.UnexpectedFailures++
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

// Blocking reports whether default non-diagnostic execution must fail.
func (r Report) Blocking(allowKnownIssues bool) bool {
	return r.Summary.UnexpectedFailures != 0 ||
		(!allowKnownIssues && r.Summary.KnownIssueFailures != 0)
}

// KnownIssueCases returns matched tracked issues in stable case order.
func (r Report) KnownIssueCases() []CaseResult {
	return r.casesWithVerdict(verdictKnownIssue)
}

// UnexpectedFailureCases returns mismatched cases in stable case order.
func (r Report) UnexpectedFailureCases() []CaseResult {
	return r.casesWithVerdict(verdictFailure)
}

func (r Report) casesWithVerdict(verdict string) []CaseResult {
	var out []CaseResult
	for _, result := range r.Cases {
		if result.Verdict == verdict {
			out = append(out, result)
		}
	}
	return out
}

// Validate checks report conservation, verdict discipline, and stable ordering.
func (r Report) Validate() error {
	if r.SchemaVersion != SchemaVersion {
		return fmt.Errorf("daily: report schema %d, want %d", r.SchemaVersion, SchemaVersion)
	}
	if r.Summary.Total != len(r.Cases) {
		return fmt.Errorf("daily: summary total %d, actual %d", r.Summary.Total, len(r.Cases))
	}
	if r.Summary.Pass+r.Summary.KnownIssueFailures+r.Summary.UnexpectedFailures != r.Summary.Total {
		return errors.New("daily: verdict counts do not conserve total")
	}
	if r.Summary.Rendered+r.Summary.Recovered+r.Summary.MissingGlyph+
		r.Summary.LayoutError+r.Summary.Failure != r.Summary.Total {
		return errors.New("daily: outcomes do not conserve total")
	}

	pass, known, unexpected := 0, 0, 0
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
		switch result.Verdict {
		case verdictPass:
			if !result.Matched || result.Issue != "" {
				return fmt.Errorf("daily: case %q has inconsistent pass verdict", result.ID)
			}
			if result.Outcome == outcomeFailure {
				return fmt.Errorf("daily: passed case %q has a failure outcome", result.ID)
			}
			pass++
		case verdictKnownIssue:
			if !result.Matched || result.Issue == "" || result.Detail == "" {
				return fmt.Errorf("daily: case %q has inconsistent known-issue verdict", result.ID)
			}
			if result.Outcome == outcomeFailure {
				return fmt.Errorf("daily: known-issue case %q has a failure outcome", result.ID)
			}
			known++
		case verdictFailure:
			if result.Matched || result.Detail == "" {
				return fmt.Errorf("daily: case %q has inconsistent failure verdict", result.ID)
			}
			if result.Outcome != outcomeFailure {
				return fmt.Errorf("daily: failed case %q has non-failure outcome %q", result.ID, result.Outcome)
			}
			unexpected++
		default:
			return fmt.Errorf("daily: case %q has unknown verdict %q", result.ID, result.Verdict)
		}
		category := actualCategories[result.Category]
		category.Category = result.Category
		category.Total++
		switch result.Verdict {
		case verdictPass:
			category.Pass++
		case verdictKnownIssue:
			category.KnownIssueFailures++
		case verdictFailure:
			category.UnexpectedFailures++
		}
		actualCategories[result.Category] = category
	}
	if pass != r.Summary.Pass || known != r.Summary.KnownIssueFailures ||
		unexpected != r.Summary.UnexpectedFailures {
		return errors.New("daily: case verdict counts disagree with summary")
	}

	categoryTotal, categoryPass, categoryKnown, categoryUnexpected := 0, 0, 0, 0
	for i, category := range r.Categories {
		if i > 0 && r.Categories[i-1].Category >= category.Category {
			return errors.New("daily: categories are not strictly sorted")
		}
		if category.Total != category.Pass+category.KnownIssueFailures+category.UnexpectedFailures {
			return fmt.Errorf("daily: category %q does not conserve counts", category.Category)
		}
		if actual := actualCategories[category.Category]; actual != category {
			return fmt.Errorf("daily: category %q summary %+v, actual %+v", category.Category, category, actual)
		}
		delete(actualCategories, category.Category)
		categoryTotal += category.Total
		categoryPass += category.Pass
		categoryKnown += category.KnownIssueFailures
		categoryUnexpected += category.UnexpectedFailures
	}
	if len(actualCategories) != 0 {
		return fmt.Errorf("daily: %d case categories are absent from summary", len(actualCategories))
	}
	if categoryTotal != r.Summary.Total || categoryPass != r.Summary.Pass ||
		categoryKnown != r.Summary.KnownIssueFailures ||
		categoryUnexpected != r.Summary.UnexpectedFailures {
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
