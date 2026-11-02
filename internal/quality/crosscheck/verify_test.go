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
	"github.com/caelyreth/untie/geometry"
	"github.com/caelyreth/untie/internal/quality/fixture"
	"github.com/caelyreth/untie/ir"
	"github.com/caelyreth/untie/source"
)

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

func TestFirstDifferenceReportsKeyPresence(t *testing.T) {
	t.Parallel()

	path, want, got := firstDifference("root",
		map[string]any{"amount": nil},
		map[string]any{},
	)
	if path != "root.amount" || want != nil || got != (missingValue{}) {
		t.Fatalf("missing field difference = %q, %#v, %#v", path, want, got)
	}
	path, want, got = firstDifference("root",
		map[string]any{},
		map[string]any{"amount": nil},
	)
	if path != "root.amount" || want != (missingValue{}) || got != nil {
		t.Fatalf("added field difference = %q, %#v, %#v", path, want, got)
	}
}

func TestSemanticNormalizationPreservesSemanticRoles(t *testing.T) {
	t.Parallel()

	span := source.NewRange(0, 1)
	glyph := func(text string) *ir.Glyph {
		return ir.NewGlyph(span, text)
	}
	fractionDefault := ir.NewFraction(span, glyph("a"), glyph("b"))
	fractionStacked := ir.NewFraction(span, glyph("a"), glyph("b"))
	fractionStacked.Style = ir.FractionNoBar
	fractionStacked.Bar = false
	fractionStacked.Role = ir.FractionRoleStacked

	scriptsDefault := ir.NewScripts(span, glyph("x"))
	scriptsDefault.Top = glyph("2")
	scriptsLimits := ir.NewScripts(span, glyph("x"))
	scriptsLimits.Top = glyph("2")
	scriptsLimits.Role = ir.ScriptRoleLimits

	cancelDefault := ir.NewCancel(span, glyph("="))
	cancelNegation := ir.NewCancel(span, glyph("="))
	cancelNegation.Role = ir.CancelRoleNegation

	tableDefault := ir.NewTable(span, ir.TableRow{Cells: []ir.Node{glyph("x")}})
	tableCases := ir.NewTable(span, ir.TableRow{Cells: []ir.Node{glyph("x")}})
	tableCases.Role = ir.TableRoleCases

	tier1 := ir.AbsoluteDelimiterSizing(ir.RawLength(ir.LengthEm, 1_200_000))
	tier2 := ir.AbsoluteDelimiterSizing(ir.RawLength(ir.LengthEm, 1_800_000))
	sizedDefault := ir.NewSizedDelimiter(span, "(", tier1)
	sizedDefault.Class = ir.ClassOpening
	sizedLarger := ir.NewSizedDelimiter(span, "(", tier2)
	sizedLarger.Class = ir.ClassOpening

	tests := []struct {
		name     string
		original ir.Node
		changed  ir.Node
	}{
		{"fraction", fractionDefault, fractionStacked},
		{"scripts", scriptsDefault, scriptsLimits},
		{"cancel", cancelDefault, cancelNegation},
		{"table", tableDefault, tableCases},
		{"sized delimiter", sizedDefault, sizedLarger},
	}
	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			left, err := normalizeFormula(&ir.Formula{Root: tc.original}, ir.Inline)
			if err != nil {
				t.Fatal(err)
			}
			right, err := normalizeFormula(&ir.Formula{Root: tc.changed}, ir.Inline)
			if err != nil {
				t.Fatal(err)
			}
			if CompareSemantic(left, right) == nil {
				t.Fatalf("%s semantic role difference normalized as equal", tc.name)
			}
		})
	}
}

func TestSemanticNormalizationMatchesNeutralSizedDelimiterForms(t *testing.T) {
	t.Parallel()

	span := source.NewRange(0, 7)
	sizing := ir.AbsoluteDelimiterSizing(ir.RawLength(ir.LengthEm, 1_800_000))
	texForm := ir.NewSizedDelimiter(span, "(", sizing)
	texForm.Class = ir.ClassOpening
	typstNeutralForm := ir.NewSizedDelimiter(span, "(", sizing)
	typstNeutralForm.Class = ir.ClassOpening

	texValue, err := normalizeFormula(&ir.Formula{Root: texForm}, ir.Inline)
	if err != nil {
		t.Fatal(err)
	}
	typstValue, err := normalizeFormula(&ir.Formula{Root: typstNeutralForm}, ir.Inline)
	if err != nil {
		t.Fatal(err)
	}
	if difference := CompareSemantic(texValue, typstValue); difference != nil {
		t.Fatalf("equivalent neutral sized delimiters differ: %v", difference)
	}
}

func TestSemanticNormalizationIgnoresUnobservableSingleColumnGap(t *testing.T) {
	t.Parallel()

	span := source.NewRange(0, 1)
	makeTable := func(gap ir.Length) *ir.Table {
		table := ir.NewTable(span, ir.TableRow{Cells: []ir.Node{
			ir.NewGlyph(span, "x"),
		}})
		table.Role = ir.TableRoleVector
		table.ColumnGap = gap
		return table
	}
	withGap, err := normalizeFormula(&ir.Formula{Root: makeTable(ir.RawLength(ir.LengthEm, 500_000))}, ir.Inline)
	if err != nil {
		t.Fatal(err)
	}
	withoutGap, err := normalizeFormula(&ir.Formula{Root: makeTable(ir.Length{})}, ir.Inline)
	if err != nil {
		t.Fatal(err)
	}
	if difference := CompareSemantic(withGap, withoutGap); difference != nil {
		t.Fatalf("single-column gap changed semantics: %v", difference)
	}

	twoColumns := ir.NewTable(span, ir.TableRow{Cells: []ir.Node{
		ir.NewGlyph(span, "x"),
		ir.NewGlyph(span, "y"),
	}})
	twoColumnsWithGap := *twoColumns
	twoColumnsWithGap.ColumnGap = ir.RawLength(ir.LengthEm, 500_000)
	left, err := normalizeFormula(&ir.Formula{Root: twoColumns}, ir.Inline)
	if err != nil {
		t.Fatal(err)
	}
	right, err := normalizeFormula(&ir.Formula{Root: &twoColumnsWithGap}, ir.Inline)
	if err != nil {
		t.Fatal(err)
	}
	if difference := CompareSemantic(left, right); difference == nil {
		t.Fatal("multi-column gap difference was normalized away")
	}

	matrix := makeTable(ir.Length{})
	matrix.Role = ir.TableRoleMatrix
	vector := makeTable(ir.Length{})
	vector.Role = ir.TableRoleVector
	matrixValue, err := normalizeFormula(&ir.Formula{Root: matrix}, ir.Inline)
	if err != nil {
		t.Fatal(err)
	}
	vectorValue, err := normalizeFormula(&ir.Formula{Root: vector}, ir.Inline)
	if err != nil {
		t.Fatal(err)
	}
	if difference := CompareSemantic(matrixValue, vectorValue); difference != nil {
		t.Fatalf("one-column matrix/vector roles are not equivalent: %v", difference)
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
		{"integral limits", `\int_0^1`, `integral_0^1`},
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
			typstSemantic, err := normalizeFormula(typstRoot, ir.Inline)
			if err != nil {
				t.Fatal(err)
			}
			if difference := CompareSemantic(texSemantic, typstSemantic); difference != nil {
				t.Fatal(difference)
			}
		})
	}
}

func TestSemanticNormalizationKeepsExplicitFractionPadding(t *testing.T) {
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
	if difference := CompareSemantic(left, right); difference == nil {
		t.Fatal("omitted fraction padding normalized as explicit 0.1em")
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
	if mark["text"] != "\u0302" {
		t.Fatalf("accent mark = %#v, want canonical glyph", mark)
	}
	if mark["class"] != "normal" {
		t.Fatalf("accent canonicalization invented metadata: %#v", mark)
	}

	tilde, err := normalizeFormula(&ir.Formula{Root: accent("~")}, ir.Inline)
	if err != nil {
		t.Fatal(err)
	}
	if CompareSemantic(tex, tilde) == nil {
		t.Fatal("different accent glyph encodings normalized as equal")
	}
}

func TestSemanticNormalizationKeepsExplicitStroke(t *testing.T) {
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
	if difference := CompareSemantic(left, right); difference == nil {
		t.Fatal("omitted stroke thickness normalized as explicit 0.05em")
	}
}

func TestSemanticNormalizationKeepsFractionOperandFence(t *testing.T) {
	t.Parallel()

	span := source.NewRange(0, 1)
	bodyFraction := ir.NewFraction(span, ir.NewGlyph(span, "b"), ir.NewGlyph(span, "c"))
	body := ir.NewRow(span, bodyFraction, ir.NewGlyph(span, "+"), ir.NewGlyph(span, "d"))
	fenced := ir.NewFenced(span, body)
	fenced.Open = ir.NewGlyph(span, "(")
	fenced.Close = ir.NewGlyph(span, ")")
	fenced.Class = ir.ClassInner
	fenced.Balanced = true

	normalized, err := normalizeFormula(&ir.Formula{Root: ir.NewFraction(span, ir.NewGlyph(span, "a"), fenced)}, ir.Inline)
	if err != nil {
		t.Fatal(err)
	}
	fractionValue, ok := normalized.(map[string]any)
	if !ok {
		t.Fatalf("normalized fraction = %#v", normalized)
	}
	denominator, ok := fractionValue["denominator"].(map[string]any)
	if !ok || denominator["kind"] != "fenced" {
		t.Fatalf("fraction denominator normalized to %#v, want fenced", fractionValue["denominator"])
	}
}

func TestSemanticNormalizationKeepsLiteralParentheses(t *testing.T) {
	t.Parallel()

	root := requireValidRoot(t, untie.ParseTypst(`(a/b)`, ir.Inline, untie.ParseOptions{}))
	normalized, err := normalizeFormula(root, ir.Inline)
	if err != nil {
		t.Fatal(err)
	}
	items, ok := normalized.([]any)
	if !ok || len(items) != 3 {
		t.Fatalf("literal parentheses normalized to %#v, want opening/fraction/closing", normalized)
	}
	open, _ := items[0].(map[string]any)
	closeItem, _ := items[2].(map[string]any)
	if open["text"] != "(" || closeItem["text"] != ")" {
		t.Fatalf("literal parentheses = %#v/%#v", items[0], items[2])
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

func TestSemanticDifferenceDistinguishesAbsentFields(t *testing.T) {
	t.Parallel()

	difference := CompareSemantic(
		map[string]any{"space": map[string]any{"type": "interword"}},
		map[string]any{"space": map[string]any{"type": "interword", "amount": nil}},
	)
	if difference == nil || !strings.Contains(difference.String(), `expected <absent>, got null`) {
		t.Fatalf("semantic difference = %v, want absent field", difference)
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

func TestCompareOutputsComparesRulesPathsAndPaint(t *testing.T) {
	t.Parallel()

	base := &display.Box{
		Width: 10, Ascent: 8, Descent: 2,
		Items: []display.Item{{
			Kind:  display.ItemRule,
			Paint: display.Paint{Role: display.RoleInk},
			Bound: geometry.Rect{X: 1, Y: 2, Width: 8, Height: 1},
			Rect:  geometry.Rect{X: 1, Y: 2, Width: 8, Height: 1},
		}},
	}
	changedRule := base.Clone()
	changedRule.Items[0].Rect.Y = 3
	changedRule.Items[0].Bound.Y = 3
	if difference := CompareOutputs(changedRule, base); difference == "" {
		t.Fatal("CompareOutputs() ignored a rule geometry difference")
	}
	changedPaint := base.Clone()
	changedPaint.Items[0].Paint.Role = display.RoleError
	if difference := CompareOutputs(changedPaint, base); difference == "" {
		t.Fatal("CompareOutputs() ignored a paint difference")
	}

	path := &display.Box{
		Width: 10, Ascent: 8, Descent: 2,
		Items: []display.Item{{
			Kind:  display.ItemGlyphPath,
			Paint: display.Paint{Role: display.RoleInk},
			Bound: geometry.Rect{Width: 10, Height: 10},
			Path: geometry.Path{{
				Op: geometry.SegLineTo,
				Points: [3]geometry.Point{
					{X: 10, Y: 2},
				},
			}},
		}},
	}
	changedPath := path.Clone()
	changedPath.Items[0].Path[0].Points[0].X = 9
	if difference := CompareOutputs(changedPath, path); difference == "" {
		t.Fatal("CompareOutputs() ignored a path geometry difference")
	}
}

func TestFixtureComparisonIgnoresOptionalAndNonGlyphFields(t *testing.T) {
	t.Parallel()

	width, ascent, descent := 7.5, 3.0, 0.75
	frame := fixture.Frame{
		Box: &fixture.FrameBox{
			Anchor: "baseline", Width: &width, Ascent: &ascent, Descent: &descent,
		},
		FontSize: 16,
	}
	box := &display.Box{
		Width: 10, Ascent: 4, Descent: 1,
		AxisHeight: 3.6,
		Items: []display.Item{{
			Kind:  display.ItemGlyphPath,
			Paint: display.Paint{Role: display.RoleInk},
			Bound: geometry.Rect{Width: 1, Height: 1},
			Path: geometry.Path{{
				Op: geometry.SegMoveTo,
				Points: [3]geometry.Point{
					{X: 1, Y: 1},
				},
			}},
		}},
	}
	if difference := CompareOutputToFixture(box, frame); difference != "" {
		t.Fatalf("CompareOutputToFixture() = %q", difference)
	}
}

func TestTypstTableAugmentMatchesPinnedFixture(t *testing.T) {
	root, _ := crosscheckPaths(t)
	corpus, err := LoadPairedCorpus(root)
	if err != nil {
		t.Fatal(err)
	}
	var testCase fixture.Case
	for _, candidate := range corpus.Cases {
		if candidate.ID == "array-hline" {
			testCase = candidate
			break
		}
	}
	if testCase.ID == "" || testCase.Fixture == nil || testCase.Fixture.Frame == nil {
		t.Fatal("array-hline fixture is missing")
	}
	converted, err := fixture.FrameToDIP(*testCase.Fixture.Frame)
	if err != nil {
		t.Fatal(err)
	}
	expr := untie.ParseTypst(testCase.Source, ir.Display, untie.ParseOptions{})
	for _, diagnostic := range expr.Diagnostics() {
		if diagnostic.Severity == untie.SeverityError {
			t.Fatalf("Typst diagnostic = %s: %s", diagnostic.Code, diagnostic.Message)
		}
	}
	harness := testHarness(t, root)
	box, err := harness.layout(context.Background(), expr.Root().Root, ir.Display, converted.FontSize)
	if err != nil {
		t.Fatal(err)
	}
	if !near(float64(box.Width), *converted.Box.Width) ||
		!near(float64(box.Ascent+box.Descent), *converted.Box.Height) {
		t.Fatalf("table box = %vx%v, fixture = %vx%v",
			box.Width, box.Ascent+box.Descent, *converted.Box.Width, *converted.Box.Height)
	}
	lines := make([][4]float64, 0, len(converted.Shapes))
	thickness := make([]float64, 0, len(converted.Shapes))
	for _, shape := range converted.Shapes {
		if len(shape.Line) != 4 || shape.Stroke == nil {
			t.Fatalf("fixture shape = %#v, want stroked line", shape)
		}
		lines = append(lines, [4]float64{
			shape.Line[0], shape.Line[1], shape.Line[2], shape.Line[3],
		})
		thickness = append(thickness, shape.Stroke.Thickness)
	}
	rules := make([]display.Item, 0)
	for _, item := range box.Items {
		if item.Kind == display.ItemRule {
			rules = append(rules, item)
		}
	}
	if len(rules) != len(lines) {
		t.Fatalf("renderer rules = %d, fixture lines = %d", len(rules), len(lines))
	}
	used := make([]bool, len(rules))
	for i, want := range lines {
		matched := false
		for j, rule := range rules {
			if used[j] {
				continue
			}
			got, gotThickness := ruleLine(rule, box.Ascent)
			if near(got[0], want[0]) && near(got[1], want[1]) &&
				near(got[2], want[2]) && near(got[3], want[3]) &&
				near(gotThickness, thickness[i]) {
				used[j] = true
				matched = true
				break
			}
		}
		if !matched {
			t.Fatalf("fixture line %v thickness %v has no matching renderer rule: %#v",
				want, thickness[i], rules)
		}
	}
}

func ruleLine(item display.Item, ascent float32) ([4]float64, float64) {
	rect := item.Rect
	if rect.Width <= rect.Height {
		x := float64(rect.X + rect.Width/2)
		return [4]float64{
			x, float64(ascent + rect.Y),
			x, float64(ascent + rect.Y + rect.Height),
		}, float64(rect.Width)
	}
	y := float64(ascent + rect.Y + rect.Height/2)
	return [4]float64{
		float64(rect.X), y,
		float64(rect.X + rect.Width), y,
	}, float64(rect.Height)
}

func TestReviewedRefreshRequiresReasonsOnlyForKindChanges(t *testing.T) {
	t.Parallel()

	previous := Report{
		SchemaVersion: 1,
		Corpus:        "paired",
		ToleranceDIP:  toleranceDIP,
		Total:         2,
		SemanticIR:    2,
		Cases: []CaseResult{
			{ID: "same", Index: 1, Category: "test", Kind: SemanticIRMismatch, Detail: "old", KnownReason: "old reason"},
			{ID: "moved", Index: 2, Category: "test", Kind: SemanticIRMismatch, Detail: "old", KnownReason: "old reason"},
		},
	}
	observed := []CaseResult{
		{ID: "same", Category: "test", Kind: SemanticIRMismatch, Detail: "new"},
		{ID: "moved", Category: "test", Kind: LayoutMismatch, Detail: "new"},
	}
	if _, err := ReviewedRefresh(observed, previous, nil); err == nil {
		t.Fatal("ReviewedRefresh accepted a kind change without a reason")
	}
	report, err := ReviewedRefresh(observed, previous, map[string]string{
		"moved": "reviewed layout difference",
	})
	if err != nil {
		t.Fatal(err)
	}
	if report.Cases[0].KnownReason != "old reason" || report.Cases[1].KnownReason != "reviewed layout difference" {
		t.Fatalf("refreshed cases = %#v", report.Cases)
	}
	if _, err := ReviewedRefresh(observed, previous, map[string]string{
		"moved":         "reviewed layout difference",
		"unused-reason": "unused",
	}); err == nil {
		t.Fatal("ReviewedRefresh accepted an unused reason")
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
	return filepath.Join(pkg, "..", "..", "..", "testdata", "upstream"),
		filepath.Join(pkg, "..", "..", "..", "testdata", "crosscheck", "paired-baseline.json")
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
