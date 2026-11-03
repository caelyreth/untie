package untie

import (
	"strings"
	"testing"

	"github.com/caelyreth/untie/source"
)

func TestChemistryTeXAltAndLayout(t *testing.T) {
	t.Parallel()

	fonts := loadTestFonts(t)
	for _, source := range []string{
		`\ce{2H2 + O2 -> 2H2O}`,
		`\ce{^{227}_{90}Th+}`,
		`\ce{A ->[heat][cat] B}`,
		`\pu{123 kJ//mol}`,
	} {
		expr := ParseTeX(source, Display, ParseOptions{})
		if expr == nil {
			t.Fatalf("%s parsed to nil", source)
		}
		for _, diagnostic := range expr.Diagnostics() {
			if diagnostic.Severity == SeverityError {
				t.Fatalf("%s diagnostics = %+v", source, expr.Diagnostics())
			}
		}
		alt := expr.Alt()
		if alt.Text == "" || alt.Speech == "" {
			t.Fatalf("%s Alt = %#v", source, alt)
		}
		box, err := Layout(expr, fonts, Style{Size: 16, GenerateAlt: true})
		if err != nil {
			t.Fatalf("%s Layout() error = %v", source, err)
		}
		if err := box.Validate(); err != nil {
			t.Fatalf("%s box invalid: %v", source, err)
		}
		if box.Width <= 0 || box.Ascent <= 0 || len(box.Items) == 0 {
			t.Fatalf("%s box = %#v", source, box)
		}
		if !box.Alt.Equal(alt) {
			t.Fatalf("%s Box.Alt = %#v, want %#v", source, box.Alt, alt)
		}
	}
}

func TestChemistryAltPreservesFormulaContent(t *testing.T) {
	t.Parallel()

	alt := ParseTeX(`\ce{2H2 + O2 -> 2H2O}`, Display, ParseOptions{}).Alt()
	for _, want := range []string{"2", "H", "O"} {
		if !strings.Contains(alt.Text, want) {
			t.Fatalf("Alt.Text = %q, want %q", alt.Text, want)
		}
	}
}

func TestChemistryUnsupportedSyntaxRemainsVisible(t *testing.T) {
	t.Parallel()

	fonts := loadTestFonts(t)
	expr := ParseTeX(`\ce{A \bond{~} B}`, Display, ParseOptions{})
	found := false
	for _, diagnostic := range expr.Diagnostics() {
		if diagnostic.Code == source.CodeUnsupported {
			found = true
		}
	}
	if !found {
		t.Fatalf("diagnostics = %+v, want unsupported", expr.Diagnostics())
	}
	box, err := Layout(expr, fonts, Style{Size: 16})
	if err != nil {
		t.Fatalf("Layout() error = %v", err)
	}
	if len(box.Items) == 0 {
		t.Fatal("unsupported chemistry produced no visible output")
	}
}
