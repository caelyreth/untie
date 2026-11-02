package fixture

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/caelyreth/untie/source"
)

func TestResolveFixtureConvertsSupplementaryPlaneCSTAndDiagnosticSpans(t *testing.T) {
	t.Parallel()

	raw := `{
		"id":"astral",
		"cst":["Math",0,3,[["MathText",0,2],["Error",2,3,"bad glyph",["use a letter"]]]],
		"diagnostics":[{"at":"src","message":"bad","severity":"error","hints":["fix it"],"span":[0,2]}]
	}`
	var fixture Fixture
	if err := json.Unmarshal([]byte(raw), &fixture); err != nil {
		t.Fatal(err)
	}
	const src = "😀x"
	resolved, err := ResolveFixture(src, "", &fixture)
	if err != nil {
		t.Fatal(err)
	}
	if resolved.CST == nil || resolved.CST.Span != source.NewRange(0, len(src)) {
		t.Fatalf("CST root = %#v", resolved.CST)
	}
	if got := string(resolved.CST.Children[0].Span.Slice(src)); got != "😀" {
		t.Fatalf("first CST child = %q", got)
	}
	errorNode := resolved.CST.Children[1]
	if errorNode.Kind != "Error" || errorNode.Message != "bad glyph" ||
		len(errorNode.Hints) != 1 || errorNode.Span != source.NewRange(4, 5) {
		t.Fatalf("error node = %#v", errorNode)
	}
	if len(resolved.Diagnostics) != 1 || !resolved.Diagnostics[0].HasSpan {
		t.Fatalf("diagnostics = %#v", resolved.Diagnostics)
	}
	if got := string(resolved.Diagnostics[0].Span.Slice(src)); got != "😀" {
		t.Fatalf("diagnostic span = %q", got)
	}
}

func TestResolveFixtureSelectsSourceForDiagnosticTarget(t *testing.T) {
	t.Parallel()

	fixture := Fixture{
		ID: "targets",
		Diagnostics: []FixtureDiagnostic{
			{At: "src", Span: &UTF16Range{Start: 0, End: 1}},
			{At: "preamble", Span: &UTF16Range{Start: 2, End: 5}},
		},
	}
	resolved, err := ResolveFixture("abc", "xxPQRyy", &fixture)
	if err != nil {
		t.Fatal(err)
	}
	if got := string(resolved.Diagnostics[0].Span.Slice("abc")); got != "a" {
		t.Fatalf("source diagnostic = %q", got)
	}
	if got := string(resolved.Diagnostics[1].Span.Slice("xxPQRyy")); got != "PQR" {
		t.Fatalf("preamble diagnostic = %q", got)
	}

	fixture.Diagnostics[1].At = "unknown"
	if _, err := ResolveFixture("abc", "xxPQRyy", &fixture); err == nil ||
		!strings.Contains(err.Error(), "unknown source target") {
		t.Fatalf("unknown target error = %v", err)
	}
}

func TestResolveFixtureRejectsSpanInsideSurrogatePair(t *testing.T) {
	t.Parallel()

	fixture := Fixture{
		ID: "surrogate",
		CST: &CSTNode{
			Kind: "Math", Span: UTF16Range{Start: 0, End: 2},
			Children: []CSTNode{{Kind: "MathText", Span: UTF16Range{Start: 0, End: 1}}},
		},
	}
	if _, err := ResolveFixture("😀", "", &fixture); err == nil {
		t.Fatal("expected a boundary error")
	}
}

func TestImportedCSTAndDiagnosticSpansResolve(t *testing.T) {
	t.Parallel()

	root := upstreamRoot(t)
	manifest, err := LoadCorpusManifest(filepath.Join(root, "manifests", "corpora.json"))
	if err != nil {
		t.Fatal(err)
	}
	specs := make(map[string]CorpusSpec, len(manifest.Corpora))
	for _, spec := range manifest.Corpora {
		specs[spec.Name] = spec
	}

	syntax, err := LoadCorpus(root, specs["typlet-syntax"])
	if err != nil {
		t.Fatal(err)
	}
	astral, ok := findCase(syntax, "astral-letters")
	if !ok {
		t.Fatal("imported astral-letters case not found")
	}
	if astral.CST == nil || astral.CST.Span != source.NewRange(0, len(astral.Source)) {
		t.Fatalf("astral CST = %#v", astral.CST)
	}
	if got := string(astral.CST.Children[0].Span.Slice(astral.Source)); got != "𝑥" {
		t.Fatalf("first astral CST token = %q", got)
	}

	code, err := LoadCorpus(root, specs["typlet-code"])
	if err != nil {
		t.Fatal(err)
	}
	unknown, ok := findCase(code, "call-unknown")
	if !ok {
		t.Fatal("imported call-unknown case not found")
	}
	if len(unknown.Diagnostics) != 1 || !unknown.Diagnostics[0].HasSpan {
		t.Fatalf("call-unknown diagnostics = %#v", unknown.Diagnostics)
	}
	if got := string(unknown.Diagnostics[0].Span.Slice(unknown.Source)); got != "foo" {
		t.Fatalf("call-unknown diagnostic span = %q", got)
	}

	paired, err := LoadCorpus(root, specs["typlet-paired"])
	if err != nil {
		t.Fatal(err)
	}
	mathFont, ok := findCase(paired, "math-font")
	if !ok {
		t.Fatal("imported math-font case not found")
	}
	if len(mathFont.Diagnostics) != 1 || mathFont.Diagnostics[0].At != "preamble" {
		t.Fatalf("math-font diagnostics = %#v", mathFont.Diagnostics)
	}
	if got := string(mathFont.Diagnostics[0].Span.Slice(mathFont.Preamble)); got != `"Cambria Math"` {
		t.Fatalf("preamble diagnostic span = %q", got)
	}
}

func findCase(corpus *Corpus, id string) (Case, bool) {
	for _, testCase := range corpus.Cases {
		if testCase.ID == id {
			return testCase, true
		}
	}
	return Case{}, false
}
