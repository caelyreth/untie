package untie_test

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

func TestDocumentedConformanceCounts(t *testing.T) {
	t.Parallel()

	tex := readJSON[conformanceSummary](t, "internal/conformance/testdata/tex-baseline.json")
	typst := readJSON[conformanceSummary](t, "internal/conformance/testdata/typst-baseline.json")
	paired := readJSON[crosscheckSummary](t, "internal/crosscheck/testdata/paired-baseline.json")

	conformanceReadme := normalizeWhitespace(readText(t, "internal/conformance/README.md"))
	for _, statement := range []string{
		fmt.Sprintf("%s passes, %s expected diagnostics, %s unsupported cases, and %s reviewed parser gaps",
			formatCount(tex.Summary.Pass), formatCount(tex.Summary.Diagnostic),
			formatCount(tex.Summary.Unsupported), formatCount(tex.Summary.Bug)),
		fmt.Sprintf("%s passes, %s expected upstream diagnostics, %s explicit depth-limit scope exclusions, and %s reviewed known gaps",
			formatCount(typst.Summary.Pass), formatCount(typst.Summary.Diagnostic),
			formatCount(typst.Summary.Unsupported), formatCount(typst.Summary.Bug)),
	} {
		if !strings.Contains(conformanceReadme, normalizeWhitespace(statement)) {
			t.Errorf("internal/conformance/README.md is stale; missing %q", statement)
		}
	}

	crosscheckReadme := readText(t, "internal/crosscheck/README.md")
	for _, field := range []struct {
		label string
		value int
	}{
		{"pass", paired.Pass},
		{"semantic-ir-mismatch", paired.SemanticIRMismatch},
		{"layout-mismatch", paired.LayoutMismatch},
		{"fixture-mismatch", paired.FixtureMismatch},
		{"documented-pair-difference", paired.DocumentedPairDifference},
		{"unexplained", paired.Unexplained},
	} {
		pattern := regexp.MustCompile(`(?m)^` + regexp.QuoteMeta(field.label) + `\s+` + strconv.Itoa(field.value) + `\s*$`)
		if !pattern.MatchString(crosscheckReadme) {
			t.Errorf("internal/crosscheck/README.md does not document %s=%d", field.label, field.value)
		}
	}
}

func TestTrackedDependencyLicenses(t *testing.T) {
	t.Parallel()

	paths := []string{
		"licenses/dsnet-compress-BSD-3-Clause.txt",
		"licenses/go-opentype-BSD-3-Clause.txt",
		"licenses/go-text-typesetting-Unlicense-OR-BSD-3-Clause.txt",
		"licenses/golang-x-image-BSD-3-Clause.txt",
		"licenses/golang-x-image-PATENTS.txt",
	}
	notices := readText(t, "THIRD_PARTY_NOTICES.md")
	for _, path := range paths {
		info, err := os.Stat(path)
		if err != nil {
			t.Errorf("tracked dependency license %s: %v", path, err)
			continue
		}
		if info.Size() == 0 {
			t.Errorf("tracked dependency license %s is empty", path)
		}
		if !strings.Contains(notices, path) {
			t.Errorf("THIRD_PARTY_NOTICES.md does not reference %s", path)
		}
	}
}

type conformanceSummary struct {
	Summary struct {
		Pass        int `json:"pass"`
		Diagnostic  int `json:"diagnostic"`
		Unsupported int `json:"unsupported"`
		Bug         int `json:"bug"`
		Unexplained int `json:"unexplained"`
	} `json:"summary"`
}

type crosscheckSummary struct {
	Pass                     int `json:"pass"`
	SemanticIRMismatch       int `json:"semanticIRMismatch"`
	LayoutMismatch           int `json:"layoutMismatch"`
	FixtureMismatch          int `json:"fixtureMismatch"`
	DocumentedPairDifference int `json:"documentedPairDifference"`
	Unexplained              int `json:"unexplained"`
}

func readJSON[T any](t testing.TB, path string) T {
	t.Helper()
	data, err := os.ReadFile(filepath.FromSlash(path))
	if err != nil {
		t.Fatal(err)
	}
	var value T
	if err := json.Unmarshal(data, &value); err != nil {
		t.Fatalf("%s: %v", path, err)
	}
	return value
}

func readText(t testing.TB, path string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.FromSlash(path))
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func formatCount(value int) string {
	text := strconv.Itoa(value)
	for i := len(text) - 3; i > 0; i -= 3 {
		text = text[:i] + "," + text[i:]
	}
	return text
}

func normalizeWhitespace(text string) string {
	return strings.Join(strings.Fields(text), " ")
}
