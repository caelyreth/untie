package tex

import (
	"testing"

	"github.com/caelyreth/untie/ir"
)

func TestMathNormalUsesAutomaticMathAlphabet(t *testing.T) {
	t.Parallel()

	result := parseTest(t, `\mathnormal{x}`, Options{})
	if len(result.Diagnostics) != 0 {
		t.Fatalf("diagnostics: %v", result.Diagnostics)
	}
	var glyph *ir.Glyph
	ir.Walk(result.Root, func(node ir.Node) bool {
		if candidate, ok := node.(*ir.Glyph); ok && glyph == nil {
			glyph = candidate
		}
		return glyph == nil
	})
	if glyph == nil {
		t.Fatal("mathnormal body has no glyph")
	}
	if glyph.Variant != ir.VariantAuto {
		t.Fatalf("mathnormal variant = %v, want auto", glyph.Variant)
	}
}

func TestOperatorsUseUprightVariant(t *testing.T) {
	t.Parallel()

	for _, src := range []string{`\sin x`, `\lim_{x\to0}`, `\operatorname{rank} A`, `\sum_i`} {
		t.Run(src, func(t *testing.T) {
			result := parseTest(t, src, Options{})
			var operators int
			ir.Walk(result.Root, func(node ir.Node) bool {
				if operator, ok := node.(*ir.Operator); ok {
					operators++
					if operator.Variant != ir.VariantNormal {
						t.Errorf("%q variant = %v, want normal", operator.Text, operator.Variant)
					}
				}
				return true
			})
			if operators == 0 {
				t.Fatal("formula has no operator node")
			}
		})
	}
}

func TestScriptsCarryNeutralLimitPlacement(t *testing.T) {
	t.Parallel()

	tests := []struct {
		source string
		top    bool
		bottom bool
		limits ir.LimitsPlacement
		base   ir.NodeKind
	}{
		{source: "x^2", top: true, limits: ir.LimitsAuto, base: ir.KindGlyph},
		{source: "x_i", bottom: true, limits: ir.LimitsAuto, base: ir.KindGlyph},
		{source: `\sum^n_i`, top: true, bottom: true, limits: ir.LimitsAuto, base: ir.KindOperator},
		{source: `\sum\limits^n_i`, top: true, bottom: true, limits: ir.LimitsAlways, base: ir.KindOperator},
		{source: `\sum\nolimits^n_i`, top: true, bottom: true, limits: ir.LimitsNever, base: ir.KindOperator},
		{source: `\int^n_i`, top: true, bottom: true, limits: ir.LimitsNever, base: ir.KindOperator},
		{source: `\sin_i`, bottom: true, limits: ir.LimitsNever, base: ir.KindOperator},
		{source: `\lim_{x\to0}`, bottom: true, limits: ir.LimitsAuto, base: ir.KindOperator},
	}
	for _, tc := range tests {
		tc := tc
		t.Run(tc.source, func(t *testing.T) {
			t.Parallel()
			result := parseTest(t, tc.source, Options{})
			if len(result.Diagnostics) != 0 {
				t.Fatalf("diagnostics: %v", result.Diagnostics)
			}
			scripts := requireKind[*ir.Scripts](t, rowItems(t, result.Root)[0])
			if scripts.Top != nil != tc.top {
				t.Errorf("Top present = %v, want %v", scripts.Top != nil, tc.top)
			}
			if scripts.Bottom != nil != tc.bottom {
				t.Errorf("Bottom present = %v, want %v", scripts.Bottom != nil, tc.bottom)
			}
			if scripts.TopLeft != nil || scripts.BottomLeft != nil || scripts.TopRight != nil || scripts.BottomRight != nil {
				t.Error("generic scripts were lowered into final side-placement fields")
			}
			if scripts.Limits != tc.limits {
				t.Errorf("Scripts limits = %v, want %v", scripts.Limits, tc.limits)
			}
			if scripts.BaseItem.NodeBase().Limits != tc.limits {
				t.Errorf("base limits = %v, want %v", scripts.BaseItem.NodeBase().Limits, tc.limits)
			}
			if scripts.BaseItem.NodeBase().Kind() != tc.base {
				t.Errorf("base kind = %v, want %v", scripts.BaseItem.NodeBase().Kind(), tc.base)
			}
		})
	}
}
