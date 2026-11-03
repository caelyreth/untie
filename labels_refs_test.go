package untie

import (
	"context"
	"strings"
	"testing"

	"github.com/caelyreth/untie/ir"
	"github.com/caelyreth/untie/source"
)

func TestParseSnapshotsReferencesAndExposesFormulaLabels(t *testing.T) {
	t.Parallel()

	references := map[string]string{"eq:a": "3"}
	expr := ParseTeX(`a+\label{eq:a}\eqref{eq:a}`, Inline, ParseOptions{References: references})
	references["eq:a"] = "changed"

	labels := expr.Labels()
	if len(labels) != 1 || labels[0].Name != "eq:a" {
		t.Fatalf("Labels() = %+v, want eq:a", labels)
	}
	labels[0].Name = "mutated"
	if expr.Labels()[0].Name != "eq:a" {
		t.Fatal("Labels() returned shared storage")
	}
	snapshot, err := expr.RootSnapshot()
	if err != nil {
		t.Fatalf("RootSnapshot() error = %v", err)
	}
	if len(snapshot.Labels) != 1 || snapshot.Labels[0].Name != "eq:a" {
		t.Fatalf("snapshot labels = %+v", snapshot.Labels)
	}
	if alt := expr.Alt().Text; !strings.Contains(alt, "(3)") {
		t.Fatalf("Alt().Text = %q, want resolved reference and no label metadata", alt)
	} else if strings.Contains(alt, "eq:a") {
		t.Fatalf("Alt().Text leaked hidden label metadata: %q", alt)
	}
}

func TestParseKeyIncludesTeXReferencesOnly(t *testing.T) {
	t.Parallel()

	texA := ParseTeX(`\eqref{a}`, Inline, ParseOptions{References: map[string]string{"a": "1"}})
	texB := ParseTeX(`\eqref{a}`, Inline, ParseOptions{References: map[string]string{"a": "2"}})
	if texA.Key() == texB.Key() {
		t.Fatal("TeX expression key ignored reference text")
	}
	orderedA := ParseTeX(`\eqref{a}\eqref{b}`, Inline, ParseOptions{References: map[string]string{
		"a": "1",
		"b": "2",
	}})
	orderedB := ParseTeX(`\eqref{a}\eqref{b}`, Inline, ParseOptions{References: map[string]string{
		"b": "2",
		"a": "1",
	}})
	if orderedA.Key() != orderedB.Key() {
		t.Fatal("TeX expression key depends on reference map insertion order")
	}
	typstA := ParseTypst(`x`, Inline, ParseOptions{References: map[string]string{"a": "1"}})
	typstB := ParseTypst(`x`, Inline, ParseOptions{References: map[string]string{"a": "2"}})
	if typstA.Key() != typstB.Key() {
		t.Fatal("Typst expression key includes ignored TeX references")
	}
}

func TestPublicUnresolvedReferenceRecovery(t *testing.T) {
	t.Parallel()

	expr := ParseTeX(`a+\eqref{missing}+b`, Inline, ParseOptions{})
	if !expr.HasErrors() {
		t.Fatal("unresolved reference produced no error diagnostic")
	}
	found := false
	start := strings.Index(expr.Source(), `\eqref{missing}`)
	wantRange := source.NewRange(start, start+len(`\eqref{missing}`))
	for _, diagnostic := range expr.Diagnostics() {
		if diagnostic.Code == CodeUnresolvedReference {
			found = true
			if diagnostic.Range != wantRange {
				t.Fatalf("diagnostic range = %v", diagnostic.Range)
			}
		}
	}
	if !found {
		t.Fatalf("diagnostics = %+v, want unresolved-reference", expr.Diagnostics())
	}
	placeholder := false
	ir.Walk(mustRoot(t, expr).Root, func(node ir.Node) bool {
		if _, ok := node.(*ir.Placeholder); ok {
			placeholder = true
		}
		return true
	})
	if !placeholder {
		t.Fatal("unresolved reference produced no visible placeholder")
	}
	if alt := expr.Alt().Text; !strings.Contains(alt, "unresolved equation reference") {
		t.Fatalf("Alt().Text = %q, want unresolved-reference marker", alt)
	}
}

func TestRendererReferenceCacheProducesReferenceSpecificBoxes(t *testing.T) {
	t.Parallel()

	renderer, err := NewRenderer(loadTestFonts(t), RendererOptions{CacheEntries: 4})
	if err != nil {
		t.Fatalf("NewRenderer() error = %v", err)
	}
	render := func(reference string) *RenderResult {
		result, err := renderer.Render(context.Background(), `\eqref{eq:a}`, RenderRequest{
			Style: Style{Size: 16},
			Parse: ParseOptions{References: map[string]string{"eq:a": reference}},
		})
		if err != nil {
			t.Fatalf("Render(%q) error = %v", reference, err)
		}
		if result.Box == nil {
			t.Fatalf("Render(%q) Box = nil", reference)
		}
		return result
	}
	first := render("1")
	second := render("2")
	firstJSON, err := first.Box.MarshalCanonical()
	if err != nil {
		t.Fatalf("first MarshalCanonical() error = %v", err)
	}
	secondJSON, err := second.Box.MarshalCanonical()
	if err != nil {
		t.Fatalf("second MarshalCanonical() error = %v", err)
	}
	if string(firstJSON) == string(secondJSON) {
		t.Fatal("renderer cache reused output across different reference text")
	}
}

func TestRendererParsesReferencesAtRenderTime(t *testing.T) {
	t.Parallel()

	renderer, err := NewRenderer(loadTestFonts(t), RendererOptions{CacheEntries: 2})
	if err != nil {
		t.Fatalf("NewRenderer() error = %v", err)
	}
	references := map[string]string{"eq:a": "1"}
	result, err := renderer.Render(context.Background(), `\eqref{eq:a}`, RenderRequest{
		Style: Style{Size: 16},
		Parse: ParseOptions{References: references},
	})
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}
	references["eq:a"] = "9"
	if len(result.Diagnostics) != 0 {
		t.Fatalf("initial diagnostics = %+v", result.Diagnostics)
	}
	again, err := renderer.Render(context.Background(), `\eqref{eq:a}`, RenderRequest{
		Style: Style{Size: 16},
		Parse: ParseOptions{References: references},
	})
	if err != nil {
		t.Fatalf("Render(second) error = %v", err)
	}
	firstJSON, _ := result.Box.MarshalCanonical()
	secondJSON, _ := again.Box.MarshalCanonical()
	if string(firstJSON) == string(secondJSON) {
		t.Fatal("renderer reused an expression after the host changed reference text")
	}
}

func TestReferenceCacheKeyIsStableAndComplete(t *testing.T) {
	t.Parallel()

	a := makeRenderExprCacheKey(`\eqref{a}`, TeX, Inline, ParseOptions{
		References: map[string]string{"a": "1", "b": "2"},
	})
	b := makeRenderExprCacheKey(`\eqref{a}`, TeX, Inline, ParseOptions{
		References: map[string]string{"b": "2", "a": "1"},
	})
	if a != b {
		t.Fatal("reference cache key depends on map insertion order")
	}
	changed := makeRenderExprCacheKey(`\eqref{a}`, TeX, Inline, ParseOptions{
		References: map[string]string{"a": "9", "b": "2"},
	})
	if a == changed {
		t.Fatal("reference cache key ignored reference text")
	}
	if makeRenderExprCacheKey("x", Typst, Inline, ParseOptions{
		References: map[string]string{"a": "1"},
	}) != makeRenderExprCacheKey("x", Typst, Inline, ParseOptions{
		References: map[string]string{"a": "2"},
	}) {
		t.Fatal("Typst cache key includes ignored TeX references")
	}
}
