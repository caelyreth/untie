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

func TestRunAlignsConstrainedDisplay(t *testing.T) {
	t.Parallel()

	font := filepath.Join("..", "..", "testdata", "upstream", "fonts", "NewCMMath-Book.otf")
	render := func(align string) string {
		t.Helper()
		var stdout, stderr bytes.Buffer
		code := run([]string{
			"-font", font,
			"-display",
			"-max-width", "200",
			"-align", align,
			"x",
		}, strings.NewReader(""), &stdout, &stderr)
		if code != 0 {
			t.Fatalf("run(-align %s) = %d, stderr = %s", align, code, stderr.String())
		}
		return stdout.String()
	}
	start := render("start")
	center := render("center")
	end := render("end")
	if start == center || center == end || start == end {
		t.Fatal("alignment did not change the public preview geometry")
	}
}

func TestRunRejectsUnknownAlign(t *testing.T) {
	t.Parallel()

	font := filepath.Join("..", "..", "testdata", "upstream", "fonts", "NewCMMath-Book.otf")
	var stdout, stderr bytes.Buffer
	code := run([]string{"-font", font, "-align", "middle", "x"}, strings.NewReader(""), &stdout, &stderr)
	if code != 2 {
		t.Fatalf("run() = %d, want 2", code)
	}
	if !strings.Contains(stderr.String(), `unknown align "middle"`) {
		t.Fatalf("stderr = %q", stderr.String())
	}
}
