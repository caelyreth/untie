package daily

import (
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestTrackedCorpusMatchesExactManifest(t *testing.T) {
	t.Parallel()

	cases, manifest := loadTrackedCorpus(t)
	if err := ValidateCorpus(cases, manifest); err != nil {
		t.Fatal(err)
	}
	if len(cases) != 92 {
		t.Fatalf("case count = %d, want 92", len(cases))
	}
	wantCategories := map[string]int{
		"algebra":                10,
		"calculus":               12,
		"delimiters":             6,
		"display-wrapping":       5,
		"equation-tags":          3,
		"links":                  2,
		"linear-algebra":         10,
		"malformed-input":        4,
		"multiline-cases":        6,
		"physics-engineering":    10,
		"probability-statistics": 8,
		"resource-limits":        4,
		"sets-logic":             8,
		"text-and-fallback":      4,
	}
	if !mapsEqual(manifest.Categories, wantCategories) {
		t.Fatalf("categories = %v, want %v", manifest.Categories, wantCategories)
	}
	if manifest.Count != 92 || manifest.CoverageGroups != 35 || manifest.SourceSpecific != 22 {
		t.Fatalf("manifest counts = total %d, groups %d, specific %d",
			manifest.Count, manifest.CoverageGroups, manifest.SourceSpecific)
	}
	if manifest.Dialects["tex"] != 51 || manifest.Dialects["typst"] != 41 {
		t.Fatalf("dialect counts = %v", manifest.Dialects)
	}
	if manifest.Modes["inline"] != 37 || manifest.Modes["display"] != 55 {
		t.Fatalf("mode counts = %v", manifest.Modes)
	}
	if len(manifest.ExpectedIssues) != 0 {
		t.Fatalf("expected issues = %v, want none after the decimal fix", manifest.ExpectedIssues)
	}
	if len(manifest.ContentSHA256) != 64 {
		t.Fatalf("content digest = %q", manifest.ContentSHA256)
	}

	exactAlt, exactHits, rootKinds := 0, 0, 0
	for _, item := range cases {
		if item.Expect.AltText != "" {
			exactAlt++
		}
		if len(item.Expect.Hits) != 0 {
			exactHits++
		}
		if item.Expect.RootKind != "" {
			rootKinds++
		}
	}
	if exactAlt < 9 || exactHits < 4 || rootKinds != 4 {
		t.Fatalf("reviewed detail coverage = alt %d, hits %d, root kinds %d", exactAlt, exactHits, rootKinds)
	}
}

func TestCanonicalDigestPinsEveryCaseField(t *testing.T) {
	t.Parallel()

	cases, manifest := loadTrackedCorpus(t)
	baseline := manifest.ContentSHA256
	index := slices.IndexFunc(cases, func(item Case) bool {
		return item.ID == "algebra.fraction.tex"
	})
	if index < 0 {
		t.Fatal("representative case not found")
	}
	mutations := []struct {
		name   string
		mutate func(*Case)
	}{
		{"source", func(item *Case) { item.Source += " " }},
		{"category", func(item *Case) { item.Category = "calculus" }},
		{"coverage-group", func(item *Case) { item.CoverageGroup = "algebra.changed" }},
		{"difference", func(item *Case) { item.ExpectedDifference = "alternative-text" }},
		{"dialect", func(item *Case) { item.Dialect = "typst" }},
		{"mode", func(item *Case) { item.Mode = "display" }},
		{"limits", func(item *Case) { item.Limits = &LimitsSpec{MaxNodes: 123} }},
		{"style", func(item *Case) { item.Style = &StyleSpec{Size: 17, MaxWidth: 80, Align: alignCenter} }},
		{"expectation", func(item *Case) { item.Expect.AltText = "changed" }},
		{"issue", func(item *Case) { item.Issue = "tracked-test-issue" }},
	}
	for _, mutation := range mutations {
		mutation := mutation
		t.Run(mutation.name, func(t *testing.T) {
			t.Parallel()
			changed := slices.Clone(cases)
			mutation.mutate(&changed[index])
			digest, err := contentDigest(changed, manifest)
			if err != nil {
				t.Fatal(err)
			}
			if digest == baseline {
				t.Fatalf("%s mutation did not change the digest", mutation.name)
			}
		})
	}

	changedManifest := manifest
	changedManifest.Count++
	digest, err := contentDigest(cases, changedManifest)
	if err != nil {
		t.Fatal(err)
	}
	if digest == baseline {
		t.Fatal("manifest invariant mutation did not change the digest")
	}
}

func TestDigestDriftAndReviewedManifestUpdate(t *testing.T) {
	t.Parallel()

	cases, manifest := loadTrackedCorpus(t)
	changed := slices.Clone(cases)
	index := slices.IndexFunc(changed, func(item Case) bool {
		return item.ID == "calculus.limit.tex"
	})
	changed[index].Expect.AltSpeech += "!"
	if err := ValidateCorpus(changed, manifest); err == nil ||
		!strings.Contains(err.Error(), "-update-manifest") {
		t.Fatalf("content drift error = %v", err)
	}

	updated, err := BuildManifest(changed, manifest.CaseFile)
	if err != nil {
		t.Fatal(err)
	}
	if updated.ContentSHA256 == manifest.ContentSHA256 {
		t.Fatal("reviewed update retained the stale digest")
	}
	path := filepath.Join(t.TempDir(), "manifest.json")
	if err := WriteManifest(path, updated); err != nil {
		t.Fatal(err)
	}
	reloaded, err := LoadManifest(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := ValidateCorpus(changed, reloaded); err != nil {
		t.Fatal(err)
	}
}

func TestCorpusRejectsUnclosedClassificationsAndPairDrift(t *testing.T) {
	t.Parallel()

	cases, manifest := loadTrackedCorpus(t)
	tests := []struct {
		name   string
		mutate func(*Case)
		want   string
	}{
		{"no classification", func(item *Case) {
			item.CoverageGroup = ""
			item.ExpectedDifference = ""
		}, "exactly one"},
		{"coverage without difference", func(item *Case) {
			item.ExpectedDifference = ""
		}, "unknown expected difference"},
		{"unknown difference", func(item *Case) {
			item.ExpectedDifference = "vibes"
		}, "unknown expected difference"},
		{"unknown source reason", func(item *Case) {
			item.CoverageGroup = ""
			item.ExpectedDifference = ""
			item.SourceSpecific = "arbitrary-text"
		}, "outside the taxonomy"},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			changed := slices.Clone(cases)
			test.mutate(&changed[0])
			err := changed[0].Validate()
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("Validate() error = %v, want %q", err, test.want)
			}
		})
	}

	badPair := slices.Clone(cases)
	for i := range badPair {
		if badPair[i].ID == "algebra.fraction.typst" {
			badPair[i].Mode = "display"
			break
		}
	}
	if _, err := BuildManifest(badPair, manifest.CaseFile); err == nil ||
		!strings.Contains(err.Error(), "mixes category, mode, or expected difference") {
		t.Fatalf("mixed coverage group error = %v", err)
	}
}

func TestLoadCasesRejectsMalformedCorpus(t *testing.T) {
	t.Parallel()

	const valid = `{"id":"sample.case","category":"algebra","coverageGroup":"sample.case","expectedDifference":"source-syntax","dialect":"tex","mode":"inline","source":"x","expect":{"parse":"ok","layout":"ok"}}`
	tests := []struct {
		name    string
		corpus  string
		wantErr string
	}{
		{"blank line", valid + "\n\n", "blank"},
		{"duplicate ID", valid + "\n" + valid + "\n", "duplicate"},
		{"unknown field", strings.Replace(valid, `"source":"x"`, `"source":"x","unknown":true`, 1), "unknown field"},
		{"bad expectation", strings.Replace(valid, `"layout":"ok"`, `"layout":"skip"`, 1), "layout expectation"},
		{"multiple values", valid + " {}", "multiple JSON values"},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			_, err := LoadCases(strings.NewReader(test.corpus))
			if err == nil || !strings.Contains(err.Error(), test.wantErr) {
				t.Fatalf("LoadCases() error = %v, want fragment %q", err, test.wantErr)
			}
		})
	}
}

func loadTrackedCorpus(t testing.TB) ([]Case, Manifest) {
	t.Helper()
	root := filepath.Join("..", "..", "..", "testdata", "daily-stem")
	cases, err := LoadCaseFile(filepath.Join(root, "cases.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	manifest, err := LoadManifest(filepath.Join(root, "manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	return cases, manifest
}
