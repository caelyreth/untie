package crosscheck

import (
	"context"
	"reflect"
	"testing"

	"github.com/caelyreth/untie"
	"github.com/caelyreth/untie/internal/fixture"
	"github.com/caelyreth/untie/ir"
	"github.com/caelyreth/untie/source"
)

func TestNormalizeFormulaIncludesEquationTagMetadata(t *testing.T) {
	t.Parallel()

	span := source.NewRange(0, 2)
	formula := &ir.Formula{
		Root: ir.NewGlyph(source.NewRange(0, 1), "x"),
		Tags: []ir.EquationTag{{
			Body:   ir.NewGlyph(source.NewRange(1, 2), "1"),
			Span:   span,
			Origin: span,
			Manual: true,
			Line:   0,
		}},
	}
	plain := &ir.Formula{Root: ir.NewGlyph(source.NewRange(0, 1), "x")}
	normalized, err := normalizeFormula(formula, ir.Display)
	if err != nil {
		t.Fatalf("normalizeFormula() error = %v", err)
	}
	withoutTags, err := normalizeFormula(plain, ir.Display)
	if err != nil {
		t.Fatalf("normalizeFormula(plain) error = %v", err)
	}
	if reflect.DeepEqual(normalized, withoutTags) {
		t.Fatal("normalization erased formula tag metadata")
	}
	object, ok := normalized.(map[string]any)
	if !ok || object["tags"] == nil {
		t.Fatalf("normalized formula = %#v, want root/tags object", normalized)
	}

	changed := *formula
	changed.Tags = append([]ir.EquationTag(nil), formula.Tags...)
	changed.Tags[0].Starred = true
	normalizedChanged, err := normalizeFormula(&changed, ir.Display)
	if err != nil {
		t.Fatalf("normalizeFormula(changed) error = %v", err)
	}
	if reflect.DeepEqual(normalized, normalizedChanged) {
		t.Fatal("normalization ignored tag presentation metadata")
	}
}

func TestTeXEquationTagGeometryMatchesPinnedRelativeLayout(t *testing.T) {
	t.Parallel()

	fixtureRoot, _ := crosscheckPaths(t)
	corpus, err := LoadPairedCorpus(fixtureRoot)
	if err != nil {
		t.Fatalf("LoadPairedCorpus() error = %v", err)
	}
	var tagCase fixture.Case
	for _, testCase := range corpus.Cases {
		if testCase.ID == "tag" {
			tagCase = testCase
			break
		}
	}
	if tagCase.ID == "" || tagCase.TeX == "" || tagCase.Fixture == nil || tagCase.Fixture.Frame == nil {
		t.Fatal("paired tag fixture is missing its TeX source or static frame")
	}
	frame, err := fixture.FrameToDIP(*tagCase.Fixture.Frame)
	if err != nil {
		t.Fatalf("FrameToDIP() error = %v", err)
	}
	mode := ir.Inline
	if tagCase.Display {
		mode = ir.Display
	}
	expr := untie.ParseTeX(tagCase.TeX, mode, untie.ParseOptions{Limits: source.DefaultLimits()})
	formula := requireValidRoot(t, expr)
	if len(formula.Tags) != 1 {
		t.Fatalf("formula tags = %+v, want one", formula.Tags)
	}
	harness := testHarness(t, fixtureRoot)
	box, err := harness.layoutFormula(context.Background(), formula, mode, frame.FontSize)
	if err != nil {
		t.Fatalf("layoutFormula() error = %v", err)
	}
	if box.Tag == nil || box.Tag.Box == nil {
		t.Fatal("TeX tag layout omitted TagBox")
	}
	withoutTag := untie.ParseTeX("E=mc^2", mode, untie.ParseOptions{Limits: source.DefaultLimits()})
	untaggedFormula := requireValidRoot(t, withoutTag)
	untagged, err := harness.layoutFormula(context.Background(), untaggedFormula, mode, frame.FontSize)
	if err != nil {
		t.Fatalf("layoutFormula(untagged) error = %v", err)
	}
	if absFloat64(float64(box.Tag.X-untagged.Width)) > toleranceDIP {
		t.Fatalf("tag X = %v, want formula width %v", box.Tag.X, untagged.Width)
	}
	if absFloat64(float64(box.Width-(untagged.Width+box.Tag.Box.Width))) > toleranceDIP {
		t.Fatalf("tagged width = %v, want additive %v", box.Width, untagged.Width+box.Tag.Box.Width)
	}
	if len(box.Tag.Box.Items) != 3 {
		t.Fatalf("tag items = %d, want non-starred parentheses and body", len(box.Tag.Box.Items))
	}
	pinnedTagLeft := 68.132
	pinnedTagWidth := 82.19 - pinnedTagLeft
	for i, want := range []float64{0, (72.411 - pinnedTagLeft) / pinnedTagWidth, (77.911 - pinnedTagLeft) / pinnedTagWidth} {
		got := float64(box.Tag.Box.Items[i].At.X) / float64(box.Tag.Box.Width)
		if absFloat64(got-want) > 0.002 {
			t.Fatalf("tag item %d relative x = %v, want pinned ratio %v", i, got, want)
		}
	}
}

func absFloat64(value float64) float64 {
	if value < 0 {
		return -value
	}
	return value
}
