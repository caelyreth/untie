package layout

import (
	"context"
	"testing"

	"github.com/caelyreth/untie/display"
	"github.com/caelyreth/untie/geometry"
	"github.com/caelyreth/untie/ir"
	"github.com/caelyreth/untie/source"
)

type requestClusterTextSource struct{}

func (requestClusterTextSource) ShapeText(req TextRequest) (TextRun, error) {
	run := TextRun{}
	advance := em(1, 2)
	for i, cluster := range req.Clusters {
		run.Clusters = append(run.Clusters, TextCluster{
			RuneStart: i,
			RuneEnd:   i + 1,
			Source:    cluster.Source,
			X:         geometry.Em(int64(i) * int64(advance)),
			Advance:   advance,
		})
		run.Glyphs = append(run.Glyphs, PositionedGlyph{
			Glyph: Glyph{
				ID:            uint16(i + 1),
				FontID:        "cluster-test",
				Advance:       advance,
				Ascent:        em(7, 10),
				Descent:       em(2, 10),
				ExactGlyphRef: true,
			},
			At:      Point{X: geometry.Em(int64(i)*int64(advance) + int64(em(1, 10)))},
			Cluster: i,
		})
	}
	run.Width = geometry.Em(int64(len(req.Clusters)) * int64(advance))
	run.Ascent = em(7, 10)
	run.Descent = em(2, 10)
	return run, nil
}

type visualClusterTextSource struct {
	sources []source.Range
	rtl     []bool
	missing bool
}

func (s visualClusterTextSource) ShapeText(req TextRequest) (TextRun, error) {
	run := TextRun{Ascent: em(7, 10), Descent: em(2, 10)}
	advance := em(1, 2)
	for i, sourceRange := range s.sources {
		rtl := i < len(s.rtl) && s.rtl[i]
		run.Clusters = append(run.Clusters, TextCluster{
			RuneStart: i,
			RuneEnd:   i + 1,
			Source:    sourceRange,
			Missing:   s.missing,
			RTL:       rtl,
			X:         geometry.Em(int64(i) * int64(advance)),
			Advance:   advance,
		})
		run.Glyphs = append(run.Glyphs, PositionedGlyph{
			Glyph: Glyph{
				ID:            uint16(i + 1),
				FontID:        "visual-cluster-test",
				Advance:       advance,
				Ascent:        em(7, 10),
				Descent:       em(2, 10),
				ExactGlyphRef: true,
			},
			At:      Point{X: geometry.Em(int64(i) * int64(advance))},
			Cluster: i,
			Missing: s.missing,
		})
	}
	run.Width = geometry.Em(int64(len(s.sources)) * int64(advance))
	return run, nil
}

func TestTextClusterHitsFollowShapedVisualOrder(t *testing.T) {
	t.Parallel()

	renderer := newTextClusterRenderer(t, requestClusterTextSource{})
	text := ir.NewText(source.NewRange(0, 2), "ab")
	text.Clusters = []ir.TextClusterSource{
		{TextStart: 0, TextEnd: 1, Source: source.NewRange(10, 11)},
		{TextStart: 1, TextEnd: 2, Source: source.NewRange(12, 13)},
	}
	box, err := renderer.Layout(text, ir.Inline, testStyle(16))
	if err != nil {
		t.Fatalf("Layout() error = %v", err)
	}
	if len(box.Hits) != 2 {
		t.Fatalf("hits = %#v, want two cluster hits", box.Hits)
	}
	for i, hit := range box.Hits {
		if hit.Kind != display.HitAtom || hit.Source != text.Clusters[i].Source {
			t.Fatalf("hit %d = %#v, want atom source %v", i, hit, text.Clusters[i].Source)
		}
		if !closeHost(hit.Area.X, float32(i*8)) || !closeHost(hit.Area.Width, 8) {
			t.Fatalf("hit %d area = %#v, want x=%d width=8", i, hit.Area, i*8)
		}
	}
}

func TestTextClusterHitsPreserveRTLVisualOrder(t *testing.T) {
	t.Parallel()

	renderer := newTextClusterRenderer(t, visualClusterTextSource{sources: []source.Range{
		source.NewRange(3, 4),
		source.NewRange(2, 3),
		source.NewRange(1, 2),
		source.NewRange(0, 1),
	}})
	text := ir.NewText(source.NewRange(0, 4), "سلام")
	text.Clusters = []ir.TextClusterSource{
		{TextStart: 0, TextEnd: 2, Source: source.NewRange(0, 1)},
		{TextStart: 2, TextEnd: 4, Source: source.NewRange(1, 2)},
		{TextStart: 4, TextEnd: 6, Source: source.NewRange(2, 3)},
		{TextStart: 6, TextEnd: 8, Source: source.NewRange(3, 4)},
	}
	box, err := renderer.Layout(text, ir.Inline, testStyle(16))
	if err != nil {
		t.Fatalf("Layout() error = %v", err)
	}
	if len(box.Hits) != 4 {
		t.Fatalf("hits = %#v, want four cluster hits", box.Hits)
	}
	want := []source.Range{
		source.NewRange(3, 4),
		source.NewRange(2, 3),
		source.NewRange(1, 2),
		source.NewRange(0, 1),
	}
	for i, hit := range box.Hits {
		if hit.Source != want[i] {
			t.Fatalf("hit %d source = %v, want visual-order source %v", i, hit.Source, want[i])
		}
	}
}

func TestTextClusterHitMergesClustersWithSameSource(t *testing.T) {
	t.Parallel()

	renderer := newTextClusterRenderer(t, visualClusterTextSource{sources: []source.Range{
		source.NewRange(20, 24),
		source.NewRange(20, 24),
	}})
	text := ir.NewText(source.NewRange(0, 2), "ab")
	text.Clusters = []ir.TextClusterSource{{
		TextStart: 0,
		TextEnd:   2,
		Source:    source.NewRange(20, 24),
	}}
	box, err := renderer.Layout(text, ir.Inline, testStyle(16))
	if err != nil {
		t.Fatalf("Layout() error = %v", err)
	}
	if len(box.Hits) != 1 {
		t.Fatalf("hits = %#v, want one merged source hit", box.Hits)
	}
	if box.Hits[0].Source != (source.Range{Start: 20, End: 24}) ||
		!closeHost(box.Hits[0].Area.Width, 16) {
		t.Fatalf("merged hit = %#v", box.Hits[0])
	}
}

func TestManualTextUsesNodeLevelHitFallback(t *testing.T) {
	t.Parallel()

	renderer := newTextClusterRenderer(t, visualClusterTextSource{sources: []source.Range{source.NewRange(0, 1)}})
	text := ir.NewText(source.NewRange(20, 22), "ab")
	text.Origin = source.NewRange(30, 32)
	box, err := renderer.Layout(text, ir.Inline, testStyle(16))
	if err != nil {
		t.Fatalf("Layout() error = %v", err)
	}
	if len(box.Hits) != 1 || box.Hits[0].Source != text.Origin || box.Hits[0].Kind != display.HitAtom {
		t.Fatalf("manual text hits = %#v, want node-level origin fallback", box.Hits)
	}
}

func TestMissingTextClusterRetainsSourceRange(t *testing.T) {
	t.Parallel()

	renderer := newTextClusterRenderer(t, visualClusterTextSource{
		sources: []source.Range{source.NewRange(40, 44)},
		missing: true,
	})
	text := ir.NewText(source.NewRange(0, 4), "😀")
	text.Clusters = []ir.TextClusterSource{{
		TextStart: 0,
		TextEnd:   4,
		Source:    source.NewRange(40, 44),
	}}
	box, err := renderer.LayoutFormulaContextOptions(
		context.Background(),
		&ir.Formula{Root: text},
		ir.Inline,
		testStyle(16),
		LayoutOptions{RecoverMissingGlyph: true},
	)
	if err != nil {
		t.Fatalf("LayoutFormulaContextOptions() error = %v", err)
	}
	if len(box.Diagnostics) != 1 || box.Diagnostics[0].Range != (source.Range{Start: 40, End: 44}) {
		t.Fatalf("diagnostics = %#v, want missing cluster range", box.Diagnostics)
	}
	if len(box.Hits) != 1 || box.Hits[0].Source != (source.Range{Start: 40, End: 44}) {
		t.Fatalf("hits = %#v, want missing cluster range", box.Hits)
	}
}

func newTextClusterRenderer(t *testing.T, text TextSource) *Renderer {
	t.Helper()
	renderer, err := New(Fonts{
		Glyphs:  &fakeGlyphSource{glyphs: map[string]Glyph{}},
		Metrics: fakeStyleFont{},
		Text:    text,
	}, Options{})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	return renderer
}
