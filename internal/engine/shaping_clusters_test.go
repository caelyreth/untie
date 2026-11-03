package engine

import (
	"context"
	"errors"
	"os"
	"reflect"
	"testing"

	"github.com/caelyreth/untie/font"
	"github.com/caelyreth/untie/geometry"
	"github.com/caelyreth/untie/internal/layout"
	"github.com/caelyreth/untie/ir"
	"github.com/caelyreth/untie/source"
)

func TestShapeTextPreservesClustersAndPerClusterFallback(t *testing.T) {
	t.Parallel()

	adapter := newClusteredShapingAdapter(t, source.Limits{})
	const text = "A👩‍💻B"
	run, err := adapter.ShapeText(layout.TextRequest{
		Context: context.Background(),
		Text:    text,
		Style:   layout.Style{Size: layout.Text},
		Variant: ir.VariantAuto,
		Font:    ir.FontText,
	})
	if err != nil {
		t.Fatalf("ShapeText: %v", err)
	}
	wantClusters := []layout.TextCluster{
		{RuneStart: 0, RuneEnd: 1, Source: source.NewRange(0, 1)},
		{RuneStart: 1, RuneEnd: 4, Source: source.NewRange(1, 12)},
		{RuneStart: 4, RuneEnd: 5, Source: source.NewRange(12, 13)},
	}
	if !reflect.DeepEqual(run.Clusters, wantClusters) {
		t.Fatalf("clusters = %#v, want %#v", run.Clusters, wantClusters)
	}
	if len(run.Glyphs) != 3 {
		t.Fatalf("glyph count = %d, want 3", len(run.Glyphs))
	}
	wantFonts := []string{"NewCMMath-Book", "NotoEmoji-ShapingSubset", "NewCMMath-Book"}
	for i, positioned := range run.Glyphs {
		if positioned.Cluster != i {
			t.Errorf("glyph %d cluster = %d, want %d", i, positioned.Cluster, i)
		}
		if positioned.Glyph.FontID != wantFonts[i] {
			t.Errorf("glyph %d font = %q, want %q", i, positioned.Glyph.FontID, wantFonts[i])
		}
		if i > 0 && positioned.At.X <= run.Glyphs[i-1].At.X {
			t.Errorf("glyph positions are not visual-order increasing: %+v", run.Glyphs)
		}
	}
}

func TestShapeTextTracksShapingClusters(t *testing.T) {
	t.Parallel()

	adapter := newClusteredShapingAdapter(t, source.Limits{})
	shape := func(tracking geometry.Em) layout.TextRun {
		t.Helper()
		run, err := adapter.ShapeText(layout.TextRequest{
			Context:  context.Background(),
			Text:     "कर्मक्ष",
			Style:    layout.Style{Size: layout.Text},
			Variant:  ir.VariantAuto,
			Font:     ir.FontText,
			Tracking: tracking,
		})
		if err != nil {
			t.Fatalf("ShapeText: %v", err)
		}
		return run
	}
	base := shape(0)
	tracked := shape(geometry.Em(geometry.EmScale / 10))
	if len(base.Clusters) != 3 || len(base.Glyphs) != 4 {
		t.Fatalf("base clusters/glyphs = %d/%d, want 3/4", len(base.Clusters), len(base.Glyphs))
	}
	wantDelta, err := geometry.EmFromParts(3, 10)
	if err != nil {
		t.Fatal(err)
	}
	if got := tracked.Width - base.Width; !closeEm(got, wantDelta) {
		t.Fatalf("tracked width delta = %d, want %d (one tracking step per cluster)", got, wantDelta)
	}
}

func TestShapeTextKeepsRTLRunsInVisualOrder(t *testing.T) {
	t.Parallel()

	adapter := newClusteredShapingAdapter(t, source.Limits{})
	run, err := adapter.ShapeText(layout.TextRequest{
		Context: context.Background(),
		Text:    "سلام",
		Style:   layout.Style{Size: layout.Text},
		Variant: ir.VariantAuto,
		Font:    ir.FontText,
	})
	if err != nil {
		t.Fatalf("ShapeText: %v", err)
	}
	starts := make([]int, len(run.Clusters))
	if len(run.Glyphs) != len(run.Clusters) {
		t.Fatalf("RTL cluster/glyph count = %d/%d, want one glyph per Arabic cluster", len(run.Clusters), len(run.Glyphs))
	}
	for i, cluster := range run.Clusters {
		starts[i] = cluster.RuneStart
		if i > 0 && run.Glyphs[i].At.X <= run.Glyphs[i-1].At.X {
			t.Fatalf("RTL glyphs are not left-to-right visual order: %+v", run.Glyphs)
		}
	}
	if want := []int{3, 2, 1, 0}; !reflect.DeepEqual(starts, want) {
		t.Fatalf("cluster order = %v, want visual RTL order %v", starts, want)
	}
}

func TestShapeTextMapsStyledRunesBackToSource(t *testing.T) {
	t.Parallel()

	adapter := newClusteredShapingAdapter(t, source.Limits{})
	tests := []struct {
		name    string
		text    string
		variant ir.Variant
		runeEnd int
		source  source.Range
	}{
		{name: "single rune", text: "x", variant: ir.VariantItalic, runeEnd: 1, source: source.NewRange(0, 1)},
		{name: "styled selector", text: "A\uFE01", variant: ir.VariantScript, runeEnd: 2, source: source.NewRange(0, 4)},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			run, err := adapter.ShapeText(layout.TextRequest{
				Context: context.Background(),
				Text:    test.text,
				Style:   layout.Style{Size: layout.Text},
				Variant: test.variant,
				Font:    ir.FontMath,
			})
			if err != nil {
				t.Fatalf("ShapeText: %v", err)
			}
			if len(run.Clusters) != 1 || run.Clusters[0] != (layout.TextCluster{
				RuneStart: 0,
				RuneEnd:   test.runeEnd,
				Source:    test.source,
			}) {
				t.Fatalf("styled clusters = %#v", run.Clusters)
			}
			if len(run.Glyphs) != 1 || run.Glyphs[0].Glyph.Text != test.text {
				t.Fatalf("styled glyphs = %#v", run.Glyphs)
			}
		})
	}
}

func TestShapeTextMissingGlyphAndLimits(t *testing.T) {
	t.Parallel()

	adapter := newClusteredShapingAdapter(t, source.Limits{})
	run, err := adapter.ShapeText(layout.TextRequest{
		Context: context.Background(),
		Text:    "😀",
		Style:   layout.Style{Size: layout.Text},
		Variant: ir.VariantAuto,
		Font:    ir.FontText,
	})
	if err != nil {
		t.Fatalf("missing emoji error = %v", err)
	}
	if len(run.Clusters) != 1 || !run.Clusters[0].Missing {
		t.Fatalf("missing emoji clusters = %#v", run.Clusters)
	}
	if len(run.Glyphs) != 1 || !run.Glyphs[0].Missing {
		t.Fatalf("missing emoji glyphs = %#v", run.Glyphs)
	}

	limited := newClusteredShapingAdapter(t, source.Limits{MaxOutputItems: 1})
	if _, err := limited.ShapeText(layout.TextRequest{
		Context: context.Background(),
		Text:    "ab",
		Style:   layout.Style{Size: layout.Text},
		Variant: ir.VariantAuto,
		Font:    ir.FontText,
	}); !errors.Is(err, layout.ErrLimit) {
		t.Fatalf("glyph limit error = %v, want ErrLimit", err)
	}
}

func TestNamedOperatorsStayOnMathGSUBPath(t *testing.T) {
	t.Parallel()

	adapter := newClusteredShapingAdapter(t, source.Limits{})
	run, err := adapter.ShapeOperator(layout.OperatorRequest{
		Context: context.Background(),
		Text:    "sin",
		Style:   layout.Style{Size: layout.Text},
		Variant: ir.VariantAuto,
	})
	if err != nil {
		t.Fatalf("ShapeOperator: %v", err)
	}
	if len(run.Clusters) != 0 {
		t.Fatalf("operator unexpectedly used general text clusters: %#v", run.Clusters)
	}
	if len(run.Glyphs) != 3 {
		t.Fatalf("operator glyph count = %d, want 3", len(run.Glyphs))
	}
}

func newClusteredShapingAdapter(t testing.TB, limits source.Limits) *adapter {
	t.Helper()
	mathData, err := os.ReadFile("../../testdata/upstream/fonts/NewCMMath-Book.otf")
	if err != nil {
		t.Fatalf("read math font: %v", err)
	}
	emojiData, err := os.ReadFile("../../testdata/upstream/fonts/NotoEmoji-ShapingSubset.ttf")
	if err != nil {
		t.Fatalf("read emoji font: %v", err)
	}
	arabicData, err := os.ReadFile("../../testdata/upstream/fonts/NotoNaskhArabic-ShapingSubset.ttf")
	if err != nil {
		t.Fatalf("read Arabic font: %v", err)
	}
	devanagariData, err := os.ReadFile("../../testdata/upstream/fonts/NotoSansDevanagari-ShapingSubset.ttf")
	if err != nil {
		t.Fatalf("read Devanagari font: %v", err)
	}
	set, err := font.LoadFontSet(font.FontSetConfig{
		Math: font.FontFace{Bytes: mathData, ID: "NewCMMath-Book"},
		Fallbacks: []font.FontFace{
			{Bytes: emojiData, ID: "NotoEmoji-ShapingSubset"},
			{Bytes: arabicData, ID: "NotoNaskhArabic-ShapingSubset"},
			{Bytes: devanagariData, ID: "NotoSansDevanagari-ShapingSubset"},
		},
	})
	if err != nil {
		t.Fatalf("LoadFontSet: %v", err)
	}
	adapter, err := newAdapter(set, limits)
	if err != nil {
		t.Fatalf("newAdapter: %v", err)
	}
	return adapter
}
