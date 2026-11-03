package tex

import (
	"testing"
	"time"

	"github.com/caelyreth/untie/ir"
	"github.com/caelyreth/untie/source"
)

func TestMathClassCommandsDoNotCreateImplicitLimits(t *testing.T) {
	t.Parallel()

	tests := []struct {
		source string
		class  ir.MathClass
	}{
		{`\mathop{x}_i`, ir.ClassLarge},
		{`\mathbin{+}_i`, ir.ClassBinary},
		{`\mathrel{=}_i`, ir.ClassRelation},
		{`\mathord{x}_i`, ir.ClassNormal},
	}
	for _, tc := range tests {
		tc := tc
		t.Run(tc.source, func(t *testing.T) {
			t.Parallel()
			scripts := requireKind[*ir.Scripts](t, rowItems(t, parseTest(t, tc.source, Options{}).Root)[0])
			if scripts.Limits != ir.LimitsNever || scripts.Class != tc.class {
				t.Fatalf("scripts class/limits = %v/%v, want %v/%v",
					scripts.Class, scripts.Limits, tc.class, ir.LimitsNever)
			}
		})
	}

	scripts := requireKind[*ir.Scripts](t, rowItems(t, parseTest(t, `\mathop{x}\limits_i`, Options{}).Root)[0])
	if scripts.Limits != ir.LimitsAlways || scripts.Class != ir.ClassLarge {
		t.Fatalf("mathop limits class/limits = %v/%v, want large/always", scripts.Class, scripts.Limits)
	}
}

func TestNamedOperatorLimitContracts(t *testing.T) {
	t.Parallel()

	for _, src := range []string{
		`\liminf_x`, `\limsup_x`, `\argmax_x`, `\argmin_x`,
		`\injlim_x`, `\projlim_x`, `\plim_x`,
		`\varlimsup_x`, `\varliminf_x`, `\varinjlim_x`, `\varprojlim_x`,
	} {
		src := src
		t.Run(src, func(t *testing.T) {
			t.Parallel()
			scripts := requireKind[*ir.Scripts](t, rowItems(t, parseTest(t, src, Options{}).Root)[0])
			if scripts.Class != ir.ClassLarge || scripts.Limits != ir.LimitsAlways {
				t.Fatalf("class/limits = %v/%v, want large/always", scripts.Class, scripts.Limits)
			}
		})
	}

	root := parseTest(t, `\liminf`, Options{}).Root
	var operatorTexts []string
	var thinSpace ir.Length
	ir.Walk(root, func(node ir.Node) bool {
		switch node := node.(type) {
		case *ir.Operator:
			operatorTexts = append(operatorTexts, node.Text)
		case *ir.Space:
			thinSpace = node.Amount
		}
		return true
	})
	if len(operatorTexts) != 2 || operatorTexts[0] != "lim" || operatorTexts[1] != "inf" {
		t.Fatalf("operator parts = %v, want [lim inf]", operatorTexts)
	}
	wantThin, _ := ir.EmLength(1.0 / 6.0)
	if thinSpace != wantThin {
		t.Fatalf("operator separator = %v, want %v", thinSpace, wantThin)
	}
}

func TestStackrelNotAndStructuralBracesUseKaTeXClasses(t *testing.T) {
	t.Parallel()

	stackrel := requireKind[*ir.Scripts](t, rowItems(t, parseTest(t, `\stackrel{?}{x}`, Options{}).Root)[0])
	if stackrel.Class != ir.ClassRelation || stackrel.Limits != ir.LimitsAlways {
		t.Fatalf("stackrel class/limits = %v/%v, want relation/always", stackrel.Class, stackrel.Limits)
	}

	not := requireKind[*ir.Glyph](t, rowItems(t, parseTest(t, `\not=`, Options{}).Root)[0])
	if not.Text != "≠" || not.Class != ir.ClassRelation {
		t.Fatalf("not glyph = %q/%v, want ≠/relation", not.Text, not.Class)
	}
	fallback := requireKind[*ir.Cancel](t, rowItems(t, parseTest(t, `\not\between`, Options{}).Root)[0])
	if fallback.Class != ir.ClassRelation || fallback.Body.NodeBase().Class != ir.ClassRelation {
		t.Fatalf("not fallback classes = %v/%v, want relation/relation", fallback.Class, fallback.Body.NodeBase().Class)
	}

	for _, source := range []string{`\overbrace{x}`, `\underbrace{x}`, `\overbracket{x}`, `\underbracket{x}`} {
		node := rowItems(t, parseTest(t, source, Options{}).Root)[0]
		if node.NodeBase().Class != ir.ClassInner || node.NodeBase().Limits != ir.LimitsAlways {
			t.Errorf("%s class/limits = %v/%v, want inner/always", source, node.NodeBase().Class, node.NodeBase().Limits)
		}
	}

	utilde := rowItems(t, parseTest(t, `\utilde{x}`, Options{}).Root)[0].(*ir.Accent)
	if !utilde.Stretchy {
		t.Fatal("utilde is not stretchy")
	}
}

func TestSemanticRolesComeFromTeXSyntax(t *testing.T) {
	t.Parallel()

	tests := []struct {
		source string
		role   ir.ScriptRole
	}{
		{`x^2`, ir.ScriptRoleOrdinary},
		{`\sin^2 x`, ir.ScriptRoleOrdinary},
		{`\sum_{i=1}^n`, ir.ScriptRoleLimits},
		{`\int_0^1`, ir.ScriptRoleLimits},
		{`\sum\nolimits_{i=1}^n`, ir.ScriptRoleLimits},
		{`\overset{a}{b}`, ir.ScriptRoleOverlay},
		{`\underset{a}{b}`, ir.ScriptRoleOverlay},
		{`\stackrel{a}{=}`, ir.ScriptRoleOverlay},
		{`\xrightarrow[below]{above}`, ir.ScriptRoleOverlay},
		{`\overbrace{x+y}^{n}`, ir.ScriptRoleBraceLabel},
		{`\underbrace{x+y}_{n}`, ir.ScriptRoleBraceLabel},
	}
	for _, tc := range tests {
		tc := tc
		t.Run(tc.source, func(t *testing.T) {
			t.Parallel()
			scripts := requireKind[*ir.Scripts](t, rowItems(t, parseTest(t, tc.source, Options{}).Root)[0])
			if scripts.Role != tc.role {
				t.Fatalf("role = %v, want %v", scripts.Role, tc.role)
			}
		})
	}

	ordinaryFraction := requireKind[*ir.Fraction](t, rowItems(t, parseTest(t, `\frac12`, Options{}).Root)[0])
	if ordinaryFraction.Role != ir.FractionRoleFraction {
		t.Fatalf("fraction role = %v, want quotient", ordinaryFraction.Role)
	}
	stacked := requireKind[*ir.Fraction](t, rowItems(t, parseTest(t, `a\atop b`, Options{}).Root)[0])
	if stacked.Role != ir.FractionRoleStacked {
		t.Fatalf("atop role = %v, want stacked", stacked.Role)
	}

	matrix := requireKind[*ir.Table](t, rowItems(t, parseTest(t, `\begin{matrix}a&b\end{matrix}`, Options{}).Root)[0])
	if matrix.Role != ir.TableRoleMatrix {
		t.Fatalf("matrix role = %v", matrix.Role)
	}
	array := requireKind[*ir.Table](t, rowItems(t, parseTest(t, `\begin{array}{cc}a&b\end{array}`, Options{}).Root)[0])
	if array.Role != ir.TableRoleArray {
		t.Fatalf("array role = %v", array.Role)
	}
	casesFence := requireKind[*ir.Fenced](t, rowItems(t, parseTest(t, `\begin{cases}a&b\end{cases}`, Options{}).Root)[0])
	cases := requireKind[*ir.Table](t, casesFence.Body[0])
	if cases.Role != ir.TableRoleCases {
		t.Fatalf("cases role = %v", cases.Role)
	}

	not := requireKind[*ir.Cancel](t, rowItems(t, parseTest(t, `\not\between`, Options{}).Root)[0])
	if not.Role != ir.CancelRoleNegation {
		t.Fatalf("not role = %v", not.Role)
	}
	cancel := requireKind[*ir.Cancel](t, rowItems(t, parseTest(t, `\cancel{x}`, Options{}).Root)[0])
	if cancel.Role != ir.CancelRoleStrike {
		t.Fatalf("cancel role = %v", cancel.Role)
	}
}

func TestNotPrefersSemanticNegatedRelations(t *testing.T) {
	t.Parallel()

	tests := map[string]string{
		`\not=`:           "≠",
		`\not{=}`:         "≠",
		`\not\equiv`:      "≢",
		`\not\exists`:     "∄",
		`\not\in`:         "∉",
		`\not\leq`:        "≰",
		`\not\sim`:        "≁",
		`\not\subseteq`:   "⊈",
		`\not\rightarrow`: "↛",
	}
	for source, want := range tests {
		source, want := source, want
		t.Run(source, func(t *testing.T) {
			t.Parallel()
			node := requireKind[*ir.Glyph](t, rowItems(t, parseTest(t, source, Options{}).Root)[0])
			if node.Text != want || node.Class != ir.ClassRelation {
				t.Fatalf("not glyph = %q/%v, want %q/relation", node.Text, node.Class, want)
			}
		})
	}
}

func TestCancelCommandsSetStrokeMode(t *testing.T) {
	t.Parallel()

	tests := []struct {
		source   string
		cross    bool
		inverted bool
	}{
		{source: `\cancel{x}`},
		{source: `\bcancel{x}`, inverted: true},
		{source: `\xcancel{x}`, cross: true},
	}
	for _, test := range tests {
		node := requireKind[*ir.Cancel](t, rowItems(t, parseTest(t, test.source, Options{}).Root)[0])
		if node.Cross != test.cross || node.Inverted != test.inverted {
			t.Errorf("%s cross/inverted = %t/%t, want %t/%t",
				test.source, node.Cross, node.Inverted, test.cross, test.inverted)
		}
	}
}

func TestGenfracStyleInheritance(t *testing.T) {
	t.Parallel()

	inherit := requireKind[*ir.Fraction](t, rowItems(t, parseTest(t, `\genfrac{}{}{}{}{a}{b}`, Options{}).Root)[0])
	if inherit.Size != ir.SizeInherit {
		t.Fatalf("empty genfrac style = %v, want inherit", inherit.Size)
	}
	text := requireKind[*ir.Fraction](t, rowItems(t, parseTest(t, `\genfrac{}{}{}{1}{a}{b}`, Options{}).Root)[0])
	if text.Size != ir.SizeText {
		t.Fatalf("text genfrac style = %v, want text", text.Size)
	}
}

func TestInlineMathInTextTerminatesAndPreservesStructure(t *testing.T) {
	t.Parallel()

	sources := []string{
		`\text{graph: \(y = mx + b\)}`,
		`\text{hello \(x + \text{world $y$} + z\)}`,
		`\text{hello $x + \text{world \(y\)} + z$}`,
	}
	for _, src := range sources {
		src := src
		t.Run(src, func(t *testing.T) {
			t.Parallel()
			result := parseWithin(t, src, 2*time.Second)
			for _, diagnostic := range result.Diagnostics {
				if diagnostic.Severity == source.SeverityError {
					t.Fatalf("diagnostics: %v", result.Diagnostics)
				}
			}
			var styles int
			ir.Walk(result.Root, func(node ir.Node) bool {
				if node.Kind() == ir.KindStyle {
					styles++
				}
				return true
			})
			if styles == 0 {
				t.Fatal("inline math content was not parsed")
			}
		})
	}

	for _, src := range []string{`\text{\(x$}`, `\text{\)}`, `\(x\)`} {
		result := parseTest(t, src, Options{})
		if len(result.Diagnostics) == 0 {
			t.Errorf("%s parsed without an error diagnostic", src)
		}
	}
}

func TestTextRunsAreCoalescedAndSpacesContracted(t *testing.T) {
	t.Parallel()

	result := parseTest(t, `\text{  if   x }`, Options{})
	var runs []string
	ir.Walk(result.Root, func(node ir.Node) bool {
		if text, ok := node.(*ir.Text); ok {
			runs = append(runs, text.Text)
		}
		return true
	})
	if len(runs) != 1 || runs[0] != " if x " {
		t.Fatalf("text runs = %q, want [%q]", runs, " if x ")
	}
}

func TestSizeDeclarationsLowerToNeutralFontSizeAndTextBox(t *testing.T) {
	t.Parallel()

	tests := map[string]float64{
		`\tiny`:         50,
		`\sixptsize`:    60,
		`\scriptsize`:   70,
		`\footnotesize`: 80,
		`\small`:        90,
		`\normalsize`:   100,
		`\large`:        120,
		`\Large`:        144,
		`\LARGE`:        172.8,
		`\huge`:         207.4,
		`\Huge`:         248.8,
	}
	for command, percent := range tests {
		command, percent := command, percent
		t.Run(command, func(t *testing.T) {
			t.Parallel()
			src := command + " label"
			result := parseWithin(t, src, 2*time.Second)
			for _, diagnostic := range result.Diagnostics {
				if diagnostic.Severity == source.SeverityError {
					t.Fatalf("diagnostics = %v", result.Diagnostics)
				}
			}
			style := requireKind[*ir.Style](t, rowItems(t, result.Root)[0])
			if !style.SetFontSize || style.FontSize.Unit != ir.LengthPercent || style.FontSize.Float() != percent {
				t.Fatalf("font size = set:%t value:%+v, want %v%%", style.SetFontSize, style.FontSize, percent)
			}
			if !style.SetSize || style.NewSize != ir.SizeText {
				t.Fatalf("style = set:%t size:%v, want explicit text style", style.SetSize, style.NewSize)
			}
		})
	}

	result := parseWithin(t, `\text{\small text}`, 2*time.Second)
	for _, diagnostic := range result.Diagnostics {
		if diagnostic.Severity == source.SeverityError {
			t.Fatalf("text sizing diagnostics = %v", result.Diagnostics)
		}
	}
	var textStyle *ir.Style
	ir.Walk(result.Root, func(node ir.Node) bool {
		if candidate, ok := node.(*ir.Style); ok && candidate.SetFontSize {
			textStyle = candidate
		}
		return true
	})
	if textStyle == nil || textStyle.FontSize.Float() != 90 {
		t.Fatalf("text sizing style = %+v, want 90%%", textStyle)
	}
}

func TestExplicitAlphabetLoweringKeepsGroupsTransparent(t *testing.T) {
	t.Parallel()

	tests := []struct {
		source  string
		variant ir.Variant
	}{
		{`\mathbb{R}`, ir.VariantDoubleStruck},
		{`\mathcal{L}`, ir.VariantScript},
		{`\mathbf{x}`, ir.VariantBold},
		{`\boldsymbol{\alpha}`, ir.VariantBoldItalic},
	}
	for _, tc := range tests {
		tc := tc
		t.Run(tc.source, func(t *testing.T) {
			t.Parallel()
			group := requireKind[*ir.Row](t, rowItems(t, parseTest(t, tc.source, Options{}).Root)[0])
			if group.Variant != ir.VariantAuto {
				t.Errorf("group variant = %v, want auto", group.Variant)
			}
			glyph := requireKind[*ir.Glyph](t, group.Items[0])
			if glyph.Class != ir.ClassNormal || glyph.Variant != tc.variant {
				t.Errorf("glyph class/variant = %v/%v, want normal/%v", glyph.Class, glyph.Variant, tc.variant)
			}
		})
	}
}

func TestCommonKaTeXCommandsAndUnrepresentableOptions(t *testing.T) {
	t.Parallel()

	strut := requireKind[*ir.Phantom](t, rowItems(t, parseTest(t, `\mathstrut`, Options{}).Root)[0])
	if strut.Width || !strut.Height || !strut.Depth {
		t.Fatalf("mathstrut phantom extents = %v/%v/%v, want false/true/true",
			strut.Width, strut.Height, strut.Depth)
	}

	if node := rowItems(t, parseTest(t, `\textcircled{1}`, Options{}).Root)[0]; node.Kind() != ir.KindAccent {
		t.Fatalf("textcircled kind = %v, want accent", node.Kind())
	}
	if got := collectText(parseTest(t, `\text{a\textdagger b\textdaggerdbl c}`, Options{}).Root); got != "a†b‡c" {
		t.Fatalf("text symbols = %q, want %q", got, "a†b‡c")
	}
	for source, want := range map[string]string{`\char` + "`a": "a", `\char` + "`\\#": "#"} {
		glyph := requireKind[*ir.Glyph](t, rowItems(t, parseTest(t, source, Options{}).Root)[0])
		if glyph.Text != want {
			t.Errorf("%s glyph = %q, want %q", source, glyph.Text, want)
		}
	}
	smash := requireKind[*ir.Phantom](t, rowItems(t, parseTest(t, `\smash[t]{x}`, Options{}).Root)[0])
	if !smash.Width || smash.Height || !smash.Depth {
		t.Fatalf("smash[t] extents = %v/%v/%v, want true/false/true",
			smash.Width, smash.Height, smash.Depth)
	}

	for _, src := range []string{
		`\cfrac{1}{2}`,
		`\genfrac(){1pt}{0}{a}{b}`,
		`a\above 1pt b`,
		`\rule[1pt]{1em}{1em}`,
		`\mathchoice{a}{b}{c}{d}`,
	} {
		result := parseTest(t, src, Options{})
		if !hasDiagnostic(result.Diagnostics, source.CodeUnsupported) {
			t.Errorf("%s diagnostics = %v, want explicit unsupported", src, diagnosticCodes(result.Diagnostics))
		}
	}
}
