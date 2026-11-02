// Command crosscheck verifies the pinned cross-dialect formula corpus.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/caelyreth/untie/internal/quality/crosscheck"
	"github.com/caelyreth/untie/internal/quality/fixture"
)

func main() {
	var fixtureRoot string
	var baseline string
	var writePath string
	var asJSON bool
	var refresh bool
	var reasonsPath string
	flag.StringVar(&fixtureRoot, "fixtures", "", "tracked upstream fixture root (default: discover testdata/upstream)")
	flag.StringVar(&baseline, "baseline", "", "data-driven known-differences baseline")
	flag.StringVar(&writePath, "write", "", "explicitly write the rendered report as a reviewed baseline")
	flag.BoolVar(&refresh, "refresh", false, "refresh details and accept only explicit kind-change reasons")
	flag.StringVar(&reasonsPath, "reasons", "", "JSON object of case ID to reviewed reason for -refresh")
	flag.BoolVar(&asJSON, "json", false, "print the complete verified report")
	flag.Parse()

	root, err := resolveFixtureRoot(fixtureRoot)
	if err != nil {
		fail(err)
	}
	if baseline == "" {
		base, err := fixture.CorpusRoot(".")
		if err != nil {
			fail(err)
		}
		baseline = filepath.Join(filepath.Dir(filepath.Dir(base)), "internal", "crosscheck", "testdata", "paired-baseline.json")
	}
	opts := crosscheck.Options{FixtureRoot: root, KnownDifferences: baseline}
	if refresh {
		if writePath == "" {
			fail(errors.New("-refresh requires -write"))
		}
		reasons := map[string]string{}
		if reasonsPath != "" {
			data, err := os.ReadFile(reasonsPath)
			if err != nil {
				fail(err)
			}
			decoder := json.NewDecoder(strings.NewReader(string(data)))
			decoder.DisallowUnknownFields()
			if err := decoder.Decode(&reasons); err != nil {
				fail(err)
			}
		}
		report, err := crosscheck.Refresh(context.Background(), opts, reasons)
		if err != nil {
			fail(err)
		}
		writeReport(report, writePath)
		return
	}
	report, err := crosscheck.Verify(context.Background(), opts)
	if err != nil {
		fail(err)
	}
	if writePath != "" {
		writeReport(report, writePath)
		return
	}
	want, err := crosscheck.LoadReport(baseline)
	if err != nil {
		fail(err)
	}
	if err := crosscheck.CompareReports(want, report); err != nil {
		fail(err)
	}
	if asJSON {
		data, err := report.MarshalCanonical()
		if err != nil {
			fail(err)
		}
		_, _ = os.Stdout.Write(data)
		return
	}
	printSummary(report)
}

func writeReport(report crosscheck.Report, path string) {
	data, err := report.MarshalCanonical()
	if err != nil {
		fail(err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		fail(err)
	}
	printSummary(report)
}

func printSummary(report crosscheck.Report) {
	fmt.Printf("paired crosscheck: %d pass, %d semantic, %d layout, %d fixture, %d pair differences, %d unexplained\n",
		report.Pass, report.SemanticIR, report.Layout, report.Fixture,
		report.PairDifference, report.Unexplained)
}

func resolveFixtureRoot(flagValue string) (string, error) {
	if flagValue != "" {
		return flagValue, nil
	}
	root, err := fixture.CorpusRoot(".")
	if err != nil {
		return "", err
	}
	return root, nil
}

func fail(err error) {
	if errors.Is(err, flag.ErrHelp) {
		os.Exit(0)
	}
	fmt.Fprintln(os.Stderr, "crosscheck:", err)
	os.Exit(1)
}
