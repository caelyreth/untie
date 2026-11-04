package untie

import (
	"strings"
	"testing"
)

func TestFeynmanSlashLayoutAndAlt(t *testing.T) {
	t.Parallel()

	fonts := loadTestFonts(t)
	for _, source := range []string{
		`\slashed{p}`,
		`\slashed{D}`,
		`\slashed{\partial}`,
		`\frac{1}{\slashed{p}-m}`,
		`\slashed{A}\quad\slashed{k}\quad\slashed{f}`,
	} {
		expr := ParseTeX(source, Display, ParseOptions{})
		if len(expr.Diagnostics()) != 0 {
			t.Fatalf("%s diagnostics = %+v", source, expr.Diagnostics())
		}
		if alt := expr.Alt(); !strings.Contains(alt.Text, "slashed(") ||
			!strings.Contains(alt.Speech, "slashed") {
			t.Fatalf("%s Alt = %#v", source, alt)
		}
		box, err := Layout(expr, fonts, Style{Size: 16, GenerateAlt: true})
		if err != nil {
			t.Fatalf("%s Layout() error = %v", source, err)
		}
		if err := box.Validate(); err != nil {
			t.Fatalf("%s box invalid: %v", source, err)
		}
		if box.Width <= 0 || len(box.Items) == 0 {
			t.Fatalf("%s box = %#v", source, box)
		}
	}
}
