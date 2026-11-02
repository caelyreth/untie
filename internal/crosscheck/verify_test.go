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
	"github.com/caelyreth/untie/source"
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

func TestSemanticNormalizationHarmonizesNeutralEncodings(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		tex   string
		typst string
	}{
		{"operator class", `\operatorname{tr}(A)`, `op("tr")(A)`},
		{"integral limits", `\int_0^1`, `integral_0^1`},
		{"symmetric fences", `\lfloor x \rfloor`, `floor(x)`},
	}
	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			texRoot := requireValidRoot(t, untie.ParseTeX(tc.tex, ir.Inline, untie.ParseOptions{}))
			typstRoot := requireValidRoot(t, untie.ParseTypst(tc.typst, ir.Inline, untie.ParseOptions{}))
			texSemantic, err := normalizeFormula(texRoot, ir.Inline)
			if err != nil {
				t.Fatal(err)
			}
			typstSemantic, err := normalizeTypstFormula(typstRoot, ir.Inline)
			if err != nil {
				t.Fatal(err)
			}
			texSemantic = normalizedRootItems(texSemantic)
			typstSemantic = normalizedRootItems(typstSemantic)
			if difference := CompareSemantic(texSemantic, typstSemantic); difference != nil {
				t.Fatal(difference)
			}
		})
	}
}

func TestSemanticNormalizationFractionDefaults(t *testing.T) {
	t.Parallel()

	glyph := func(text string) *ir.Glyph {
		return ir.NewGlyph(source.NewRange(0, 1), text)
	}
	implicit := ir.NewFraction(source.NewRange(0, 1), glyph("a"), glyph("b"))
	implicit.Class = ir.ClassNormal
	explicit := ir.NewFraction(source.NewRange(0, 1), glyph("a"), glyph("b"))
	explicit.Class = ir.ClassNormal
	explicit.Padding = ir.RawLength(ir.LengthEm, 100_000)

	left, err := normalizeFormula(&ir.Formula{Root: implicit}, ir.Inline)
	if err != nil {
		t.Fatal(err)
	}
	right, err := normalizeFormula(&ir.Formula{Root: explicit}, ir.Inline)
	if err != nil {
		t.Fatal(err)
	}
	if difference := CompareSemantic(left, right); difference != nil {
		t.Fatal(difference)
	}
}

func TestSemanticNormalizationAccentMark(t *testing.T) {
	t.Parallel()

	span := source.NewRange(0, 1)
	accent := func(mark string) *ir.Accent {
		return ir.NewAccent(span, ir.NewGlyph(span, "x"), ir.NewGlyph(span, mark), ir.PositionAbove)
	}
	tex, err := normalizeFormula(&ir.Formula{Root: accent("^")}, ir.Inline)
	if err != nil {
		t.Fatal(err)
	}
	typst, err := normalizeFormula(&ir.Formula{Root: accent("\u0302")}, ir.Inline)
	if err != nil {
		t.Fatal(err)
	}
	if difference := CompareSemantic(tex, typst); difference != nil {
		t.Fatal(difference)
	}
	item := tex.(map[string]any)
	mark := item["accent"].(map[string]any)
	if mark["class"] != "diacritic" || mark["text"] != "\u0302" {
		t.Fatalf("accent mark = %#v, want canonical diacritic", mark)
	}
}

func TestSemanticNormalizationDefaultStroke(t *testing.T) {
	t.Parallel()

	span := source.NewRange(0, 1)
	implicit := ir.NewCancel(span, ir.NewGlyph(span, "x"))
	explicit := ir.NewCancel(span, ir.NewGlyph(span, "x"))
	explicit.Thickness = ir.RawLength(ir.LengthEm, 50_000)

	left, err := normalizeFormula(&ir.Formula{Root: implicit}, ir.Inline)
	if err != nil {
		t.Fatal(err)
	}
	right, err := normalizeFormula(&ir.Formula{Root: explicit}, ir.Inline)
	if err != nil {
		t.Fatal(err)
	}
	if difference := CompareSemantic(left, right); difference != nil {
		t.Fatal(difference)
	}
}

func TestSemanticNormalizationDeparenthesizesTypstFractionOperand(t *testing.T) {
	t.Parallel()

	span := source.NewRange(0, 1)
	fraction := ir.NewFraction(span, ir.NewGlyph(span, "b"), ir.NewGlyph(span, "c"))
	body := ir.NewRow(span, fraction, ir.NewGlyph(span, "+"), ir.NewGlyph(span, "d"))
	fenced := ir.NewFenced(span, body)
	fenced.Open = ir.NewGlyph(span, "(")
	fenced.Close = ir.NewGlyph(span, ")")
	fenced.Class = ir.ClassInner
	fenced.Balanced = true

	kept, err := normalizeFractionOperand(fenced, semanticMode{size: ir.SizeText})
	if err != nil {
		t.Fatal(err)
	}
	if item, ok := kept.(map[string]any); !ok || item["kind"] != "fenced" {
		t.Fatalf("non-Typst operand normalized to %#v, want fenced", kept)
	}

	stripped, err := normalizeFractionOperand(fenced, semanticMode{
		size:                          ir.SizeText,
		deparenthesizeFractionOperand: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	items, ok := stripped.([]any)
	if !ok || len(items) != 3 {
		t.Fatalf("Typst operand normalized to %#v, want three atoms", stripped)
	}
}

func normalizedRootItems(value any) any {
	if row, ok := value.(map[string]any); ok && row["kind"] == "row" {
		if items, ok := row["items"].([]any); ok {
			return items
		}
	}
	if _, ok := value.([]any); ok {
		return value
	}
	if value == nil {
		return value
	}
	return []any{value}
}

func TestSemanticNormalizationKeepsScalingFences(t *testing.T) {
	t.Parallel()

	root := requireValidRoot(t, untie.ParseTypst(`(a/b)`, ir.Inline, untie.ParseOptions{}))
	normalized, err := normalizeTypstFormula(root, ir.Inline)
	if err != nil {
		t.Fatal(err)
	}
	value := normalized
	for {
		switch current := value.(type) {
		case []any:
			if len(current) != 1 {
				goto found
			}
			value = current[0]
		case map[string]any:
			items, ok := current["items"].([]any)
			if current["kind"] != "row" || !ok || len(items) != 1 {
				goto found
			}
			value = items[0]
		default:
			goto found
		}
	}
found:
	item, ok := value.(map[string]any)
	if !ok || item["kind"] != "fenced" {
		t.Fatalf("scaling fence normalized to %#v, want a fenced node", value)
	}
}

func TestSemanticNormalizationKeepsInnerRowBoundary(t *testing.T) {
	t.Parallel()

	span := source.NewRange(0, 1)
	glyph := ir.NewGlyph(span, "x")
	normal := ir.NewRow(span, glyph)
	inner := ir.NewRow(span, ir.NewGlyph(span, "x"))
	inner.Class = ir.ClassInner

	normalValue, err := normalizeFormula(&ir.Formula{Root: normal}, ir.Inline)
	if err != nil {
		t.Fatal(err)
	}
	innerValue, err := normalizeFormula(&ir.Formula{Root: inner}, ir.Inline)
	if err != nil {
		t.Fatal(err)
	}
	row, ok := innerValue.(map[string]any)
	if !ok || row["kind"] != "row" {
		t.Fatalf("inner row normalized to %#v, want a row node", innerValue)
	}
	if difference := CompareSemantic(normalValue, innerValue); difference == nil {
		t.Fatal("normal and inner row classes normalized as equal")
	}
}

func TestSemanticNormalizationPreservesExplicitLimitOverrides(t *testing.T) {
	t.Parallel()

	span := source.NewRange(0, 1)
	relationAuto := ir.NewGlyph(span, "→")
	relationAuto.Class = ir.ClassRelation
	relationAuto.Limits = ir.LimitsAuto
	relationNever := ir.NewGlyph(span, "→")
	relationNever.Class = ir.ClassRelation
	relationNever.Limits = ir.LimitsNever
	left, err := normalizeFormula(&ir.Formula{Root: relationAuto}, ir.Inline)
	if err != nil {
		t.Fatal(err)
	}
	right, err := normalizeFormula(&ir.Formula{Root: relationNever}, ir.Inline)
	if err != nil {
		t.Fatal(err)
	}
	if CompareSemantic(left, right) == nil {
		t.Fatal("explicit relation nolimits normalized as the default")
	}

	integralAuto := ir.NewGlyph(span, "∫")
	integralAuto.Class = ir.ClassLarge
	integralAuto.Limits = ir.LimitsAuto
	integralAlways := ir.NewGlyph(span, "∫")
	integralAlways.Class = ir.ClassLarge
	integralAlways.Limits = ir.LimitsAlways
	left, err = normalizeFormula(&ir.Formula{Root: integralAuto}, ir.Inline)
	if err != nil {
		t.Fatal(err)
	}
	right, err = normalizeFormula(&ir.Formula{Root: integralAlways}, ir.Inline)
	if err != nil {
		t.Fatal(err)
	}
	if CompareSemantic(left, right) == nil {
		t.Fatal("explicit integral limits normalized as the default")
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
