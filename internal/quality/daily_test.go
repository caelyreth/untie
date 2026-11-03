package quality_test

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/caelyreth/untie/internal/quality/daily"
)

func TestDailySTEMCorpus(t *testing.T) {
	root := repositoryRoot(t)
	corpusRoot := filepath.Join(root, "testdata", "daily-stem")
	casePath := filepath.Join(corpusRoot, "cases.jsonl")
	manifestPath := filepath.Join(corpusRoot, "manifest.json")
	cases, err := daily.LoadCaseFile(casePath)
	if err != nil {
		t.Fatal(err)
	}
	if *updateDailyManifest {
		manifest, buildErr := daily.BuildManifest(cases, filepath.Base(casePath))
		if buildErr != nil {
			t.Fatal(buildErr)
		}
		if writeErr := daily.WriteManifest(manifestPath, manifest); writeErr != nil {
			t.Fatal(writeErr)
		}
		t.Logf("updated %s", manifestPath)
	}
	manifest, err := daily.LoadManifest(manifestPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := daily.ValidateCorpus(cases, manifest); err != nil {
		t.Fatal(err)
	}

	fonts, err := daily.LoadFontSet(filepath.Join(root, "testdata", "upstream", "fonts", "NewCMMath-Book.otf"))
	if err != nil {
		t.Fatal(err)
	}
	runner, err := daily.NewRunner(fonts, 2*time.Second)
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
	want := daily.Summary{
		Total: 124, Pass: 124, KnownIssueFailures: 0, UnexpectedFailures: 0,
		Rendered: 112, Recovered: 7, MissingGlyph: 4, LayoutError: 1, Failure: 0,
	}
	if report.Summary != want {
		t.Fatalf("summary = %+v, want %+v", report.Summary, want)
	}
	for _, result := range report.Cases {
		if !result.Matched || result.Verdict != "pass" {
			t.Errorf("%s verdict=%s matched=%v detail=%s",
				result.ID, result.Verdict, result.Matched, result.Detail)
		}
	}
	if _, err := report.MarshalCanonical(); err != nil {
		t.Fatalf("canonical report: %v", err)
	}
}
