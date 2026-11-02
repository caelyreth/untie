package daily

import (
	"bytes"
	"context"
	"errors"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/caelyreth/untie"
	"github.com/caelyreth/untie/ir"
	"github.com/caelyreth/untie/source"
)

func TestTrackedDailyCorpusPasses(t *testing.T) {
	cases, _ := loadTrackedCorpus(t)
	fontPath := filepath.Join("..", "..", "..", "testdata", "upstream", "fonts", "NewCMMath-Book.otf")
	fonts, err := LoadFonts(fontPath)
	if err != nil {
		t.Fatal(err)
	}
	runner, err := NewRunner(fonts, 2*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	report, err := runner.Run(t.Context(), cases)
	if err != nil {
		t.Fatal(err)
	}
	if err := report.Validate(); err != nil {
		for _, result := range report.Cases {
			t.Logf("%s: matched=%v verdict=%q outcome=%q detail=%q",
				result.ID, result.Matched, result.Verdict, result.Outcome, result.Detail)
		}
		t.Fatal(err)
	}
	want := Summary{
		Total: 92, Pass: 92, KnownIssueFailures: 0, UnexpectedFailures: 0,
		Rendered: 80, Recovered: 7, MissingGlyph: 4, LayoutError: 1, Failure: 0,
	}
	if report.Summary != want {
		t.Fatalf("summary = %+v, want %+v", report.Summary, want)
	}
	for _, result := range report.Cases {
		if result.Verdict != verdictPass || !result.Matched {
			t.Errorf("%s verdict=%s matched=%v detail=%s",
				result.ID, result.Verdict, result.Matched, result.Detail)
		}
	}
	if _, err := report.MarshalCanonical(); err != nil {
		t.Fatalf("canonical report: %v", err)
	}

	subset := selectCases(t, cases, "algebra.fraction.tex", "algebra.fraction.typst")
	first, err := runner.Run(t.Context(), subset)
	if err != nil {
		t.Fatal(err)
	}
	second, err := runner.Run(t.Context(), subset)
	if err != nil {
		t.Fatal(err)
	}
	firstJSON, err := first.MarshalCanonical()
	if err != nil {
		t.Fatal(err)
	}
	secondJSON, err := second.MarshalCanonical()
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(firstJSON, secondJSON) {
		t.Fatal("repeated subset runs produced different reports")
	}
}

func TestKnownIssuesFailClosed(t *testing.T) {
	t.Parallel()

	testCase := Case{
		ID: "synthetic.issue", Category: "test", Dialect: "tex", Mode: "inline",
		Issue: "tracked-test-issue",
	}
	runner, err := NewRunnerWithEvaluator(EvaluatorFunc(func(context.Context, Case) CaseResult {
		return CaseResult{Outcome: outcomeMissingGlyph, Matched: true}
	}), time.Second)
	if err != nil {
		t.Fatal(err)
	}
	report, err := runner.Run(t.Context(), []Case{testCase})
	if err != nil {
		t.Fatal(err)
	}
	if report.Summary.Pass != 0 || report.Summary.KnownIssueFailures != 1 ||
		report.Summary.UnexpectedFailures != 0 {
		t.Fatalf("summary = %+v", report.Summary)
	}
	if !report.Blocking(false) {
		t.Fatal("known issue did not block default execution")
	}
	if report.Blocking(true) {
		t.Fatal("known issue blocked explicit diagnostic allowance")
	}
	if known := report.KnownIssueCases(); len(known) != 1 || known[0].Detail == "" {
		t.Fatalf("known issue details = %+v", known)
	}
}

func TestPanicIsContainedPerCase(t *testing.T) {
	t.Parallel()

	cases := []Case{
		{ID: "panic.one", Category: "test", Dialect: "tex", Mode: "inline"},
		{ID: "panic.two", Category: "test", Dialect: "tex", Mode: "inline"},
	}
	runner, err := NewRunnerWithEvaluator(EvaluatorFunc(func(_ context.Context, testCase Case) CaseResult {
		if testCase.ID == "panic.one" {
			panic("boom")
		}
		return CaseResult{Outcome: outcomeRender, Matched: true}
	}), time.Second)
	if err != nil {
		t.Fatal(err)
	}
	report, err := runner.Run(t.Context(), cases)
	if err != nil {
		t.Fatal(err)
	}
	if report.Summary.Pass != 1 || report.Summary.UnexpectedFailures != 1 {
		t.Fatalf("summary = %+v", report.Summary)
	}
	failures := report.UnexpectedFailureCases()
	if len(failures) != 1 || !strings.Contains(failures[0].Detail, "panic: boom") {
		t.Fatalf("panic failures = %+v", failures)
	}
}

func TestTimeoutPoisonsRunner(t *testing.T) {
	t.Parallel()

	release := make(chan struct{})
	defer close(release)
	runner, err := NewRunnerWithEvaluator(EvaluatorFunc(func(context.Context, Case) CaseResult {
		<-release
		return CaseResult{}
	}), 10*time.Millisecond)
	if err != nil {
		t.Fatal(err)
	}
	testCase := Case{ID: "synthetic.timeout", Category: "test", Dialect: "tex", Mode: "inline"}
	report, err := runner.Run(t.Context(), []Case{testCase})
	if !errors.Is(err, ErrCaseTimeout) || !errors.Is(err, ErrRunnerFatal) {
		t.Fatalf("timeout error = %v", err)
	}
	if report.Summary.UnexpectedFailures != 1 {
		t.Fatalf("timeout summary = %+v", report.Summary)
	}
	if _, err := runner.Run(t.Context(), nil); !errors.Is(err, ErrRunnerFatal) {
		t.Fatalf("second Run() error = %v, want fatal runner", err)
	}
}

func TestCooperativeDeadlinePoisonsRunner(t *testing.T) {
	t.Parallel()

	runner, err := NewRunnerWithEvaluator(EvaluatorFunc(func(ctx context.Context, _ Case) CaseResult {
		<-ctx.Done()
		return CaseResult{}
	}), 10*time.Millisecond)
	if err != nil {
		t.Fatal(err)
	}
	report, err := runner.Run(t.Context(), []Case{{
		ID: "synthetic.cooperative-timeout", Category: "test", Dialect: "tex", Mode: "inline",
	}})
	if !errors.Is(err, ErrCaseTimeout) || !errors.Is(err, ErrRunnerFatal) {
		t.Fatalf("cooperative timeout error = %v", err)
	}
	if report.Summary.UnexpectedFailures != 1 {
		t.Fatalf("cooperative timeout summary = %+v", report.Summary)
	}
}

func TestCancellationReturnsContextAndPoisonsRunner(t *testing.T) {
	t.Parallel()

	release := make(chan struct{})
	defer close(release)
	started := make(chan struct{})
	runner, err := NewRunnerWithEvaluator(EvaluatorFunc(func(context.Context, Case) CaseResult {
		close(started)
		<-release
		return CaseResult{}
	}), time.Second)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	result := make(chan error, 1)
	go func() {
		_, runErr := runner.Run(ctx, []Case{{ID: "synthetic.cancel", Category: "test", Dialect: "tex", Mode: "inline"}})
		result <- runErr
	}()
	<-started
	cancel()
	if err := <-result; !errors.Is(err, context.Canceled) {
		t.Fatalf("Run() error = %v, want context.Canceled", err)
	}
	if _, err := runner.Run(t.Context(), nil); !errors.Is(err, ErrRunnerFatal) {
		t.Fatalf("runner after cancellation error = %v, want fatal state", err)
	}
}

func TestCooperativeCancellationDoesNotPoisonRunner(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32
	started := make(chan struct{})
	runner, err := NewRunnerWithEvaluator(EvaluatorFunc(func(ctx context.Context, _ Case) CaseResult {
		if calls.Add(1) == 1 {
			close(started)
			<-ctx.Done()
			return CaseResult{}
		}
		return CaseResult{Outcome: outcomeRender, Matched: true}
	}), time.Second)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	type outcome struct {
		report Report
		err    error
	}
	done := make(chan outcome, 1)
	go func() {
		report, runErr := runner.Run(ctx, []Case{{
			ID: "synthetic.cooperative-cancel", Category: "test", Dialect: "tex", Mode: "inline",
		}})
		done <- outcome{report: report, err: runErr}
	}()
	<-started
	cancel()
	first := <-done
	if !errors.Is(first.err, context.Canceled) {
		t.Fatalf("canceled Run() error = %v", first.err)
	}
	if err := first.report.Validate(); err != nil {
		t.Fatalf("canceled report: %v", err)
	}
	if _, err := runner.Run(t.Context(), []Case{{
		ID: "synthetic.after-cancel", Category: "test", Dialect: "tex", Mode: "inline",
	}}); err != nil {
		t.Fatalf("runner was poisoned after a confirmed worker return: %v", err)
	}
}

func TestValidateHitsRejectsVisibleZeroRanges(t *testing.T) {
	t.Parallel()

	root := ir.NewGlyph(source.NewRange(0, 1), "x")
	box := &untie.Box{Hits: []untie.Hit{{
		Source: source.NewRange(1, 1), Kind: untie.HitAtom,
	}}}
	problem := validateHits(root, Case{Source: "x"}, box)
	if !strings.Contains(problem, "zero-length source range") {
		t.Fatalf("validateHits() = %q", problem)
	}
}

func TestAncestorRowHitDoesNotCoverMeaningfulLeaves(t *testing.T) {
	t.Parallel()

	x := ir.NewGlyph(source.NewRange(0, 1), "x")
	y := ir.NewGlyph(source.NewRange(1, 2), "y")
	root := ir.NewRow(source.NewRange(0, 2), x, y)
	box := &untie.Box{Hits: []untie.Hit{{
		Source: source.NewRange(0, 2), Kind: untie.HitRow,
	}}}
	problem := validateHits(root, Case{Source: "xy"}, box)
	if !strings.Contains(problem, "no permitted leaf hit coverage") {
		t.Fatalf("validateHits() = %q", problem)
	}
}

func TestAncestorFenceHitDoesNotCoverDelimiterLeaf(t *testing.T) {
	t.Parallel()

	open := ir.NewGlyph(source.NewRange(0, 1), "(")
	open.Class = ir.ClassOpening
	root := ir.NewRow(source.NewRange(0, 2), open)
	box := &untie.Box{Hits: []untie.Hit{{
		Source: source.NewRange(0, 2), Kind: untie.HitFence,
	}}}
	problem := validateHits(root, Case{Source: "(x"}, box)
	if !strings.Contains(problem, "no permitted leaf hit coverage") {
		t.Fatalf("validateHits() = %q", problem)
	}
}

func selectCases(t testing.TB, cases []Case, ids ...string) []Case {
	t.Helper()
	selected := make([]Case, 0, len(ids))
	for _, id := range ids {
		index := -1
		for i := range cases {
			if cases[i].ID == id {
				index = i
				break
			}
		}
		if index < 0 {
			t.Fatalf("case %q not found", id)
		}
		selected = append(selected, cases[index])
	}
	return selected
}
