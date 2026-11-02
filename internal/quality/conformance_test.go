package quality_test

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/caelyreth/untie/internal/quality/conformance"
)

func TestTeXConformance(t *testing.T) {
	if raceEnabled {
		t.Skip("process-per-case corpus is covered by non-race CI")
	}
	root := repositoryRoot(t)
	baseline := filepath.Join(root, "testdata", "conformance", "tex-baseline.json")
	report, err := conformance.BuildTeXReport(context.Background(), conformance.SuiteOptions{
		FixtureRoot: filepath.Join(root, "testdata", "upstream"),
		PolicyPath:  filepath.Join(root, "testdata", "conformance", "tex-policy.json"),
		Frontend:    qualityProcessFrontend("tex"),
		CaseTimeout: 250 * time.Millisecond,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := report.Validate(); err != nil {
		t.Fatalf("actual report: %v", err)
	}
	if err := conformance.CheckCorpusSet(report); err != nil {
		t.Fatal(err)
	}
	if report.Summary.Unexplained != 0 {
		t.Fatalf("report has %d unexplained cases", report.Summary.Unexplained)
	}
	if *updateTeXBaseline {
		data, err := report.CanonicalJSON()
		if err != nil {
			t.Fatal(err)
		}
		if err := writeReviewedFile(baseline, data); err != nil {
			t.Fatal(err)
		}
		t.Logf("updated %s", baseline)
		return
	}
	want, err := conformance.LoadReport(baseline)
	if err != nil {
		t.Fatal(err)
	}
	if difference := conformance.CompareReports(want, report); difference != "" {
		t.Fatalf("TeX conformance baseline changed: %s", difference)
	}
}

func TestTypstConformance(t *testing.T) {
	if raceEnabled {
		t.Skip("process-per-case corpus is covered by non-race CI")
	}
	root := repositoryRoot(t)
	baseline := filepath.Join(root, "testdata", "conformance", "typst-baseline.json")
	report, err := conformance.BuildTypstReport(context.Background(), conformance.SuiteOptions{
		FixtureRoot: filepath.Join(root, "testdata", "upstream"),
		PolicyPath:  filepath.Join(root, "testdata", "conformance", "typst-policy.json"),
		Frontend:    qualityProcessFrontend("typst"),
		CaseTimeout: 5 * time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := report.Validate(); err != nil {
		t.Fatalf("actual report: %v", err)
	}
	if err := conformance.CheckTypstCorpusSet(report); err != nil {
		t.Fatal(err)
	}
	if report.Summary.Unexplained != 0 {
		t.Fatalf("report has %d unexplained cases", report.Summary.Unexplained)
	}
	if report.Summary.Total != 171 {
		t.Fatalf("total = %d, want 171", report.Summary.Total)
	}
	if *updateTypstBaseline {
		data, err := report.CanonicalJSON()
		if err != nil {
			t.Fatal(err)
		}
		if err := writeReviewedFile(baseline, data); err != nil {
			t.Fatal(err)
		}
		t.Logf("updated %s", baseline)
		return
	}
	want, err := conformance.LoadReport(baseline)
	if err != nil {
		t.Fatal(err)
	}
	if difference := conformance.CompareReports(want, report); difference != "" {
		t.Fatalf("Typst conformance baseline changed: %s", difference)
	}
}
