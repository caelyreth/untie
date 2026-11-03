package main

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunWritesSVG(t *testing.T) {
	fontPath := filepath.Join("..", "..", "testdata", "upstream", "fonts", "NewCMMath-Book.otf")
	var output bytes.Buffer
	err := run([]string{
		"-font", fontPath,
		"-source", `x^2+\frac{1}{2}`,
		"-padding", "10",
	}, &output)
	if err != nil {
		t.Fatal(err)
	}
	document := output.String()
	for _, want := range []string{
		`<?xml version="1.0" encoding="UTF-8"?>`,
		`<svg `,
		`viewBox="0 0 `,
		`<path `,
		`<rect `,
	} {
		if !strings.Contains(document, want) {
			t.Errorf("SVG output does not contain %q", want)
		}
	}
}

func TestRunRejectsUnknownDialect(t *testing.T) {
	err := run([]string{"-dialect", "unknown"}, &bytes.Buffer{})
	if err == nil || !strings.Contains(err.Error(), "unknown dialect") {
		t.Fatalf("run error = %v", err)
	}
}
