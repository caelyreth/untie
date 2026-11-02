package crosscheck

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/caelyreth/untie"
	"github.com/caelyreth/untie/display"
	"github.com/caelyreth/untie/ir"
)

func TestPairedConformanceBaseline(t *testing.T) {
	root, baseline := crosscheckPaths(t)
	report, err := Verify(context.Background(), Options{
		FixtureRoot: root, KnownDifferences: baseline,
	})
	if err != nil {
		t.Fatal(err)
	}
	want, err := LoadReport(baseline)
	if err != nil {
		t.Fatal(err)
	}
	if err := CompareReports(want, report); err != nil {
		t.Fatal(err)
	}
	if report.Unexplained != 0 {
		t.Fatalf("Unexplained = %d", report.Unexplained)
	}
}

func TestBaselineCoversCorpusExactly(t *testing.T) {
	root, baseline := crosscheckPaths(t)
	corpus, err := LoadPairedCorpus(root)
	if err != nil {
		t.Fatal(err)
	}
	report, err := LoadReport(baseline)
	if err != nil {
		t.Fatal(err)
	}
	if len(corpus.Cases) != len(report.Cases) {
		t.Fatalf("corpus has %d cases, baseline has %d", len(corpus.Cases), len(report.Cases))
	}
	for i, testCase := range corpus.Cases {
		result := report.Cases[i]
		if result.ID != testCase.ID || result.Category != testCase.Category {
			t.Fatalf("case %d: baseline %q/%q, corpus %q/%q",
				i+1, result.ID, result.Category, testCase.ID, testCase.Category)
		}
		if result.Kind != Pass && result.KnownReason == "" {
			t.Fatalf("case %q has no reason", result.ID)
		}
	}
}

func TestSupplementaryPlaneSpansAndLayout(t *testing.T) {
	root, _ := crosscheckPaths(t)
	harness := testHarness(t, root)
	src := "𝑥+𝑦^2"

	typstExpr := untie.ParseTypst(src, ir.Inline, untie.ParseOptions{})
	texExpr := untie.ParseTeX(src, ir.Inline, untie.ParseOptions{})
	typstRoot := requireValidRoot(t, typstExpr)
	texRoot := requireValidRoot(t, texExpr)
	for name, formula := range map[string]*ir.Formula{"Typst": typstRoot, "TeX": texRoot} {
		found := false
		ir.Walk(formula.Root, func(node ir.Node) bool {
			glyph, ok := node.(*ir.Glyph)
			if ok && glyph.Text == "𝑥" {
				found = true
				if glyph.Span.Len() != len("𝑥") {
					t.Errorf("%s astral span length = %d, want %d", name, glyph.Span.Len(), len("𝑥"))
				}
			}
			return true
		})
		if !found {
			t.Errorf("%s root has no supplementary-plane glyph", name)
		}
	}

	typstSemantic, err := normalizeFormula(typstRoot, ir.Inline)
	if err != nil {
		t.Fatal(err)
	}
	texSemantic, err := normalizeFormula(texRoot, ir.Inline)
	if err != nil {
		t.Fatal(err)
	}
	if difference := CompareSemantic(texSemantic, typstSemantic); difference != nil {
		t.Fatal(difference)
	}
	typstBox, err := harness.layout(t.Context(), typstRoot.Root, ir.Inline, 16)
	if err != nil {
		t.Fatal(err)
	}
	texBox, err := harness.layout(t.Context(), texRoot.Root, ir.Inline, 16)
	if err != nil {
		t.Fatal(err)
	}
	if difference := CompareOutputs(texBox, typstBox); difference != "" {
		t.Fatal(difference)
	}
}

func TestRepeatedGlyphRun(t *testing.T) {
	root, _ := crosscheckPaths(t)
	harness := testHarness(t, root)
	src := strings.Repeat("x ", 127) + "x"

	typstRoot := requireValidRoot(t, untie.ParseTypst(src, ir.Inline, untie.ParseOptions{}))
	texRoot := requireValidRoot(t, untie.ParseTeX(src, ir.Inline, untie.ParseOptions{}))
	typstSemantic, err := normalizeFormula(typstRoot, ir.Inline)
	if err != nil {
		t.Fatal(err)
	}
	texSemantic, err := normalizeFormula(texRoot, ir.Inline)
	if err != nil {
		t.Fatal(err)
	}
	if difference := CompareSemantic(texSemantic, typstSemantic); difference != nil {
		t.Fatal(difference)
	}
	typstBox, err := harness.layout(t.Context(), typstRoot.Root, ir.Inline, 16)
	if err != nil {
		t.Fatal(err)
	}
	texBox, err := harness.layout(t.Context(), texRoot.Root, ir.Inline, 16)
	if err != nil {
		t.Fatal(err)
	}
	if difference := CompareOutputs(texBox, typstBox); difference != "" {
		t.Fatal(difference)
	}
	if got := countGlyphItems(typstBox); got != 128 {
		t.Fatalf("Typst glyph item count = %d, want 128", got)
	}
	if got := countGlyphItems(texBox); got != 128 {
		t.Fatalf("TeX glyph item count = %d, want 128", got)
	}
}

func TestSemanticNormalizationRejectsMaterialDifference(t *testing.T) {
	superscript := requireValidRoot(t, untie.ParseTeX(`x^2`, ir.Inline, untie.ParseOptions{}))
	subscript := requireValidRoot(t, untie.ParseTeX(`x_2`, ir.Inline, untie.ParseOptions{}))
	left, err := normalizeFormula(superscript, ir.Inline)
	if err != nil {
		t.Fatal(err)
	}
	right, err := normalizeFormula(subscript, ir.Inline)
	if err != nil {
		t.Fatal(err)
	}
	if CompareSemantic(left, right) == nil {
		t.Fatal("superscript and subscript normalized as equal")
	}
}

func TestSemanticNormalizationPreservesLetterVariants(t *testing.T) {
	italic := requireValidRoot(t, untie.ParseTeX(`\mathit{x}`, ir.Inline, untie.ParseOptions{}))
	upright := requireValidRoot(t, untie.ParseTeX(`\mathrm{x}`, ir.Inline, untie.ParseOptions{}))
	left, err := normalizeFormula(italic, ir.Inline)
	if err != nil {
		t.Fatal(err)
	}
	right, err := normalizeFormula(upright, ir.Inline)
	if err != nil {
		t.Fatal(err)
	}
	if CompareSemantic(left, right) == nil {
		t.Fatal("italic and upright letters normalized as equal")
	}
}

func TestOutputComparisonUsesFixtureTolerance(t *testing.T) {
	want := &display.Box{Width: 10, Ascent: 4, Descent: 1}
	within := &display.Box{Width: 10 + toleranceDIP*0.9, Ascent: 4, Descent: 1}
	outside := &display.Box{Width: 10 + toleranceDIP*1.1, Ascent: 4, Descent: 1}
	if difference := CompareOutputs(within, want); difference != "" {
		t.Fatalf("within tolerance: %s", difference)
	}
	if difference := CompareOutputs(outside, want); difference == "" {
		t.Fatal("outside tolerance compared equal")
	}
}

func TestReportRejectsUnexplainedAndMissingReasons(t *testing.T) {
	report := Report{
		SchemaVersion: 1, Corpus: "test", ToleranceDIP: toleranceDIP, Total: 1,
		Unexplained: 1,
		Cases:       []CaseResult{{ID: "x", Index: 1, Kind: Unexplained}},
	}
	if err := report.Validate(); err == nil {
		t.Fatal("Validate accepted an unexplained case without a reason")
	}
	report.Cases[0].Kind = LayoutMismatch
	if err := report.Validate(); err == nil {
		t.Fatal("Validate accepted a layout mismatch without a reason")
	}
	report.Cases[0].KnownReason = "documented engine limitation"
	if err := report.Validate(); err == nil {
		t.Fatal("Validate accepted a nonzero unexplained count")
	}
	report.Unexplained = 0
	report.Layout = 1
	if err := report.Validate(); err != nil {
		t.Fatal(err)
	}
}

func crosscheckPaths(t *testing.T) (string, string) {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	pkg := filepath.Dir(file)
	return filepath.Join(pkg, "..", "..", "testdata", "upstream"),
		filepath.Join(pkg, "testdata", "paired-baseline.json")
}

func testHarness(t *testing.T, fixtureRoot string) *layoutHarness {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(fixtureRoot, "fonts", "NewCMMath-Book.otf"))
	if err != nil {
		t.Fatal(err)
	}
	harness, err := newLayoutHarness(data)
	if err != nil {
		t.Fatal(err)
	}
	return harness
}

func requireValidRoot(t *testing.T, expr *untie.Expr) *ir.Formula {
	t.Helper()
	for _, diagnostic := range expr.Diagnostics() {
		if diagnostic.Severity == untie.SeverityError {
			t.Fatalf("parse diagnostic: %v", diagnostic)
		}
	}
	root, err := expr.RootSnapshot()
	if err != nil {
		t.Fatal(err)
	}
	return root
}

func countGlyphItems(box *display.Box) int {
	count := 0
	for _, item := range box.Items {
		if item.Kind == display.ItemGlyphPath {
			count++
		}
	}
	return count
}
