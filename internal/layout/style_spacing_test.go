package layout

import (
	"testing"

	"github.com/caelyreth/untie/geometry"
	"github.com/caelyreth/untie/ir"
)

func TestEightStylesAndTransitions(t *testing.T) {
	t.Parallel()

	styles := make(map[Style]bool)
	for _, size := range []MathSize{Display, Text, Script, ScriptScript} {
		for _, cramped := range []bool{false, true} {
			style := Style{Size: size, Cramped: cramped}
			if err := style.Validate(); err != nil {
				t.Fatalf("Validate(%s) error = %v", style, err)
			}
			styles[style] = true
		}
	}
	if len(styles) != 8 {
		t.Fatalf("got %d styles, want 8", len(styles))
	}

	tests := []struct {
		name string
		got  Style
		want Style
	}{
		{"display superscript", Style{Size: Display}.Superscript(), Style{Size: Script}},
		{"text superscript", Style{Size: Text, Cramped: true}.Superscript(), Style{Size: Script, Cramped: true}},
		{"script superscript", Style{Size: Script}.Superscript(), Style{Size: ScriptScript}},
		{"script-script superscript", Style{Size: ScriptScript}.Superscript(), Style{Size: ScriptScript}},
		{"subscript cramped", Style{Size: Text}.Subscript(), Style{Size: Script, Cramped: true}},
		{"display numerator", Style{Size: Display}.Numerator(), Style{Size: Text}},
		{"text numerator", Style{Size: Text}.Numerator(), Style{Size: Script}},
		{"script numerator", Style{Size: Script}.Numerator(), Style{Size: ScriptScript}},
		{"denominator cramped", Style{Size: Text}.Denominator(), Style{Size: Script, Cramped: true}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if test.got != test.want {
				t.Fatalf("got %s, want %s", test.got, test.want)
			}
		})
	}
}

func TestMathSpacingMatrix(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		left, right ir.MathClass
		want        geometry.Em
	}{
		{"ordinary binary", ir.ClassNormal, ir.ClassBinary, MediumSpace},
		{"binary ordinary", ir.ClassBinary, ir.ClassNormal, MediumSpace},
		{"relation ordinary", ir.ClassRelation, ir.ClassNormal, ThickSpace},
		{"ordinary relation", ir.ClassNormal, ir.ClassRelation, ThickSpace},
		{"relation relation", ir.ClassRelation, ir.ClassRelation, 0},
		{"punctuation ordinary", ir.ClassPunctuation, ir.ClassNormal, ThinSpace},
		{"ordinary punctuation", ir.ClassNormal, ir.ClassPunctuation, 0},
		{"opening ordinary", ir.ClassOpening, ir.ClassNormal, 0},
		{"ordinary closing", ir.ClassNormal, ir.ClassClosing, 0},
		{"large opening", ir.ClassLarge, ir.ClassOpening, 0},
		{"large ordinary", ir.ClassLarge, ir.ClassNormal, ThinSpace},
		{"space ordinary", ir.ClassSpace, ir.ClassNormal, 0},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := MathSpacing(test.left, test.right, Style{Size: Text}); got != test.want {
				t.Fatalf("MathSpacing() = %d, want %d", got, test.want)
			}
		})
	}

	for _, style := range []Style{
		{Size: Script},
		{Size: Script, Cramped: true},
		{Size: ScriptScript},
		{Size: ScriptScript, Cramped: true},
	} {
		for _, pair := range []struct {
			left, right ir.MathClass
		}{
			{ir.ClassNormal, ir.ClassLarge},
			{ir.ClassLarge, ir.ClassNormal},
			{ir.ClassLarge, ir.ClassLarge},
			{ir.ClassClosing, ir.ClassLarge},
			{ir.ClassInner, ir.ClassLarge},
		} {
			if got := MathSpacing(pair.left, pair.right, style); got != ThinSpace {
				t.Fatalf("%s: tight operator spacing = %d, want %d", style, got, ThinSpace)
			}
		}
		for _, pair := range []struct {
			left, right ir.MathClass
		}{
			{ir.ClassNormal, ir.ClassBinary},
			{ir.ClassRelation, ir.ClassNormal},
			{ir.ClassPunctuation, ir.ClassNormal},
			{ir.ClassOpening, ir.ClassLarge},
			{ir.ClassLarge, ir.ClassClosing},
		} {
			if got := MathSpacing(pair.left, pair.right, style); got != 0 {
				t.Fatalf("%s: unexpected tight spacing = %d", style, got)
			}
		}
	}

	for _, size := range []MathSize{Display, Text} {
		for _, cramped := range []bool{false, true} {
			style := Style{Size: size, Cramped: cramped}
			if got := MathSpacing(ir.ClassNormal, ir.ClassBinary, style); got != MediumSpace {
				t.Fatalf("%s: binary spacing = %d, want %d", style, got, MediumSpace)
			}
		}
	}

	classes := []ir.MathClass{
		ir.ClassNormal,
		ir.ClassLarge,
		ir.ClassBinary,
		ir.ClassRelation,
		ir.ClassOpening,
		ir.ClassClosing,
		ir.ClassPunctuation,
		ir.ClassInner,
	}
	wantMatrix := [8][8]geometry.Em{
		{0, ThinSpace, MediumSpace, ThickSpace, 0, 0, 0, ThinSpace},
		{ThinSpace, ThinSpace, 0, ThickSpace, 0, 0, 0, ThinSpace},
		{MediumSpace, MediumSpace, 0, 0, MediumSpace, 0, 0, MediumSpace},
		{ThickSpace, ThickSpace, 0, 0, ThickSpace, 0, 0, ThickSpace},
		{0, 0, 0, 0, 0, 0, 0, 0},
		{0, ThinSpace, MediumSpace, ThickSpace, 0, 0, 0, ThinSpace},
		{ThinSpace, ThinSpace, 0, ThinSpace, ThinSpace, ThinSpace, ThinSpace, ThinSpace},
		{ThinSpace, ThinSpace, MediumSpace, ThickSpace, ThinSpace, 0, ThinSpace, ThinSpace},
	}
	for left, leftClass := range classes {
		for right, rightClass := range classes {
			if got, want := MathSpacing(leftClass, rightClass, Style{Size: Text}), wantMatrix[left][right]; got != want {
				t.Fatalf("MathSpacing(%d, %d) = %d, want %d", leftClass, rightClass, got, want)
			}
		}
	}
}

func TestNormalizeClass(t *testing.T) {
	t.Parallel()

	ordinary, relation, binary := ir.ClassNormal, ir.ClassRelation, ir.ClassBinary
	tests := []struct {
		name     string
		previous *ir.MathClass
		current  ir.MathClass
		want     ir.MathClass
	}{
		{"binary at start", nil, ir.ClassBinary, ir.ClassUnary},
		{"binary after ordinary", &ordinary, ir.ClassBinary, ir.ClassBinary},
		{"binary after relation", &relation, ir.ClassBinary, ir.ClassUnary},
		{"varying after ordinary", &ordinary, ir.ClassVarying, ir.ClassBinary},
		{"varying after binary", &binary, ir.ClassVarying, ir.ClassUnary},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := NormalizeClass(test.previous, test.current); got != test.want {
				t.Fatalf("NormalizeClass() = %d, want %d", got, test.want)
			}
		})
	}
}
