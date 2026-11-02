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

func TestDocumentedDailyCorpusCounts(t *testing.T) {
	t.Parallel()

	manifest := readJSON[dailyManifest](t, "testdata/daily-stem/manifest.json")
	if manifest.Count != 92 || len(manifest.Categories) != 14 {
		t.Fatalf("daily manifest has %d cases and %d categories", manifest.Count, len(manifest.Categories))
	}
	if len(manifest.ContentSHA256) != 64 {
		t.Fatalf("daily manifest content digest = %q", manifest.ContentSHA256)
	}
	if len(manifest.ExpectedIssues) != 0 {
		t.Fatalf("daily manifest has known issues: %v", manifest.ExpectedIssues)
	}
	statements := []string{
		fmt.Sprintf("daily-STEM acceptance corpus with %d tracked cases across %d categories",
			manifest.Count, len(manifest.Categories)),
		fmt.Sprintf("%d TeX and %d Typst cases, %d inline and %d display cases, %d coverage groups, and %d explicitly source-specific cases",
			manifest.Dialects["tex"], manifest.Dialects["typst"], manifest.Modes["inline"],
			manifest.Modes["display"], manifest.CoverageGroups, manifest.SourceSpecific),
	}
	status := normalizeWhitespace(readText(t, "docs/status.md"))
	for _, statement := range statements {
		if !strings.Contains(status, normalizeWhitespace(statement)) {
			t.Errorf("docs/status.md is stale; missing %q", statement)
		}
	}
	readme := normalizeWhitespace(readText(t, "testdata/daily-stem/README.md"))
	readmeStatements := []string{
		fmt.Sprintf("%d cases across %d categories", manifest.Count, len(manifest.Categories)),
		fmt.Sprintf("%d TeX and %d Typst cases, with %d inline and %d display cases",
			manifest.Dialects["tex"], manifest.Dialects["typst"], manifest.Modes["inline"], manifest.Modes["display"]),
		fmt.Sprintf("%d coverage groups and %d explicitly source-specific cases",
			manifest.CoverageGroups, manifest.SourceSpecific),
	}
	for _, statement := range readmeStatements {
		if !strings.Contains(readme, normalizeWhitespace(statement)) {
			t.Errorf("testdata/daily-stem/README.md is stale; missing %q", statement)
		}
	}
	for category, count := range manifest.Categories {
		row := normalizeWhitespace(fmt.Sprintf("| %s | %d |", category, count))
		if !strings.Contains(readme, row) {
			t.Errorf("testdata/daily-stem/README.md is stale; missing category row %q", row)
		}
	}
	for _, required := range []string{
		"go run ./cmd/dailyreport -update-manifest",
		"go run ./cmd/dailyreport -allow-known-issues",
		"does not assert semantic equivalence",
	} {
		if !strings.Contains(readme, normalizeWhitespace(required)) {
			t.Errorf("testdata/daily-stem/README.md is missing %q", required)
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

type dailyManifest struct {
	ContentSHA256  string            `json:"contentSHA256"`
	Count          int               `json:"count"`
	Categories     map[string]int    `json:"categories"`
	Dialects       map[string]int    `json:"dialects"`
	Modes          map[string]int    `json:"modes"`
	CoverageGroups int               `json:"coverageGroups"`
	SourceSpecific int               `json:"sourceSpecific"`
	ExpectedIssues map[string]string `json:"expectedIssues"`
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
