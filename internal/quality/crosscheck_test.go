package quality_test

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/caelyreth/untie/internal/quality/crosscheck"
)

func TestPairedCrosscheck(t *testing.T) {
	root := repositoryRoot(t)
	fixtureRoot := filepath.Join(root, "testdata", "upstream")
	baseline := filepath.Join(root, "testdata", "crosscheck", "paired-baseline.json")
	options := crosscheck.Options{
		FixtureRoot:      fixtureRoot,
		KnownDifferences: baseline,
	}

	var report crosscheck.Report
	var err error
	if *updateCrosscheckBaseline {
		reasons := map[string]string{}
		if *crosscheckReasonsPath != "" {
			data, readErr := os.ReadFile(*crosscheckReasonsPath)
			if readErr != nil {
				t.Fatal(readErr)
			}
			decoder := json.NewDecoder(bytes.NewReader(data))
			decoder.DisallowUnknownFields()
			if decodeErr := decoder.Decode(&reasons); decodeErr != nil {
				t.Fatal(decodeErr)
			}
		}
		report, err = crosscheck.Refresh(context.Background(), options, reasons)
		if err != nil {
			t.Fatal(err)
		}
		data, marshalErr := report.MarshalCanonical()
		if marshalErr != nil {
			t.Fatal(marshalErr)
		}
		if writeErr := writeReviewedFile(baseline, data); writeErr != nil {
			t.Fatal(writeErr)
		}
		t.Logf("updated %s", baseline)
	} else {
		report, err = crosscheck.Verify(context.Background(), options)
		if err != nil {
			t.Fatal(err)
		}
		want, loadErr := crosscheck.LoadReport(baseline)
		if loadErr != nil {
			t.Fatal(loadErr)
		}
		if compareErr := crosscheck.CompareReports(want, report); compareErr != nil {
			t.Fatal(compareErr)
		}
	}
	if report.Unexplained != 0 {
		t.Fatalf("Unexplained = %d", report.Unexplained)
	}
}

func TestCrosscheckBaselineCoversCorpusExactly(t *testing.T) {
	root := repositoryRoot(t)
	corpus, err := crosscheck.LoadPairedCorpus(filepath.Join(root, "testdata", "upstream"))
	if err != nil {
		t.Fatal(err)
	}
	report, err := crosscheck.LoadReport(filepath.Join(root, "testdata", "crosscheck", "paired-baseline.json"))
	if err != nil {
		t.Fatal(err)
	}
	if len(corpus.Cases) != len(report.Cases) {
		t.Fatalf("corpus has %d cases, baseline has %d", len(corpus.Cases), len(report.Cases))
	}
	for i, testCase := range corpus.Cases {
		result := report.Cases[i]
		if result.ID != testCase.ID || result.Category != testCase.Category {
			t.Fatalf("case %d: baseline %q/%q, corpus %q/%q",
				i+1, result.ID, result.Category, testCase.ID, testCase.Category)
		}
		if result.Kind != crosscheck.Pass && result.KnownReason == "" {
			t.Fatalf("case %q has no reason", result.ID)
		}
	}
}

func TestCrosscheckSourcePairReasons(t *testing.T) {
	t.Parallel()

	report, err := crosscheck.LoadReport(filepath.Join(
		repositoryRoot(t), "testdata", "crosscheck", "paired-baseline.json",
	))
	if err != nil {
		t.Fatal(err)
	}
	sourceDifferences := []string{
		"middle", "braket", "greek", "arrows", "euler-product",
		"text-in-math", "null-delim", "color",
	}
	byID := make(map[string]crosscheck.CaseResult, len(report.Cases))
	for _, result := range report.Cases {
		byID[result.ID] = result
	}
	for _, id := range sourceDifferences {
		result, ok := byID[id]
		if !ok {
			t.Fatalf("missing source-pair case %q", id)
		}
		if !strings.Contains(strings.ToLower(result.KnownReason), "source-pair difference") {
			t.Fatalf("%s reason = %q, want a source-pair classification", id, result.KnownReason)
		}
	}
}
