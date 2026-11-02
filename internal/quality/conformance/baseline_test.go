package conformance

import (
	"context"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

func TestTeXConformanceBaseline(t *testing.T) {
	if raceEnabled {
		t.Skip("process-per-case corpus is covered by non-race CI; run package race tests without baseline workers")
	}
	packageRoot := conformancePackageRoot(t)
	report, err := BuildTeXReport(context.Background(), SuiteOptions{
		FixtureRoot: filepath.Join(packageRoot, "..", "..", "..", "testdata", "upstream"),
		PolicyPath:  filepath.Join(packageRoot, "testdata", "tex-policy.json"),
		Frontend:    testProcessFrontend(),
		CaseTimeout: 250 * time.Millisecond,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := report.Validate(); err != nil {
		t.Fatalf("actual report: %v", err)
	}
	if err := CheckCorpusSet(report); err != nil {
		t.Fatal(err)
	}
	if report.Summary.Unexplained != 0 {
		t.Fatalf("report has %d unexplained cases", report.Summary.Unexplained)
	}

	want, err := LoadReport(filepath.Join(packageRoot, "testdata", "tex-baseline.json"))
	if err != nil {
		t.Fatal(err)
	}
	if difference := CompareReports(want, report); difference != "" {
		t.Fatalf("TeX conformance baseline changed; review the behavior before explicitly regenerating with cmd/texreport: %s", difference)
	}
}

func conformancePackageRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	return filepath.Dir(file)
}
