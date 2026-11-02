package typst

import (
	"testing"

	"github.com/caelyreth/untie/ir"
	"github.com/caelyreth/untie/source"
)

func TestModuleNamesAreResolvedInTheirScope(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		src      string
		code     source.Code
		wantSpan source.Range
	}{
		{"bare math module", `math.frac(1, 2, style: "skewed")`, source.CodeUnknownVariable, source.NewRange(0, 4)},
		{"bare sym module", `sym.arrow.r`, source.CodeUnknownVariable, source.NewRange(0, 3)},
		{"unknown math field", `#math.nope`, source.CodeUnknownVariable, source.NewRange(6, 10)},
		{"unknown code root", `#foo.bar`, source.CodeUnknownVariable, source.NewRange(1, 4)},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			result := parse(t, test.src)
			if len(result.Diagnostics) != 1 {
				t.Fatalf("diagnostics = %#v, want one", result.Diagnostics)
			}
			diagnostic := result.Diagnostics[0]
			if diagnostic.Code != test.code || diagnostic.Range != test.wantSpan {
				t.Fatalf("diagnostic = %#v, want %s at %#v", diagnostic, test.code, test.wantSpan)
			}
		})
	}

	for _, src := range []string{
		`#math.frac(1, 2, style: "skewed")`,
		`#std.math.frac(1, 2)`,
		`#sym.arrow.r`,
		`#std.sym.alpha`,
	} {
		result := parse(t, src)
		requireNoDiagnostics(t, src, result)
	}
}

func TestBareCodeKeywordsRemainUnknownInMathMode(t *testing.T) {
	t.Parallel()

	for _, name := range []string{"none", "auto", "true", "false", "rgb"} {
		result := parse(t, name)
		if len(result.Diagnostics) != 1 || result.Diagnostics[0].Code != source.CodeUnknownVariable {
			t.Fatalf("%s diagnostics = %#v, want unknown-variable", name, result.Diagnostics)
		}
	}

	for _, src := range []string{"#none", "#auto", "#true", "#false", `#rgb(255, 0, 0)`} {
		result := parse(t, src)
		requireNoDiagnostics(t, src, result)
	}
}

func TestCallableSymbolsFollowTypstWrapperRules(t *testing.T) {
	t.Parallel()

	for _, src := range []string{
		`paren.l(x)`,
		`bracket.l(x)`,
		`chevron.l(x)`,
		`bar.v(x)`,
		`bar.v.double(x)`,
		`angle.l(x)`,
	} {
		result := parse(t, src)
		requireNoDiagnostics(t, src, result)
		fenced := find[*ir.Fenced](t, result.Root)
		if fenced.Open == nil || fenced.Close == nil {
			t.Fatalf("%s fences = %#v", src, fenced)
		}
	}

	result := parse(t, `paren.r(x)`)
	requireNoDiagnostics(t, "closing delimiter call", result)
	if got := nodeText(result.Root); got != ")(x)" {
		t.Fatalf("closing delimiter call = %q, want non-function display", got)
	}
}

func TestAttachAndCasesPreserveNeutralMetadata(t *testing.T) {
	t.Parallel()

	tests := []struct {
		src    string
		class  ir.MathClass
		limits ir.LimitsPlacement
	}{
		{`attach(limits(sum), tl: b, bl: a, tr: d, br: c)`, ir.ClassLarge, ir.LimitsAlways},
		{`attach(limits(=), t: "def")`, ir.ClassRelation, ir.LimitsAlways},
		{`op("argmax", limits: #true)_x`, ir.ClassLarge, ir.LimitsAlways},
	}
	for _, test := range tests {
		result := parse(t, test.src)
		requireNoDiagnostics(t, test.src, result)
		scripts := find[*ir.Scripts](t, result.Root)
		if scripts.Class != test.class || scripts.Limits != test.limits {
			t.Fatalf("%s scripts metadata = class %v limits %v", test.src, scripts.Class, scripts.Limits)
		}
	}

	for _, test := range []struct {
		src   string
		open  string
		close string
	}{
		{`cases(a, b)`, "{", ""},
		{`cases(reverse: #true, a, b)`, "", "}"},
		{`cases(delim: "[", a, b)`, "[", ""},
		{`cases(reverse: #true, delim: "[", a, b)`, "", "]"},
	} {
		result := parse(t, test.src)
		requireNoDiagnostics(t, test.src, result)
		fenced := find[*ir.Fenced](t, result.Root)
		if nodeText(fenced.Open) != test.open || nodeText(fenced.Close) != test.close || fenced.Balanced {
			t.Fatalf("%s delimiters = %q/%q balanced=%v", test.src,
				nodeText(fenced.Open), nodeText(fenced.Close), fenced.Balanced)
		}
	}
}

func TestSemanticRolesComeFromTypstSyntax(t *testing.T) {
	t.Parallel()

	scriptTests := []struct {
		src  string
		role ir.ScriptRole
	}{
		{`x^2`, ir.ScriptRoleOrdinary},
		{`sin^2 x`, ir.ScriptRoleOrdinary},
		{`sum_(i=1)^n`, ir.ScriptRoleLimits},
		{`integral_0^1`, ir.ScriptRoleLimits},
		{`limits(sum)_(i=1)^n`, ir.ScriptRoleLimits},
		{`attach(x, t: a)`, ir.ScriptRoleOverlay},
		{`overbrace(x+y, n)`, ir.ScriptRoleBraceLabel},
		{`underbrace(x+y, n)`, ir.ScriptRoleBraceLabel},
	}
	for _, tc := range scriptTests {
		tc := tc
		t.Run(tc.src, func(t *testing.T) {
			t.Parallel()
			result := parse(t, tc.src)
			requireNoDiagnostics(t, tc.src, result)
			scripts := find[*ir.Scripts](t, result.Root)
			if scripts.Role != tc.role {
				t.Fatalf("role = %v, want %v", scripts.Role, tc.role)
			}
		})
	}

	binom := parse(t, `binom(n, k)`)
	requireNoDiagnostics(t, "binom", binom)
	fraction := find[*ir.Fraction](t, binom.Root)
	if fraction.Role != ir.FractionRoleBinomial {
		t.Fatalf("binom role = %v", fraction.Role)
	}

	tableTests := []struct {
		src  string
		role ir.TableRole
	}{
		{`mat(1, 2; 3, 4)`, ir.TableRoleMatrix},
		{`vec(1, 2, 3)`, ir.TableRoleVector},
		{`cases(a, b)`, ir.TableRoleCases},
	}
	for _, tc := range tableTests {
		tc := tc
		t.Run(tc.src, func(t *testing.T) {
			t.Parallel()
			result := parse(t, tc.src)
			requireNoDiagnostics(t, tc.src, result)
			table := find[*ir.Table](t, result.Root)
			if table.Role != tc.role {
				t.Fatalf("role = %v, want %v", table.Role, tc.role)
			}
		})
	}

	cancel := parse(t, `cancel(x)`)
	requireNoDiagnostics(t, "cancel", cancel)
	if got := find[*ir.Cancel](t, cancel.Root).Role; got != ir.CancelRoleStrike {
		t.Fatalf("cancel role = %v, want strike", got)
	}
}

func TestClassAndAlphabetStylesDoNotRewriteDescendants(t *testing.T) {
	t.Parallel()

	classed := parse(t, `class("relation", a+b)`)
	requireNoDiagnostics(t, "compound class", classed)
	row := findCompoundRow(t, classed.Root, ir.ClassRelation)
	assertSpacingClasses(t, row, ir.ClassRelation)

	styled := parse(t, `bold(a+b)`)
	requireNoDiagnostics(t, "compound alphabet style", styled)
	row = findCompoundRow(t, styled.Root, ir.ClassNormal)
	assertSpacingClasses(t, row, ir.ClassNormal)
}

func TestMultilineRowsUseNeutralRowSegments(t *testing.T) {
	t.Parallel()

	gathered := parse(t, `a = b+c \ d = e`)
	requireNoDiagnostics(t, "gathered rows", gathered)
	multiline, ok := gathered.Root.(*ir.Multiline)
	if !ok || len(multiline.Rows) != 2 {
		t.Fatalf("gathered root = %#v", gathered.Root)
	}
	for rowIndex, row := range multiline.Rows {
		if len(row) != 1 {
			t.Fatalf("gathered row %d has %d parts", rowIndex, len(row))
		}
		if _, ok := row[0].(*ir.Row); !ok {
			t.Fatalf("gathered row %d part = %T", rowIndex, row[0])
		}
	}

	aligned := parse(t, `a &= b \ c &= d`)
	requireNoDiagnostics(t, "aligned rows", aligned)
	multiline, ok = aligned.Root.(*ir.Multiline)
	if !ok || len(multiline.Rows) != 2 {
		t.Fatalf("aligned root = %#v", aligned.Root)
	}
	for rowIndex, row := range multiline.Rows {
		if len(row) != 3 {
			t.Fatalf("aligned row %d has %d parts", rowIndex, len(row))
		}
		if _, ok := row[0].(*ir.Row); !ok {
			t.Fatalf("aligned row %d first part = %T", rowIndex, row[0])
		}
		if _, ok := row[1].(*ir.AlignMarker); !ok {
			t.Fatalf("aligned row %d alignment = %T", rowIndex, row[1])
		}
		if _, ok := row[2].(*ir.Row); !ok {
			t.Fatalf("aligned row %d last part = %T", rowIndex, row[2])
		}
	}
}

func findCompoundRow(t *testing.T, root ir.Node, class ir.MathClass) *ir.Row {
	t.Helper()
	var found *ir.Row
	ir.Walk(root, func(node ir.Node) bool {
		row, ok := node.(*ir.Row)
		if ok && len(row.Items) == 3 && row.Class == class {
			found = row
			return false
		}
		return true
	})
	if found == nil {
		t.Fatalf("did not find compound row with class %v in %T", class, root)
	}
	return found
}

func assertSpacingClasses(t *testing.T, row *ir.Row, wrapper ir.MathClass) {
	t.Helper()
	if len(row.Items) != 3 {
		t.Fatalf("compound row has %d items, want 3", len(row.Items))
	}
	if row.Items[0].NodeBase().Class != ir.ClassAlphabetic ||
		row.Items[1].NodeBase().Class != ir.ClassBinary ||
		row.Items[2].NodeBase().Class != ir.ClassAlphabetic {
		t.Fatalf("compound child classes = %v/%v/%v",
			row.Items[0].NodeBase().Class, row.Items[1].NodeBase().Class, row.Items[2].NodeBase().Class)
	}
	if row.Class != wrapper {
		t.Fatalf("compound wrapper class = %v, want %v", row.Class, wrapper)
	}
}

func TestDifferentialAndBuiltinSpacingUseExactMathUnits(t *testing.T) {
	t.Parallel()

	result := parse(t, `integral f(x) dif x + Dif`)
	requireNoDiagnostics(t, "differential", result)
	var differentials []*ir.Row
	ir.Walk(result.Root, func(node ir.Node) bool {
		row, ok := node.(*ir.Row)
		if !ok || len(row.Items) != 2 {
			return true
		}
		space, spaceOK := row.Items[0].(*ir.Space)
		glyph, glyphOK := row.Items[1].(*ir.Glyph)
		if spaceOK && glyphOK && (glyph.Text == "d" || glyph.Text == "D") {
			if space.Weak && space.Amount == builtinMuLength(3) &&
				glyph.Class == ir.ClassUnary && glyph.Variant == ir.VariantNormal {
				differentials = append(differentials, row)
			}
		}
		return true
	})
	if len(differentials) != 2 {
		t.Fatalf("differentials = %d, want 2", len(differentials))
	}

	spacing := map[string]int64{
		"thin":  3,
		"med":   4,
		"thick": 5,
		"quad":  18,
		"wide":  36,
	}
	for name, want := range spacing {
		result := parse(t, name)
		requireNoDiagnostics(t, name, result)
		space := find[*ir.Space](t, result.Root)
		if space.Amount != builtinMuLength(float64(want)) {
			t.Fatalf("%s amount = %#v, want %d mu", name, space.Amount, want)
		}
	}
}

func TestMatrixArrayRowsPadAndRemainBounded(t *testing.T) {
	t.Parallel()

	for _, src := range []string{
		`mat(#(1, 2), #(3, 4))`,
		`mat(..#((1, 2), (3, 4)))`,
	} {
		result := parse(t, src)
		requireNoDiagnostics(t, src, result)
		table := find[*ir.Table](t, result.Root)
		if len(table.Rows) != 2 || len(table.Columns) != 2 {
			t.Fatalf("%s shape = %dx%d", src, len(table.Rows), len(table.Columns))
		}
		for rowIndex, row := range table.Rows {
			if len(row.Cells) != 2 {
				t.Fatalf("%s row %d has %d cells", src, rowIndex, len(row.Cells))
			}
		}
	}

	ragged := parse(t, `mat(#(1,), #(2, 3))`)
	requireNoDiagnostics(t, "ragged matrix rows", ragged)
	table := find[*ir.Table](t, ragged.Root)
	if len(table.Rows) != 2 || len(table.Rows[0].Cells) != 2 || len(table.Rows[1].Cells) != 2 {
		t.Fatalf("ragged matrix shape = %#v", table.Rows)
	}

	limits := source.DefaultLimits()
	limits.MaxTableCells = 3
	bounded := Parse(`mat(#(1, 2), #(3, 4))`, ir.Inline, Options{Limits: limits})
	if bounded.Root == nil {
		t.Fatal("bounded matrix returned nil root")
	}
	if err := ir.Validate(&ir.Formula{Root: bounded.Root}, limits); err != nil {
		t.Fatalf("bounded matrix produced invalid IR: %v", err)
	}
	var tableLimited bool
	for _, diagnostic := range bounded.Diagnostics {
		if diagnostic.Code == source.CodeLimitExceeded {
			tableLimited = true
		}
	}
	if !tableLimited {
		t.Fatalf("bounded matrix diagnostics = %#v, want limit", bounded.Diagnostics)
	}
}

func TestValueDisplayAndMathStylesPreserveSemantics(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		src  string
		text string
	}{
		{`#auto`, "auto"},
		{`#true`, "true"},
		{`#false`, "false"},
		{`#1em`, "1em"},
		{`#(1, 2)`, "(1, 2)"},
	} {
		result := parse(t, test.src)
		requireNoDiagnostics(t, test.src, result)
		text := find[*ir.Text](t, result.Root)
		if text.Text != test.text || text.Font != ir.FontText || text.Variant != ir.VariantMonospace {
			t.Fatalf("%s text = %#v", test.src, text)
		}
	}

	for _, src := range []string{`#text(fill: red, $x$)`} {
		result := parse(t, src)
		requireNoDiagnostics(t, src, result)
		find[*ir.Color](t, result.Root)
	}
	for _, src := range []string{
		`#box(fill: red, $x$)`,
		`#box(fill: rgb(255, 0, 0), $x$)`,
	} {
		result := parse(t, src)
		requireNoDiagnostics(t, src, result)
		box := find[*ir.Box](t, result.Root)
		if !box.HasFill || !box.Fill.HasExplicit || box.Fill.Explicit.R != 0xff {
			t.Fatalf("%s fill = %#v", src, box.Fill)
		}
	}

	for _, test := range []struct {
		src   string
		class ir.MathClass
	}{
		{`bold(alpha)`, ir.ClassAlphabetic},
		{`italic(plus)`, ir.ClassBinary},
		{`bb(R)`, ir.ClassAlphabetic},
		{`sans(integral)`, ir.ClassLarge},
	} {
		result := parse(t, test.src)
		requireNoDiagnostics(t, test.src, result)
		glyph := find[*ir.Glyph](t, result.Root)
		if glyph.Class != test.class {
			t.Fatalf("%s class = %v, want %v", test.src, glyph.Class, test.class)
		}
		if glyph.Variant == ir.VariantAuto {
			t.Fatalf("%s did not apply a math variant", test.src)
		}
	}

	for _, src := range []string{
		`dash(x)`,
		`diaer(x)`,
		`acute.double(x)`,
		`harpoon(x)`,
		`harpoon.lt(x)`,
	} {
		result := parse(t, src)
		requireNoDiagnostics(t, src, result)
		find[*ir.Accent](t, result.Root)
	}
}
