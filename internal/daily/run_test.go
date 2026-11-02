package daily

import (
	"bytes"
	"path/filepath"
	"testing"
	"time"
)

func TestTrackedDailyCorpusPasses(t *testing.T) {
	cases, _ := loadTrackedCorpus(t)
	fontPath := filepath.Join("..", "..", "testdata", "upstream", "fonts", "NewCMMath-Book.otf")
	engine, err := LoadEngine(fontPath)
	if err != nil {
		t.Fatal(err)
	}
	runner, err := NewRunner(engine, 2*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	report, err := runner.Run(t.Context(), cases)
	if err != nil {
		t.Fatal(err)
	}
	if err := report.Validate(); err != nil {
		for _, result := range report.Cases {
			t.Logf("%s: pass=%v outcome=%q detail=%q", result.ID, result.Pass, result.Outcome, result.Detail)
		}
		t.Fatal(err)
	}
	want := Summary{
		Total: 81, Pass: 81, Rendered: 72, Recovered: 4,
		MissingGlyph: 5, LayoutError: 0, Failure: 0, KnownIssues: 1,
	}
	if report.Summary != want {
		t.Fatalf("summary = %+v, want %+v", report.Summary, want)
	}
	for _, result := range report.Cases {
		if !result.Pass {
			t.Errorf("%s failed: %s", result.ID, result.Detail)
		}
	}
	issue := findResult(report.Cases, "physics.units.typst")
	if issue == nil || issue.Issue != "typst-decimal-text-run" ||
		issue.Outcome != "missing-glyph" || !issue.Pass {
		t.Fatalf("tracked Typst decimal issue = %#v", issue)
	}
	if _, err := report.MarshalCanonical(); err != nil {
		t.Fatalf("canonical report: %v", err)
	}
	repeated, err := runner.Run(t.Context(), cases)
	if err != nil {
		t.Fatal(err)
	}
	firstJSON, err := report.MarshalCanonical()
	if err != nil {
		t.Fatal(err)
	}
	repeatedJSON, err := repeated.MarshalCanonical()
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(firstJSON, repeatedJSON) {
		t.Fatal("repeated corpus runs produced different reports")
	}
}

func TestRunnerRejectsNilInputs(t *testing.T) {
	t.Parallel()

	if _, err := NewRunner(nil, time.Second); err == nil {
		t.Fatal("NewRunner accepted a nil engine")
	}
	var runner *Runner
	if _, err := runner.Run(t.Context(), nil); err == nil {
		t.Fatal("nil runner accepted a run")
	}
}

func TestContainsAll(t *testing.T) {
	t.Parallel()

	if problem := containsAll("layout: missing glyph: \"中\"", []string{"missing glyph", `"中"`}); problem != "" {
		t.Fatal(problem)
	}
	if problem := containsAll("layout: missing glyph", []string{"different"}); problem == "" {
		t.Fatal("containsAll accepted a missing fragment")
	}
}

func findResult(results []CaseResult, id string) *CaseResult {
	for i := range results {
		if results[i].ID == id {
			return &results[i]
		}
	}
	return nil
}
