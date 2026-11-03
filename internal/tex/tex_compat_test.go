package tex

import (
	"testing"

	"github.com/caelyreth/untie/ir"
	"github.com/caelyreth/untie/source"
)

func TestSizingDeclarationsScopeToTheRemainingGroup(t *testing.T) {
	t.Parallel()

	result := parseTest(t, `{\tiny a}b`, Options{})
	items := rowItems(t, result.Root)
	if len(items) != 2 {
		t.Fatalf("root items = %d, want sized group and unsized trailing glyph", len(items))
	}
	group := requireKind[*ir.Row](t, items[0])
	if len(group.Items) != 1 {
		t.Fatalf("sized group items = %d, want one style", len(group.Items))
	}
	style := requireKind[*ir.Style](t, group.Items[0])
	if style.FontSize.Float() != 50 || style.NewSize != ir.SizeText {
		t.Fatalf("grouped size = %+v/%v, want 50%%/text", style.FontSize, style.NewSize)
	}
	if glyph := requireKind[*ir.Glyph](t, items[1]); glyph.Text != "b" {
		t.Fatalf("trailing glyph = %q, want b", glyph.Text)
	}
}

func TestNestedSizingDeclarationsReplaceRatherThanMultiply(t *testing.T) {
	t.Parallel()

	result := parseTest(t, `\Huge a\small b`, Options{})
	outer := requireKind[*ir.Style](t, rowItems(t, result.Root)[0])
	if outer.FontSize.Float() != 248.8 {
		t.Fatalf("outer size = %v, want 248.8%%", outer.FontSize.Float())
	}
	body := rowItems(t, outer.Body)
	var inner *ir.Style
	for _, node := range body {
		if candidate, ok := node.(*ir.Style); ok {
			inner = candidate
		}
	}
	if inner == nil || inner.FontSize.Float() != 90 {
		t.Fatalf("inner size = %+v, want absolute 90%%", inner)
	}
}

func TestSizingInsideScriptsPreservesScriptStructure(t *testing.T) {
	t.Parallel()

	result := parseTest(t, `\small x^2`, Options{})
	style := requireKind[*ir.Style](t, rowItems(t, result.Root)[0])
	inner := rowItems(t, style.Body)
	scripts := requireKind[*ir.Scripts](t, inner[0])
	if scripts.Top == nil || scripts.BaseItem == nil {
		t.Fatalf("scripts = %+v, want base and superscript", scripts)
	}
	if style.FontSize.Float() != 90 {
		t.Fatalf("script sizing = %v, want 90%%", style.FontSize.Float())
	}
}

func TestOverlapCommandsUseNeutralAlignmentAndMode(t *testing.T) {
	t.Parallel()

	tests := []struct {
		source    string
		alignment ir.OverlayAlignment
		textMode  bool
	}{
		{source: `\mathllap{x}`, alignment: ir.OverlayEnd},
		{source: `\mathrlap{x}`, alignment: ir.OverlayStart},
		{source: `\mathclap{x}`, alignment: ir.OverlayCenter},
		{source: `\llap{x}`, alignment: ir.OverlayEnd, textMode: true},
		{source: `\rlap{x}`, alignment: ir.OverlayStart, textMode: true},
		{source: `\clap{x}`, alignment: ir.OverlayCenter, textMode: true},
	}
	for _, test := range tests {
		test := test
		t.Run(test.source, func(t *testing.T) {
			t.Parallel()
			result := parseTest(t, test.source, Options{})
			for _, diagnostic := range result.Diagnostics {
				if diagnostic.Severity == source.SeverityError {
					t.Fatalf("diagnostics = %v", result.Diagnostics)
				}
			}
			overlay := requireKind[*ir.Overlay](t, rowItems(t, result.Root)[0])
			if overlay.Alignment != test.alignment {
				t.Fatalf("alignment = %v, want %v", overlay.Alignment, test.alignment)
			}
			foundText := false
			foundGlyph := false
			ir.Walk(overlay.Body, func(node ir.Node) bool {
				switch node.(type) {
				case *ir.Text:
					foundText = true
				case *ir.Glyph:
					foundGlyph = true
				}
				return true
			})
			if test.textMode && (!foundText || foundGlyph) {
				t.Fatalf("text-mode overlap text/glyph = %t/%t", foundText, foundGlyph)
			}
			if !test.textMode && foundText {
				t.Fatal("math-mode overlap unexpectedly used the text parser")
			}
		})
	}
}

func TestRaiseBoxUsesTextModeBodyAndSignedLength(t *testing.T) {
	t.Parallel()

	tests := []struct {
		source string
		unit   ir.LengthUnit
		value  float64
	}{
		{source: `\raisebox{2.5pt}{text}`, unit: ir.LengthPt, value: 2.5},
		{source: `\raisebox{-0.25ex}{text}`, unit: ir.LengthEx, value: -0.25},
	}
	for _, test := range tests {
		test := test
		t.Run(test.source, func(t *testing.T) {
			t.Parallel()
			result := parseTest(t, test.source, Options{})
			for _, diagnostic := range result.Diagnostics {
				if diagnostic.Severity == source.SeverityError {
					t.Fatalf("diagnostics = %v", result.Diagnostics)
				}
			}
			raise := requireKind[*ir.Raise](t, rowItems(t, result.Root)[0])
			if raise.Amount.Unit != test.unit || raise.Amount.Float() != test.value {
				t.Fatalf("amount = %+v, want %v %v", raise.Amount, test.value, test.unit)
			}
			foundText := false
			foundGlyph := false
			ir.Walk(raise.Body, func(node ir.Node) bool {
				switch node.(type) {
				case *ir.Text:
					foundText = true
				case *ir.Glyph:
					foundGlyph = true
				}
				return true
			})
			if !foundText || foundGlyph {
				t.Fatalf("raise body text/glyph = %t/%t, want text-only hbox", foundText, foundGlyph)
			}
		})
	}
}

func TestRaiseBoxRejectsMathOnlyBody(t *testing.T) {
	t.Parallel()

	result := parseTest(t, `\raisebox{1pt}{\frac12}`, Options{})
	if !hasDiagnostic(result.Diagnostics, source.CodeUnsupported) {
		t.Fatalf("diagnostics = %v, want text-mode unsupported diagnostic", diagnosticCodes(result.Diagnostics))
	}
	if got := rowItems(t, result.Root)[0].Kind(); got != ir.KindRaise {
		t.Fatalf("root kind = %v, want raise", got)
	}
}

func TestRaiseBoxAndOverlapWorkInsideTextMode(t *testing.T) {
	t.Parallel()

	result := parseTest(t, `\text{a\raisebox{1pt}{b}\clap{c}d}`, Options{})
	for _, diagnostic := range result.Diagnostics {
		if diagnostic.Severity == source.SeverityError {
			t.Fatalf("text-mode diagnostics = %v", result.Diagnostics)
		}
	}
	var raise *ir.Raise
	var overlay *ir.Overlay
	ir.Walk(result.Root, func(node ir.Node) bool {
		switch node := node.(type) {
		case *ir.Raise:
			raise = node
		case *ir.Overlay:
			overlay = node
		}
		return true
	})
	if raise == nil || overlay == nil || overlay.Alignment != ir.OverlayCenter {
		t.Fatalf("text-mode raise/overlay = %+v/%+v", raise, overlay)
	}
}
