package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestRunWritesMathML(t *testing.T) {
	var output bytes.Buffer
	err := run([]string{
		"-source", `\frac{1}{2}`,
		"-mode", "display",
	}, &output)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		`<math xmlns="http://www.w3.org/1998/Math/MathML"`,
		`display="block"`,
		`<mfrac`,
	} {
		if !strings.Contains(output.String(), want) {
			t.Errorf("output does not contain %q: %s", want, output.String())
		}
	}
}

func TestRunRejectsUnknownModes(t *testing.T) {
	err := run([]string{"-dialect", "unknown"}, &bytes.Buffer{})
	if err == nil || !strings.Contains(err.Error(), "unknown dialect") {
		t.Fatalf("error = %v", err)
	}
	err = run([]string{"-mode", "unknown"}, &bytes.Buffer{})
	if err == nil || !strings.Contains(err.Error(), "unknown mode") {
		t.Fatalf("error = %v", err)
	}
}
