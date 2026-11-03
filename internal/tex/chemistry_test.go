package tex

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/caelyreth/untie/ir"
	"github.com/caelyreth/untie/source"
)

type chemistryFixture struct {
	Name        string   `json:"name"`
	Source      string   `json:"source"`
	Provenance  string   `json:"provenance"`
	Diagnostics []string `json:"diagnostics"`
}

func TestMhchemFixtureCorpus(t *testing.T) {
	t.Parallel()

	path := filepath.Join("..", "..", "testdata", "tex", "mhchem_cases.json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var cases []chemistryFixture
	if err := json.Unmarshal(data, &cases); err != nil {
		t.Fatal(err)
	}
	if len(cases) == 0 {
		t.Fatal("empty mhchem fixture corpus")
	}
	for _, testCase := range cases {
		testCase := testCase
		t.Run(testCase.Name, func(t *testing.T) {
			t.Parallel()
			if testCase.Provenance == "" {
				t.Fatal("fixture has no provenance")
			}
			result := parseTest(t, testCase.Source, Options{})
			got := make([]string, len(result.Diagnostics))
			for i, diagnostic := range result.Diagnostics {
				got[i] = string(diagnostic.Code)
			}
			if !slices.Equal(got, testCase.Diagnostics) {
				t.Fatalf("diagnostics = %v, want %v (%+v)", got, testCase.Diagnostics, result.Diagnostics)
			}
		})
	}
}

func TestMhchemFormulaStructure(t *testing.T) {
	t.Parallel()

	water := chemistryItems(t, parseTest(t, `\ce{H2O}`, Options{}))
	if len(water) != 2 {
		t.Fatalf("H2O items = %d, want 2", len(water))
	}
	hydrogen := requireKind[*ir.Scripts](t, water[0])
	if text := requireKind[*ir.Text](t, hydrogen.BaseItem); text.Text != "H" || text.Variant != ir.VariantNormal {
		t.Fatalf("hydrogen base = %#v", text)
	}
	if suffix := requireKind[*ir.Glyph](t, hydrogen.Bottom); suffix.Text != "2" {
		t.Fatalf("hydrogen suffix = %#v", suffix)
	}
	if oxygen := requireKind[*ir.Text](t, water[1]); oxygen.Text != "O" {
		t.Fatalf("oxygen = %#v", oxygen)
	}

	grouped := chemistryItems(t, parseTest(t, `\ce{(NH4)2S}`, Options{}))
	group := requireKind[*ir.Scripts](t, grouped[0])
	fence := requireKind[*ir.Fenced](t, group.BaseItem)
	if fence.Open == nil || fence.Close == nil || !fence.Balanced {
		t.Fatalf("group fence = %#v", fence)
	}
	if multiplier := requireKind[*ir.Glyph](t, group.Bottom); multiplier.Text != "2" {
		t.Fatalf("group multiplier = %#v", multiplier)
	}
}

func TestMhchemIsotopesChargesAndArrowLimits(t *testing.T) {
	t.Parallel()

	isotope := requireKind[*ir.Scripts](t, chemistryItems(t, parseTest(t, `\ce{^{227}_{90}Th+}`, Options{}))[0])
	if isotope.TopLeft == nil || isotope.BottomLeft == nil || isotope.Top == nil {
		t.Fatalf("isotope scripts = %#v", isotope)
	}
	if isotope.Role != ir.ScriptRoleOrdinary || isotope.Limits != ir.LimitsAuto {
		t.Fatalf("isotope role/limits = %v/%v", isotope.Role, isotope.Limits)
	}
	if got := sourceText(t, `\ce{^{227}_{90}Th+}`, isotope.TopLeft.NodeBase().Span); got != "227" {
		t.Fatalf("top-left source = %q", got)
	}

	const reaction = `\ce{A ->[heat][cat] B}`
	items := chemistryItems(t, parseTest(t, reaction, Options{}))
	var arrow *ir.Scripts
	for _, item := range items {
		if candidate, ok := item.(*ir.Scripts); ok && candidate.Role == ir.ScriptRoleLimits {
			arrow = candidate
			break
		}
	}
	if arrow == nil {
		t.Fatal("reaction arrow annotations did not lower to limits")
	}
	if arrow.Limits != ir.LimitsAlways || arrow.Top == nil || arrow.Bottom == nil {
		t.Fatalf("arrow limits = %v, top=%t, bottom=%t", arrow.Limits, arrow.Top != nil, arrow.Bottom != nil)
	}
	if got := sourceText(t, reaction, arrow.Top.NodeBase().Span); !strings.Contains(got, "heat") {
		t.Fatalf("arrow top source = %q", got)
	}
}

func TestMhchemSourceClustersAndPhysicalUnits(t *testing.T) {
	t.Parallel()

	const source = `\ce{Cl2}`
	result := parseTest(t, source, Options{})
	var text *ir.Text
	ir.Walk(result.Root, func(node ir.Node) bool {
		if candidate, ok := node.(*ir.Text); ok && candidate.Text == "Cl" {
			text = candidate
		}
		return true
	})
	if text == nil {
		t.Fatal("missing Cl text node")
	}
	if len(text.Clusters) != 2 {
		t.Fatalf("Cl clusters = %d, want 2", len(text.Clusters))
	}
	if first := sourceText(t, source, text.Clusters[0].Source); first != "C" {
		t.Fatalf("first cluster source = %q", first)
	}
	if second := sourceText(t, source, text.Clusters[1].Source); second != "l" {
		t.Fatalf("second cluster source = %q", second)
	}

	quantity := chemistryItems(t, parseTest(t, `\pu{123 kJ//mol}`, Options{}))
	if got := leafText(quantity[0]); got != "123" {
		t.Fatalf("quantity number = %q", got)
	}
	var units, slashes []string
	for _, item := range quantity {
		switch node := item.(type) {
		case *ir.Text:
			units = append(units, node.Text)
		case *ir.Glyph:
			if node.Text == "/" {
				slashes = append(slashes, node.Text)
			}
		}
	}
	if !slices.Equal(units, []string{"kJ", "mol"}) {
		t.Fatalf("quantity units = %v", units)
	}
	if !slices.Equal(slashes, []string{"/"}) {
		t.Fatalf("quantity separators = %v", slashes)
	}
}

func TestMhchemNodeLimitsApplyBeforeLowering(t *testing.T) {
	t.Parallel()

	limits := source.DefaultLimits()
	limits.MaxNodes = 4
	result := Parse(`\ce{H2SO4}`, ir.Display, Options{Limits: limits})
	if result.Root == nil {
		t.Fatal("nil root")
	}
	if !hasDiagnostic(result.Diagnostics, source.CodeLimitExceeded) {
		t.Fatalf("diagnostics = %v, want limit-exceeded", diagnosticCodes(result.Diagnostics))
	}
	if err := ir.Validate(&ir.Formula{Root: result.Root}, limits); err != nil {
		t.Fatalf("limited IR invalid: %v", err)
	}
}

func TestMhchemCanonicalValidationAndClone(t *testing.T) {
	t.Parallel()

	result := parseTest(t, `\ce{2H2 + ^{227}_{90}Th+ ->[\Delta][cat] 2H2O(s)}`, Options{})
	formula := &ir.Formula{Root: result.Root, Tags: result.Tags}
	original, err := ir.MarshalCanonical(formula)
	if err != nil {
		t.Fatalf("MarshalCanonical() error = %v", err)
	}
	cloned, err := ir.CloneFormula(formula)
	if err != nil {
		t.Fatalf("CloneFormula() error = %v", err)
	}
	roundTrip, err := ir.MarshalCanonical(cloned)
	if err != nil {
		t.Fatalf("MarshalCanonical(clone) error = %v", err)
	}
	if string(roundTrip) != string(original) {
		t.Fatal("chemistry canonical form changed after clone")
	}
}

func chemistryItems(t *testing.T, result Result) []ir.Node {
	t.Helper()
	root := rowItems(t, result.Root)
	if len(root) != 1 {
		t.Fatalf("root chemistry wrapper items = %d, want 1", len(root))
	}
	body, ok := root[0].(*ir.Row)
	if !ok {
		t.Fatalf("chemistry body = %T, want *ir.Row", root[0])
	}
	return body.Items
}

func sourceText(t *testing.T, source string, span source.Range) string {
	t.Helper()
	if span.IsDetached() || !span.IsValidFor(len(source)) {
		t.Fatalf("invalid source span %+v", span)
	}
	return source[span.Start:span.End]
}

func leafText(node ir.Node) string {
	var text strings.Builder
	ir.Walk(node, func(child ir.Node) bool {
		switch child := child.(type) {
		case *ir.Glyph:
			text.WriteString(child.Text)
		case *ir.Text:
			text.WriteString(child.Text)
		}
		return true
	})
	return text.String()
}
