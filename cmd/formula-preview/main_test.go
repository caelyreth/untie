package main

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunRendersSVG(t *testing.T) {
	t.Parallel()

	font := filepath.Join("..", "..", "testdata", "upstream", "fonts", "NewCMMath-Book.otf")
	var stdout, stderr bytes.Buffer
	code := run([]string{"-font", font, "x^2"}, strings.NewReader(""), &stdout, &stderr)
	if code != 0 {
		t.Fatalf("run() = %d, stderr = %s", code, stderr.String())
	}
	if !strings.HasPrefix(stdout.String(), "<svg ") || !strings.Contains(stdout.String(), "<path ") {
		t.Fatalf("unexpected SVG: %.200s", stdout.String())
	}
}

func TestRunRequiresFont(t *testing.T) {
	t.Parallel()

	var stdout, stderr bytes.Buffer
	if code := run([]string{"x"}, strings.NewReader(""), &stdout, &stderr); code != 2 {
		t.Fatalf("run() = %d, want 2", code)
	}
}
