package main

import (
	"bytes"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/caelyreth/untie/internal/daily"
)

func TestRunReportsFullCorpus(t *testing.T) {
	var stdout, stderr bytes.Buffer
	root := filepath.Join("..", "..")
	code := run([]string{
		"-corpus", filepath.Join(root, "testdata", "daily-stem", "cases.jsonl"),
		"-manifest", filepath.Join(root, "testdata", "daily-stem", "manifest.json"),
		"-font", filepath.Join(root, "testdata", "upstream", "fonts", "NewCMMath-Book.otf"),
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("run() = %d, stderr = %s, stdout = %s", code, stderr.String(), stdout.String())
	}
	for _, want := range []string{
		"daily STEM: 81 cases, 81 pass, 0 fail",
		"outcomes: 72 rendered, 4 recovered, 5 missing-glyph, 0 layout-error, 0 failure",
		"known issues: 1",
		"algebra=10/10",
	} {
		if !strings.Contains(stdout.String(), want) {
			t.Errorf("stdout does not contain %q:\n%s", want, stdout.String())
		}
	}
}

func TestRunRejectsMissingFont(t *testing.T) {
	var stdout, stderr bytes.Buffer
	root := filepath.Join("..", "..")
	code := run([]string{
		"-corpus", filepath.Join(root, "testdata", "daily-stem", "cases.jsonl"),
		"-manifest", filepath.Join(root, "testdata", "daily-stem", "manifest.json"),
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

type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) {
	return 0, errors.New("write failed")
}
