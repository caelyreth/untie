package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestRunTeX(t *testing.T) {
	t.Parallel()

	var stdout, stderr bytes.Buffer
	code := run([]string{"-dialect", "tex", "x^2"}, strings.NewReader(""), &stdout, &stderr)
	if code != 0 {
		t.Fatalf("run() = %d, stderr = %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), `"kind":"row"`) {
		t.Fatalf("stdout = %s", stdout.String())
	}
}

func TestRunReadsStdin(t *testing.T) {
	t.Parallel()

	var stdout, stderr bytes.Buffer
	code := run([]string{"-dialect", "typst"}, strings.NewReader("x^2"), &stdout, &stderr)
	if code != 0 {
		t.Fatalf("run() = %d, stderr = %s", code, stderr.String())
	}
	if stdout.Len() == 0 {
		t.Fatal("stdout is empty")
	}
}

func TestRunRejectsBadDialect(t *testing.T) {
	t.Parallel()

	var stdout, stderr bytes.Buffer
	code := run([]string{"-dialect", "unknown", "x"}, strings.NewReader(""), &stdout, &stderr)
	if code != 2 {
		t.Fatalf("run() = %d, want 2", code)
	}
}
