package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/caelyreth/untie/internal/daily"
)

func TestRunReportsSmallCorpus(t *testing.T) {
	corpus, manifest, font := writeCLIFixture(t)
	paths := []string{"-corpus", corpus, "-manifest", manifest, "-font", font}
	var stdout, stderr bytes.Buffer
	code := run(paths, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("run() = %d, stderr = %s, stdout = %s", code, stderr.String(), stdout.String())
	}
	for _, want := range []string{
		"daily STEM: 2 cases, 2 pass, 0 known-issue failures, 0 unexpected failures",
		"outcomes: 2 rendered, 0 recovered, 0 missing-glyph, 0 layout-error, 0 failure",
		"algebra=2/2",
	} {
		if !strings.Contains(stdout.String(), want) {
			t.Errorf("stdout does not contain %q:\n%s", want, stdout.String())
		}
	}
}

func TestRunUpdatesManifestExplicitly(t *testing.T) {
	corpus, manifest, font := writeCLIFixture(t)
	if err := os.Remove(manifest); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	code := run([]string{"-corpus", corpus, "-manifest", manifest, "-update-manifest"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("update run() = %d, stderr = %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "updated manifest: 2 cases, 1 coverage groups, 0 source-specific") {
		t.Fatalf("update stdout = %s", stdout.String())
	}
	stdout.Reset()
	stderr.Reset()
	code = run([]string{"-corpus", corpus, "-manifest", manifest, "-font", font}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("post-update run() = %d, stderr = %s", code, stderr.String())
	}
}

func TestKnownAndUnexpectedExitPolicy(t *testing.T) {
	t.Parallel()

	known := syntheticReport(t, daily.CaseResult{
		ID: "synthetic.issue", Category: "test", Outcome: "missing-glyph",
		Matched: true, Issue: "tracked-test-issue",
	})
	if reportExitCode(known, false) == 0 {
		t.Fatal("default CLI policy accepted a matched known issue")
	}
	if reportExitCode(known, true) != 0 {
		t.Fatal("-allow-known-issues rejected a matched known issue")
	}
	var summary bytes.Buffer
	if err := printSummary(&summary, known); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(summary.String(), "0 pass, 1 known-issue failures, 0 unexpected failures") ||
		!strings.Contains(summary.String(), "KNOWN ISSUE synthetic.issue (tracked-test-issue)") {
		t.Fatalf("known summary = %s", summary.String())
	}

	unexpected := syntheticReport(t, daily.CaseResult{
		ID: "synthetic.failure", Category: "test", Outcome: "failure",
		Matched: false, Detail: "observed mismatch",
	})
	if reportExitCode(unexpected, true) == 0 {
		t.Fatal("-allow-known-issues accepted an unexpected failure")
	}
	summary.Reset()
	if err := printSummary(&summary, unexpected); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(summary.String(), "1 unexpected failures") ||
		!strings.Contains(summary.String(), "UNEXPECTED FAILURE synthetic.failure") {
		t.Fatalf("unexpected summary = %s", summary.String())
	}
}

func TestRunKnownIssueFailsClosed(t *testing.T) {
	corpus, manifest, font := writeKnownIssueFixture(t)
	base := []string{"-corpus", corpus, "-manifest", manifest, "-font", font}
	var stdout, stderr bytes.Buffer
	if code := run(base, &stdout, &stderr); code == 0 {
		t.Fatalf("default known-issue run succeeded:\n%s", stdout.String())
	}
	if !strings.Contains(stdout.String(), "0 pass, 1 known-issue failures, 0 unexpected failures") ||
		!strings.Contains(stdout.String(), "KNOWN ISSUE cli.known.tex") {
		t.Fatalf("default known-issue summary = %s", stdout.String())
	}
	if !strings.Contains(stderr.String(), "allow-known-issues") {
		t.Fatalf("default known-issue stderr = %q", stderr.String())
	}

	stdout.Reset()
	stderr.Reset()
	if code := run(append(base, "-allow-known-issues"), &stdout, &stderr); code != 0 {
		t.Fatalf("diagnostic run = %d, stderr = %s", code, stderr.String())
	}
}

func TestRunRejectsMissingFont(t *testing.T) {
	corpus, manifest, _ := writeCLIFixture(t)
	var stdout, stderr bytes.Buffer
	code := run([]string{
		"-corpus", corpus,
		"-manifest", manifest,
		"-font", filepath.Join(t.TempDir(), "missing.otf"),
	}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("run() = %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), "dailyreport:") {
		t.Fatalf("stderr = %q", stderr.String())
	}
}

func TestRunHelp(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if code := run([]string{"-h"}, &stdout, &stderr); code != 0 {
		t.Fatalf("run(-h) = %d, want 0", code)
	}
}

func TestPrintSummaryReportsWriteFailure(t *testing.T) {
	if err := printSummary(failingWriter{}, daily.Report{}); err == nil {
		t.Fatal("printSummary ignored a write failure")
	}
}

func writeCLIFixture(t testing.TB) (corpus, manifest, font string) {
	t.Helper()
	directory := t.TempDir()
	corpus = filepath.Join(directory, "cases.jsonl")
	manifest = filepath.Join(directory, "manifest.json")
	cases := []daily.Case{
		{
			ID: "cli.sample.tex", Category: "algebra", CoverageGroup: "cli.sample",
			ExpectedDifference: "source-syntax", Dialect: "tex", Mode: "inline", Source: "x",
			Expect: daily.Expectation{Parse: "ok", Layout: "ok"},
		},
		{
			ID: "cli.sample.typst", Category: "algebra", CoverageGroup: "cli.sample",
			ExpectedDifference: "source-syntax", Dialect: "typst", Mode: "inline", Source: "x",
			Expect: daily.Expectation{Parse: "ok", Layout: "ok"},
		},
	}
	var data bytes.Buffer
	encoder := json.NewEncoder(&data)
	for _, item := range cases {
		if err := encoder.Encode(item); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(corpus, data.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
	declared, err := daily.BuildManifest(cases, filepath.Base(corpus))
	if err != nil {
		t.Fatal(err)
	}
	if err := daily.WriteManifest(manifest, declared); err != nil {
		t.Fatal(err)
	}
	font = filepath.Join("..", "..", "testdata", "upstream", "fonts", "NewCMMath-Book.otf")
	return corpus, manifest, font
}

func writeKnownIssueFixture(t testing.TB) (corpus, manifest, font string) {
	t.Helper()
	directory := t.TempDir()
	corpus = filepath.Join(directory, "cases.jsonl")
	manifest = filepath.Join(directory, "manifest.json")
	cases := []daily.Case{{
		ID: "cli.known.tex", Category: "text-and-fallback",
		SourceSpecific: "host-font-boundary", Dialect: "tex", Mode: "inline",
		Source: `x+\text{中}`,
		Expect: daily.Expectation{
			Parse: "ok", Layout: "missing-glyph",
			LayoutErrorContains: []string{`missing glyph: "中"`},
		},
		Issue: "host-font-missing",
	}}
	var data bytes.Buffer
	if err := json.NewEncoder(&data).Encode(cases[0]); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(corpus, data.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
	declared, err := daily.BuildManifest(cases, filepath.Base(corpus))
	if err != nil {
		t.Fatal(err)
	}
	if err := daily.WriteManifest(manifest, declared); err != nil {
		t.Fatal(err)
	}
	font = filepath.Join("..", "..", "testdata", "upstream", "fonts", "NewCMMath-Book.otf")
	return corpus, manifest, font
}

func syntheticReport(t testing.TB, result daily.CaseResult) daily.Report {
	t.Helper()
	runner, err := daily.NewRunnerWithEvaluator(daily.EvaluatorFunc(func(context.Context, daily.Case) daily.CaseResult {
		return result
	}), time.Second)
	if err != nil {
		t.Fatal(err)
	}
	report, err := runner.Run(t.Context(), []daily.Case{{
		ID: result.ID, Category: result.Category, Dialect: "tex", Mode: "inline",
		Issue: result.Issue,
	}})
	if err != nil {
		t.Fatal(err)
	}
	return report
}

type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) {
	return 0, errors.New("write failed")
}
