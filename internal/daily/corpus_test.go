package daily

import (
	"crypto/sha256"
	"fmt"
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
	if len(cases) != 81 {
		t.Fatalf("case count = %d, want 81", len(cases))
	}
	wantCategories := map[string]int{
		"algebra":                10,
		"calculus":               12,
		"delimiters":             6,
		"display-wrapping":       1,
		"linear-algebra":         10,
		"malformed-input":        4,
		"multiline-cases":        6,
		"physics-engineering":    10,
		"probability-statistics": 8,
		"resource-limits":        2,
		"sets-logic":             8,
		"text-and-fallback":      4,
	}
	if !mapsEqual(manifest.Categories, wantCategories) {
		t.Fatalf("categories = %v, want %v", manifest.Categories, wantCategories)
	}
	ids := make([]string, len(cases))
	for i, item := range cases {
		ids[i] = item.ID
	}
	slices.Sort(ids)
	sum := sha256.Sum256([]byte(strings.Join(ids, "\n") + "\n"))
	const wantIDHash = "e5fe4ebf73bfe9e25ad27797b05e05b0764e8d944d13247589f60a9670e9501c"
	if got := fmt.Sprintf("%x", sum); got != wantIDHash {
		t.Fatalf("case ID hash = %s, want %s", got, wantIDHash)
	}
	if manifest.EquivalentGroups != 34 || manifest.SourceSpecific != 13 {
		t.Fatalf("manifest groups/specific = %d/%d, want 34/13",
			manifest.EquivalentGroups, manifest.SourceSpecific)
	}
}

func TestCorpusCoverageShape(t *testing.T) {
	t.Parallel()

	cases, _ := loadTrackedCorpus(t)
	dialects := make(map[string]int)
	modes := make(map[string]int)
	layouts := make(map[string]int)
	parses := make(map[string]int)
	for _, item := range cases {
		dialects[item.Dialect]++
		modes[item.Mode]++
		layouts[item.Expect.Layout]++
		parses[item.Expect.Parse]++
	}
	for name, got := range map[string]map[string]int{
		"dialect": dialects, "mode": modes, "layout": layouts, "parse": parses,
	} {
		t.Logf("%s counts: %v", name, got)
	}
	if dialects["tex"] != 41 || dialects["typst"] != 40 {
		t.Fatalf("dialect counts = %v, want tex=41 typst=40", dialects)
	}
	if modes["inline"] != 33 || modes["display"] != 48 {
		t.Fatalf("mode counts = %v, want inline=33 display=48", modes)
	}
	if layouts["missing-glyph"] != 5 || parses["diagnostic"] != 6 {
		t.Fatalf("boundary counts = layout %v parse %v, want 5 missing-glyph and 6 diagnostic",
			layouts, parses)
	}
}

func TestCorpusRejectsClassificationAndManifestDrift(t *testing.T) {
	t.Parallel()

	cases, manifest := loadTrackedCorpus(t)
	bad := slices.Clone(cases)
	bad[0].SourceSpecific = ""
	bad[0].EquivalentGroup = ""
	if err := bad[0].Validate(); err == nil {
		t.Fatal("case without a comparison classification passed validation")
	}

	missing := manifest
	missing.CaseIDs = append([]string(nil), manifest.CaseIDs[:len(manifest.CaseIDs)-1]...)
	missing.Count--
	if err := ValidateCorpus(cases, missing); err == nil {
		t.Fatal("manifest with a missing case ID passed validation")
	}

	badCategory := manifest
	badCategory.Categories = mapsClone(manifest.Categories)
	badCategory.Categories["algebra"]++
	if err := ValidateCorpus(cases, badCategory); err == nil {
		t.Fatal("manifest with a wrong category count passed validation")
	}

	badDialect := manifest
	badDialect.Dialects = mapsClone(manifest.Dialects)
	badDialect.Dialects["tex"]++
	if err := ValidateCorpus(cases, badDialect); err == nil {
		t.Fatal("manifest with a wrong dialect count passed validation")
	}

	badPair := slices.Clone(cases)
	for i := range badPair {
		if badPair[i].ID == "algebra.fraction.typst" {
			badPair[i].Mode = "display"
			break
		}
	}
	if err := ValidateCorpus(badPair, manifest); err == nil {
		t.Fatal("equivalent group with different modes passed validation")
	}
}

func TestLoadCasesRejectsMalformedCorpus(t *testing.T) {
	t.Parallel()

	const valid = `{"id":"sample.case","category":"algebra","sourceSpecific":"sample","dialect":"tex","mode":"inline","source":"x","expect":{"parse":"ok","layout":"ok"}}`
	tests := []struct {
		name    string
		corpus  string
		wantErr string
	}{
		{"blank line", valid + "\n\n", "blank"},
		{"duplicate ID", valid + "\n" + valid + "\n", "duplicate"},
		{"unknown field", strings.Replace(valid, `"source":"x"`, `"source":"x","unknown":true`, 1), "unknown field"},
		{"no classification", strings.Replace(valid, `"sourceSpecific":"sample",`, "", 1), "exactly one"},
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
	root := filepath.Join("..", "..", "testdata", "daily-stem")
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

func mapsClone[V comparable](in map[string]V) map[string]V {
	out := make(map[string]V, len(in))
	for key, value := range in {
		out[key] = value
	}
	return out
}
