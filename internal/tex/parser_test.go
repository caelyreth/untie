package tex

import (
	"strings"
	"testing"

	"github.com/caelyreth/untie/ir"
	"github.com/caelyreth/untie/source"
)

func parseTest(t *testing.T, src string, opts Options) Result {
	t.Helper()
	result := Parse(src, ir.Display, opts)
	if result.Root == nil {
		t.Fatal("Parse returned a nil root")
	}
	if err := ir.Validate(&ir.Formula{Root: result.Root}, opts.Limits); err != nil {
		t.Fatalf("invalid IR for %q: %v", src, err)
	}
	return result
}

func rowItems(t *testing.T, node ir.Node) []ir.Node {
	t.Helper()
	row, ok := node.(*ir.Row)
	if !ok {
		t.Fatalf("got %T, want *ir.Row", node)
	}
	return row.Items
}

func requireKind[T ir.Node](t *testing.T, node ir.Node) T {
	t.Helper()
	typed, ok := node.(T)
	if !ok {
		t.Fatalf("got %T", node)
	}
	return typed
}

func diagnosticCodes(diagnostics []source.Diagnostic) []source.Code {
	out := make([]source.Code, len(diagnostics))
	for i, d := range diagnostics {
		out[i] = d.Code
	}
	return out
}

func hasDiagnostic(diagnostics []source.Diagnostic, code source.Code) bool {
	for _, d := range diagnostics {
		if d.Code == code {
			return true
		}
	}
	return false
}

func TestParseLexicalSpans(t *testing.T) {
	t.Parallel()

	result := parseTest(t, "x+α", Options{})
	items := rowItems(t, result.Root)
	if len(items) != 3 {
		t.Fatalf("got %d items, want 3", len(items))
	}
	cases := []struct {
		text string
		span source.Range
	}{
		{"x", source.NewRange(0, 1)},
		{"+", source.NewRange(1, 2)},
		{"α", source.NewRange(2, 4)},
	}
	for i, want := range cases {
		glyph := requireKind[*ir.Glyph](t, items[i])
		if glyph.Text != want.text || glyph.Span != want.span {
			t.Errorf("item %d = (%q, %v), want (%q, %v)", i, glyph.Text, glyph.Span, want.text, want.span)
		}
	}
	if result.Root.NodeBase().Span != source.NewRange(0, 4) {
		t.Errorf("root span = %v, want [0,4)", result.Root.NodeBase().Span)
	}
	if result.Root.NodeBase().Size != ir.SizeDisplay {
		t.Errorf("root size = %v, want display", result.Root.NodeBase().Size)
	}
}

func TestLexerCommentsAndControlWordWhitespace(t *testing.T) {
	t.Parallel()

	result := parseTest(t, "\\alpha % ignored\n + \\beta", Options{})
	items := rowItems(t, result.Root)
	if len(items) != 3 {
		t.Fatalf("got %d items, want 3", len(items))
	}
	if glyph := requireKind[*ir.Glyph](t, items[0]); glyph.Text != "α" {
		t.Errorf("first glyph = %q", glyph.Text)
	}
	if glyph := requireKind[*ir.Glyph](t, items[2]); glyph.Text != "β" {
		t.Errorf("third glyph = %q", glyph.Text)
	}
}

func TestCoreConstructsLowerToOnePassIR(t *testing.T) {
	t.Parallel()

	tests := []struct {
		source string
		check  func(*testing.T, ir.Node)
	}{
		{
			source: "x^2_i",
			check: func(t *testing.T, root ir.Node) {
				items := rowItems(t, root)
				scripts := requireKind[*ir.Scripts](t, items[0])
				if scripts.BaseItem.NodeBase().Kind() != ir.KindGlyph {
					t.Errorf("base kind = %v", scripts.BaseItem.NodeBase().Kind())
				}
				if scripts.Top.NodeBase().Kind() != ir.KindGlyph || scripts.Bottom.NodeBase().Kind() != ir.KindGlyph {
					t.Errorf("scripts were not lowered")
				}
			},
		},
		{
			source: "^2",
			check: func(t *testing.T, root ir.Node) {
				scripts := requireKind[*ir.Scripts](t, rowItems(t, root)[0])
				if scripts.BaseItem.NodeBase().Kind() != ir.KindRow {
					t.Errorf("empty script base kind = %v", scripts.BaseItem.NodeBase().Kind())
				}
				if scripts.Top == nil {
					t.Error("missing superscript")
				}
			},
		},
		{
			source: "f''",
			check: func(t *testing.T, root ir.Node) {
				scripts := requireKind[*ir.Scripts](t, rowItems(t, root)[0])
				top := requireKind[*ir.Row](t, scripts.Top)
				if len(top.Items) != 2 {
					t.Fatalf("prime count = %d, want 2", len(top.Items))
				}
			},
		},
		{
			source: `\frac{1}{2}`,
			check: func(t *testing.T, root ir.Node) {
				fraction := requireKind[*ir.Fraction](t, rowItems(t, root)[0])
				if !fraction.Bar || fraction.Style != ir.FractionVertical {
					t.Errorf("fraction defaults = bar:%v style:%v", fraction.Bar, fraction.Style)
				}
			},
		},
		{
			source: `\frac12`,
			check: func(t *testing.T, root ir.Node) {
				fraction := requireKind[*ir.Fraction](t, rowItems(t, root)[0])
				if glyph := requireKind[*ir.Glyph](t, fraction.Numerator); glyph.Text != "1" {
					t.Errorf("numerator = %q", glyph.Text)
				}
			},
		},
		{
			source: `\sqrt{x}`,
			check: func(t *testing.T, root ir.Node) {
				radical := requireKind[*ir.Radical](t, rowItems(t, root)[0])
				if radical.Index != nil {
					t.Errorf("unexpected radical index %T", radical.Index)
				}
			},
		},
		{
			source: `\sqrt[3]{x}`,
			check: func(t *testing.T, root ir.Node) {
				radical := requireKind[*ir.Radical](t, rowItems(t, root)[0])
				if radical.Index == nil {
					t.Error("missing radical index")
				}
			},
		},
		{
			source: `\left(\frac ab\right)`,
			check: func(t *testing.T, root ir.Node) {
				fenced := requireKind[*ir.Fenced](t, rowItems(t, root)[0])
				if !fenced.Balanced || fenced.Open == nil || fenced.Close == nil {
					t.Errorf("fence = balanced:%v open:%T close:%T", fenced.Balanced, fenced.Open, fenced.Close)
				}
			},
		},
		{
			source: `\left.x\right.`,
			check: func(t *testing.T, root ir.Node) {
				fenced := requireKind[*ir.Fenced](t, rowItems(t, root)[0])
				if !fenced.Balanced {
					t.Error("null-delimited fence should be balanced")
				}
				if fenced.Open == nil || fenced.Close == nil {
					t.Error("null delimiters must use zero-ink IR placeholders with the current IR contract")
				}
			},
		},
		{
			source: `\hat{x}`,
			check: func(t *testing.T, root ir.Node) {
				accent := requireKind[*ir.Accent](t, rowItems(t, root)[0])
				if accent.Position != ir.PositionAbove || accent.Body == nil || accent.Accent == nil {
					t.Errorf("accent was not lowered: %#v", accent)
				}
			},
		},
		{
			source: `\text{hello world}`,
			check: func(t *testing.T, root ir.Node) {
				text := collectText(root)
				if text != "hello world" {
					t.Errorf("text = %q", text)
				}
			},
		},
		{
			source: `\mathbf{x}`,
			check: func(t *testing.T, root ir.Node) {
				var found bool
				ir.Walk(root, func(node ir.Node) bool {
					if glyph, ok := node.(*ir.Glyph); ok {
						found = true
						if glyph.Variant != ir.VariantBold {
							t.Errorf("variant = %v, want bold", glyph.Variant)
						}
					}
					return true
				})
				if !found {
					t.Error("missing glyph")
				}
			},
		},
		{
			source: `\displaystyle x`,
			check: func(t *testing.T, root ir.Node) {
				style := requireKind[*ir.Style](t, rowItems(t, root)[0])
				if !style.SetSize || style.NewSize != ir.SizeDisplay {
					t.Errorf("style = %#v", style)
				}
			},
		},
		{
			source: `\sum_{i=1}^{n}`,
			check: func(t *testing.T, root ir.Node) {
				scripts := requireKind[*ir.Scripts](t, rowItems(t, root)[0])
				operator := requireKind[*ir.Operator](t, scripts.BaseItem)
				if operator.Text != "∑" || operator.Limits != ir.LimitsAuto {
					t.Errorf("operator = %#v", operator)
				}
			},
		},
		{
			source: `\lim_{x\to0}`,
			check: func(t *testing.T, root ir.Node) {
				scripts := requireKind[*ir.Scripts](t, rowItems(t, root)[0])
				operator := requireKind[*ir.Operator](t, scripts.BaseItem)
				if operator.Limits != ir.LimitsAuto {
					t.Errorf("lim limits = %v, want auto", operator.Limits)
				}
			},
		},
		{
			source: `\sum\nolimits_{i=1}^n`,
			check: func(t *testing.T, root ir.Node) {
				scripts := requireKind[*ir.Scripts](t, rowItems(t, root)[0])
				if scripts.BaseItem.NodeBase().Limits != ir.LimitsNever {
					t.Errorf("limits = %v, want never", scripts.BaseItem.NodeBase().Limits)
				}
			},
		},
		{
			source: `\operatorname{sin} x`,
			check: func(t *testing.T, root ir.Node) {
				items := rowItems(t, root)
				operator := requireKind[*ir.Operator](t, items[0])
				if operator.Text != "sin" {
					t.Errorf("operator = %q", operator.Text)
				}
			},
		},
		{
			source: `a\,\quad b`,
			check: func(t *testing.T, root ir.Node) {
				items := rowItems(t, root)
				if items[1].NodeBase().Kind() != ir.KindSpace || items[2].NodeBase().Kind() != ir.KindSpace {
					t.Errorf("spacing nodes were not lowered")
				}
			},
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.source, func(t *testing.T) {
			t.Parallel()
			result := parseTest(t, tc.source, Options{})
			if len(result.Diagnostics) != 0 {
				t.Fatalf("unexpected diagnostics: %v", result.Diagnostics)
			}
			tc.check(t, result.Root)
		})
	}
}

func TestFractionPrecedence(t *testing.T) {
	t.Parallel()

	result := parseTest(t, `a+b \over c+d`, Options{})
	items := rowItems(t, result.Root)
	if len(items) != 1 {
		t.Fatalf("got %d top-level items, want 1", len(items))
	}
	fraction := requireKind[*ir.Fraction](t, items[0])
	if n := requireKind[*ir.Row](t, fraction.Numerator); len(n.Items) != 3 {
		t.Errorf("numerator items = %d, want 3", len(n.Items))
	}
	if d := requireKind[*ir.Row](t, fraction.Denominator); len(d.Items) != 3 {
		t.Errorf("denominator items = %d, want 3", len(d.Items))
	}
}

func collectText(node ir.Node) string {
	var b strings.Builder
	ir.Walk(node, func(n ir.Node) bool {
		if text, ok := n.(*ir.Text); ok {
			b.WriteString(text.Text)
		}
		return true
	})
	return b.String()
}
