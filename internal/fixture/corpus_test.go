package fixture

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestRunCorpusRunsEveryCaseAndAccountsForAll(t *testing.T) {
	t.Parallel()
	corpus := testCorpus("pass")
	var calls []string
	report, err := RunCorpus(context.Background(), corpus, RunOptions{Tolerance: TypletFrameTolerancePoints}, func(_ context.Context, c Case) (Outcome, error) {
		calls = append(calls, c.ID)
		return Outcome{Classification: Pass, Frame: ptrFrame(testFrame())}, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(calls, ",") != "pass-1,pass-2" {
		t.Fatalf("calls = %v", calls)
	}
	if report.Total != 2 || report.Pass != 2 || report.Bug != 0 {
		t.Fatalf("report = %#v", report)
	}
	if err := report.Validate(); err != nil {
		t.Fatal(err)
	}
}

func TestRunCorpusNeverSilentlySkips(t *testing.T) {
	t.Parallel()
	corpus := testCorpus("mixed")
	var calls int
	report, err := RunCorpus(context.Background(), corpus, RunOptions{}, func(_ context.Context, c Case) (Outcome, error) {
		calls++
		if c.ID == "mixed-1" {
			return Outcome{}, errors.New("parser exploded")
		}
		return Outcome{Classification: DiagnosticExpected, Detail: "missing argument"}, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if calls != 2 {
		t.Fatalf("calls = %d, want 2", calls)
	}
	if report.Bug != 1 || report.DiagnosticExpected != 1 || report.Total != 2 {
		t.Fatalf("report = %#v", report)
	}
	if !strings.Contains(report.Results[0].Detail, "parser exploded") {
		t.Fatalf("callback error missing from report: %#v", report.Results[0])
	}
}

func TestRunCorpusRequiresDeclaredKnownDifferences(t *testing.T) {
	t.Parallel()
	corpus := testCorpus("known")
	_, err := RunCorpus(context.Background(), corpus, RunOptions{}, func(_ context.Context, c Case) (Outcome, error) {
		return Outcome{Classification: KnownDifference}, nil
	})
	if err == nil || !strings.Contains(err.Error(), "undeclared known difference") {
		t.Fatalf("error = %v", err)
	}

	corpus.Spec.KnownDifferences = []KnownDifferenceEntry{
		{ID: "known-1", Reason: "upstream bug"},
		{ID: "known-2", Reason: "font fallback"},
	}
	report, err := RunCorpus(context.Background(), corpus, RunOptions{}, func(_ context.Context, c Case) (Outcome, error) {
		return Outcome{Classification: KnownDifference}, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if report.KnownDifference != 2 || !strings.Contains(report.Results[0].Detail, "upstream bug") {
		t.Fatalf("report = %#v", report)
	}
}

func TestRunCorpusTurnsFrameMismatchIntoBugAndContinues(t *testing.T) {
	t.Parallel()
	corpus := testCorpus("frames")
	report, err := RunCorpus(context.Background(), corpus, RunOptions{Tolerance: TypletFrameTolerancePoints}, func(_ context.Context, c Case) (Outcome, error) {
		frame := testFrame()
		if c.ID == "frames-1" {
			frame.Glyphs[0].X = 2
		}
		return Outcome{Classification: Pass, Frame: &frame}, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if report.Bug != 1 || report.Pass != 1 {
		t.Fatalf("report = %#v", report)
	}
	if !strings.Contains(report.Results[0].Detail, "fixture frames-1") {
		t.Fatalf("frame diff lacks formula ID: %q", report.Results[0].Detail)
	}
}

func TestRunCorpusHonorsCancellation(t *testing.T) {
	t.Parallel()
	corpus := testCorpus("cancel")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var calls int
	report, err := RunCorpus(ctx, corpus, RunOptions{}, func(_ context.Context, c Case) (Outcome, error) {
		calls++
		cancel()
		return Outcome{Classification: Pass, Frame: ptrFrame(testFrame())}, nil
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v, want context.Canceled", err)
	}
	if calls != 1 || len(report.Results) != 1 {
		t.Fatalf("calls = %d, results = %d", calls, len(report.Results))
	}
}

func TestRunCorpusRejectsMissingResult(t *testing.T) {
	t.Parallel()
	corpus := testCorpus("invalid")
	_, err := RunCorpus(context.Background(), corpus, RunOptions{}, func(_ context.Context, c Case) (Outcome, error) {
		return Outcome{}, nil
	})
	if err == nil || !strings.Contains(err.Error(), "invalid classification") {
		t.Fatalf("error = %v", err)
	}
}

func TestReportValidateConservesCounts(t *testing.T) {
	t.Parallel()
	base := Report{Corpus: "x", Total: 2, Pass: 1, Results: []Result{{ID: "a", Classification: Pass}}}
	if err := base.Validate(); err == nil {
		t.Fatal("expected count mismatch")
	}
	base.Bug = 1
	base.Results = append(base.Results, Result{ID: "b", Classification: Bug})
	if err := base.Validate(); err != nil {
		t.Fatalf("valid report rejected: %v", err)
	}
}

func testCorpus(prefix string) *Corpus {
	base := testFrame()
	base.Glyphs[0].X = 0
	return &Corpus{
		Name: prefix,
		Spec: CorpusSpec{Name: prefix, ExpectedCount: 2},
		Cases: []Case{
			{Index: 1, ID: prefix + "-1", Fixture: &Fixture{ID: prefix + "-1", Frame: &base}},
			{Index: 2, ID: prefix + "-2", Fixture: &Fixture{ID: prefix + "-2", Frame: &base}},
		},
	}
}

func ptrFrame(frame Frame) *Frame {
	return &frame
}
