package tex

import (
	"testing"

	"github.com/caelyreth/untie/ir"
)

func TestCompoundNodesUseNeutralClasses(t *testing.T) {
	t.Parallel()

	fraction := requireKind[*ir.Fraction](t, rowItems(t, parseTest(t, `\frac12`, Options{}).Root)[0])
	if fraction.Class != ir.ClassNormal {
		t.Errorf("fraction class = %v, want normal", fraction.Class)
	}
	if fraction.Padding != defaultFractionPadding {
		t.Errorf("fraction padding = %v, want %v", fraction.Padding, defaultFractionPadding)
	}

	style := requireKind[*ir.Style](t, rowItems(t, parseTest(t, `\displaystyle x`, Options{}).Root)[0])
	if style.Class != ir.ClassNormal {
		t.Errorf("style class = %v, want normal", style.Class)
	}

	operator := requireKind[*ir.Operator](t, rowItems(t, parseTest(t, `\operatorname{tr}`, Options{}).Root)[0])
	if operator.Class != ir.ClassLarge {
		t.Errorf("operatorname class = %v, want large", operator.Class)
	}

	line := requireKind[*ir.Line](t, rowItems(t, parseTest(t, `\overline{x}`, Options{}).Root)[0])
	if line.Thickness != defaultStrokeThickness {
		t.Errorf("line thickness = %v, want %v", line.Thickness, defaultStrokeThickness)
	}
	cancel := requireKind[*ir.Cancel](t, rowItems(t, parseTest(t, `\cancel{x}`, Options{}).Root)[0])
	if cancel.Thickness != defaultStrokeThickness {
		t.Errorf("cancel thickness = %v, want %v", cancel.Thickness, defaultStrokeThickness)
	}
}

func TestImplicitGroupsDoNotBecomeInnerAtoms(t *testing.T) {
	t.Parallel()

	scripts := requireKind[*ir.Scripts](t, rowItems(t, parseTest(t, `x^{2}`, Options{}).Root)[0])
	group := requireKind[*ir.Row](t, scripts.Top)
	if group.Class != ir.ClassNormal {
		t.Errorf("script group class = %v, want normal", group.Class)
	}

	radical := requireKind[*ir.Radical](t, rowItems(t, parseTest(t, `\sqrt{x+1}`, Options{}).Root)[0])
	radicand := requireKind[*ir.Row](t, radical.Radicand)
	if radicand.Class != ir.ClassNormal {
		t.Errorf("radicand group class = %v, want normal", radicand.Class)
	}
}

func TestStyleDeclarationsRemainOnTheWrapper(t *testing.T) {
	t.Parallel()

	root := parseTest(t, `\displaystyle\sum_{i=1}^n`, Options{}).Root
	style := requireKind[*ir.Style](t, rowItems(t, root)[0])
	if !style.SetSize || style.NewSize != ir.SizeDisplay {
		t.Fatalf("style wrapper = %#v, want display size", style)
	}
	body := rowItems(t, style.Body)
	scripts := requireKind[*ir.Scripts](t, body[0])
	if scripts.Size != ir.SizeInherit || scripts.Bottom.NodeBase().Size != ir.SizeInherit {
		t.Fatalf("style declaration leaked into descendants: scripts=%v bottom=%v",
			scripts.Size, scripts.Bottom.NodeBase().Size)
	}
}

func TestFenceRolesAndMatrixSemantics(t *testing.T) {
	t.Parallel()

	pmatrix := requireKind[*ir.Fenced](t, rowItems(t, parseTest(t, `\begin{pmatrix}1&2\\3&4\end{pmatrix}`, Options{}).Root)[0])
	if pmatrix.Class != ir.ClassInner || !pmatrix.Balanced {
		t.Errorf("pmatrix class/balanced = %v/%v, want inner/true", pmatrix.Class, pmatrix.Balanced)
	}
	table := requireKind[*ir.Table](t, pmatrix.Body[0])
	if table.Class != ir.ClassNormal {
		t.Errorf("matrix table class = %v, want normal", table.Class)
	}
	if len(table.Columns) != 2 || table.Columns[0] != ir.AlignCenter || table.Columns[1] != ir.AlignCenter {
		t.Errorf("matrix columns = %v, want two centered columns", table.Columns)
	}

	binom := requireKind[*ir.Fenced](t, rowItems(t, parseTest(t, `\binom nk`, Options{}).Root)[0])
	if binom.Balanced {
		t.Error("binom fence is balanced, want independent matrix-style fences")
	}
	fraction := requireKind[*ir.Fraction](t, binom.Body[0])
	if fraction.Style != ir.FractionNoBar || fraction.Bar {
		t.Errorf("binom fraction style/bar = %v/%v, want no-bar/false", fraction.Style, fraction.Bar)
	}

	small := requireKind[*ir.Fenced](t, rowItems(t, parseTest(
		t,
		`\left(\begin{smallmatrix}a&b\\c&d\end{smallmatrix}\right)`,
		Options{},
	).Root)[0])
	if small.Size != ir.SizeScript ||
		small.Open.NodeBase().Size != ir.SizeInherit ||
		small.Close.NodeBase().Size != ir.SizeInherit {
		t.Errorf("smallmatrix fence sizes = %v/%v/%v, want wrapper script with inherited delimiters",
			small.Size, small.Open.NodeBase().Size, small.Close.NodeBase().Size)
	}
}

func TestOperatorLimitDefaults(t *testing.T) {
	t.Parallel()

	tests := []struct {
		source string
		limits ir.LimitsPlacement
	}{
		{`\det A`, ir.LimitsNever},
		{`\gcd(a,b)`, ir.LimitsNever},
		{`\lim_{x\to0}`, ir.LimitsAuto},
		{`\int_0^1`, ir.LimitsNever},
		{`\operatorname*{argmax}_x`, ir.LimitsAlways},
	}
	for _, tc := range tests {
		tc := tc
		t.Run(tc.source, func(t *testing.T) {
			t.Parallel()
			node := rowItems(t, parseTest(t, tc.source, Options{}).Root)[0]
			if node.NodeBase().Limits != tc.limits {
				t.Fatalf("limits = %v, want %v", node.NodeBase().Limits, tc.limits)
			}
		})
	}
}

func TestStackedAndSideScripts(t *testing.T) {
	t.Parallel()

	overset := requireKind[*ir.Scripts](t, rowItems(t, parseTest(t, `\overset{a}{=}`, Options{}).Root)[0])
	if overset.Limits != ir.LimitsAlways || overset.Top == nil || overset.Bottom != nil {
		t.Errorf("overset limits/top/bottom = %v/%v/%v", overset.Limits, overset.Top != nil, overset.Bottom != nil)
	}

	arrow := requireKind[*ir.Scripts](t, rowItems(t, parseTest(t, `\xrightarrow[below]{above}`, Options{}).Root)[0])
	if arrow.Limits != ir.LimitsAlways || arrow.Top == nil || arrow.Bottom == nil {
		t.Errorf("xrightarrow limits/top/bottom = %v/%v/%v", arrow.Limits, arrow.Top != nil, arrow.Bottom != nil)
	}

	sideset := requireKind[*ir.Scripts](t, rowItems(t, parseTest(t, `\sideset{_a^b}{_c^d}\sum`, Options{}).Root)[0])
	if sideset.TopLeft == nil || sideset.BottomLeft == nil || sideset.TopRight == nil || sideset.BottomRight == nil {
		t.Errorf("sideset fields = %v/%v/%v/%v", sideset.TopLeft, sideset.BottomLeft, sideset.TopRight, sideset.BottomRight)
	}
	if sideset.Top != nil || sideset.Bottom != nil {
		t.Error("sideset incorrectly used semantic top/bottom fields")
	}
}

func TestSubstackUsesMultiline(t *testing.T) {
	t.Parallel()

	outer := requireKind[*ir.Scripts](t, rowItems(t, parseTest(t, `\sum_{\substack{a\\b}}`, Options{}).Root)[0])
	var stack *ir.Multiline
	ir.Walk(outer.Bottom, func(node ir.Node) bool {
		if candidate, ok := node.(*ir.Multiline); ok {
			stack = candidate
		}
		return stack == nil
	})
	if stack == nil {
		t.Fatal("substack did not lower to multiline")
	}
	if !stack.Centered || len(stack.Rows) != 2 {
		t.Errorf("substack centered/rows = %v/%d, want true/2", stack.Centered, len(stack.Rows))
	}
}

func TestArrayRulesAndPmodAreRepresented(t *testing.T) {
	t.Parallel()

	table := requireKind[*ir.Table](t, rowItems(t, parseTest(
		t,
		`\begin{array}{c|c}a&b\\\hline c&d\end{array}`,
		Options{},
	).Root)[0])
	if len(table.Horizontal) != 1 || len(table.Vertical) != 1 {
		t.Errorf("array rules = %d horizontal, %d vertical; want 1/1", len(table.Horizontal), len(table.Vertical))
	}
	if table.Horizontal[0].Count != 1 {
		t.Errorf("single hline count = %d, want 1", table.Horizontal[0].Count)
	}

	double := requireKind[*ir.Table](t, rowItems(t, parseTest(
		t,
		`\begin{array}{cc}a&b\\\hline\hline c&d\end{array}`,
		Options{},
	).Root)[0])
	if len(double.Horizontal) != 1 || double.Horizontal[0].Count != 2 {
		t.Errorf("double hline strokes = %+v, want one line with count 2", double.Horizontal)
	}

	result := parseTest(t, `a\pmod n`, Options{})
	var fenced *ir.Fenced
	ir.Walk(result.Root, func(node ir.Node) bool {
		if candidate, ok := node.(*ir.Fenced); ok && fenced == nil {
			fenced = candidate
		}
		return fenced == nil
	})
	if fenced == nil {
		t.Fatal("pmod did not lower to a fenced group")
	}
	var operators int
	ir.Walk(fenced, func(node ir.Node) bool {
		if operator, ok := node.(*ir.Operator); ok && operator.Text == "mod" {
			operators++
			if operator.Class != ir.ClassLarge || operator.Limits != ir.LimitsNever {
				t.Errorf("pmod operator class/limits = %v/%v", operator.Class, operator.Limits)
			}
		}
		return true
	})
	if operators != 1 {
		t.Fatalf("pmod operator count = %d, want 1", operators)
	}
}
