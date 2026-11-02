package layout

import (
	"context"
	"errors"
	"testing"

	"github.com/caelyreth/untie/display"
	"github.com/caelyreth/untie/geometry"
	"github.com/caelyreth/untie/ir"
	"github.com/caelyreth/untie/source"
)

func TestTableRequestPreservesStyleLinesAndCellStyle(t *testing.T) {
	t.Parallel()

	glyphs := &fakeGlyphSource{glyphs: map[string]Glyph{}}
	tables := &fakeTableLayouter{}
	renderer, err := New(Fonts{Glyphs: glyphs, Metrics: fakeStyleFont{}}, Options{Tables: tables})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	table := ir.NewTable(source.NewRange(0, 3), ir.TableRow{
		Cells: []ir.Node{glyph("x", ir.ClassNormal)},
	})
	table.Horizontal = []ir.TableLine{{Position: 0, Count: 1, Dashed: true, Thickness: irEm(0.02)}}
	table.Vertical = []ir.TableLine{{Position: 1, Count: 2, Thickness: irEm(0.03)}}
	style := ir.NewStyle(source.NewRange(0, 4), table)
	style.SetSize = true
	style.NewSize = ir.SizeScript
	style.SetCramped = true
	style.Cramped = true

	if _, err := renderer.Layout(style, ir.Inline, testStyle(16)); err != nil {
		t.Fatalf("Layout() error = %v", err)
	}
	if !tables.called {
		t.Fatal("table extension was not called")
	}
	if tables.req.Context == nil {
		t.Fatal("table request has nil context")
	}
	if tables.req.Limits.MaxOutputItems == 0 {
		t.Fatal("table request has unset limits")
	}
	if tables.req.Style.Size != Script || !tables.req.Style.Cramped {
		t.Fatalf("request style = %+v, want script/cramped", tables.req.Style)
	}
	if len(tables.req.Rows) != 1 || len(tables.req.Rows[0].Cells) != 1 {
		t.Fatalf("rows = %+v", tables.req.Rows)
	}
	if tables.req.Rows[0].Cells[0].Size != Script {
		t.Fatalf("cell size = %v, want script", tables.req.Rows[0].Cells[0].Size)
	}
	cell := tables.req.Rows[0].Cells[0]
	if cell.ownHit < 0 || cell.hits[cell.ownHit].kind != display.HitTableCell {
		t.Fatalf("cell own hit = %+v, want table-cell kind", cell.hits)
	}
	if len(tables.req.Horizontal) != 1 || !tables.req.Horizontal[0].Dashed || tables.req.Horizontal[0].Thickness <= 0 {
		t.Fatalf("horizontal lines = %+v", tables.req.Horizontal)
	}
	if len(tables.req.Vertical) != 1 || tables.req.Vertical[0].Position != 1 || tables.req.Vertical[0].Count != 2 {
		t.Fatalf("vertical lines = %+v", tables.req.Vertical)
	}
}

type cancellingTableLayouter struct {
	cancel context.CancelFunc
	base   fakeTableLayouter
}

func (c *cancellingTableLayouter) LayoutTable(req TableRequest) (Fragment, bool) {
	c.cancel()
	return c.base.LayoutTable(req)
}

func TestTableExtensionObservesCancellation(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	tables := &cancellingTableLayouter{cancel: cancel}
	renderer, err := New(Fonts{
		Glyphs:  &fakeGlyphSource{glyphs: map[string]Glyph{}},
		Metrics: fakeStyleFont{},
	}, Options{Tables: tables})
	if err != nil {
		t.Fatal(err)
	}
	table := ir.NewTable(source.NewRange(0, 1), ir.TableRow{
		Cells: []ir.Node{glyph("x", ir.ClassNormal)},
	})
	if _, err := renderer.LayoutContext(ctx, table, ir.Inline, testStyle(16)); !errors.Is(err, context.Canceled) {
		t.Fatalf("LayoutContext() error = %v, want context.Canceled", err)
	}
}

type cyclicTableLayouter struct{}

func (cyclicTableLayouter) LayoutTable(req TableRequest) (Fragment, bool) {
	fragment, err := NewFragment(
		Metrics{
			Width:   geometry.Em(geometry.EmScale),
			Ascent:  geometry.Em(geometry.EmScale / 2),
			Descent: geometry.Em(geometry.EmScale / 2),
		},
		ir.ClassInner,
		req.Style.Size,
		geometry.Em(geometry.EmScale),
	)
	if err != nil {
		return Fragment{}, false
	}
	if err := fragment.AddGroup(0, 0, fragment); err != nil {
		return Fragment{}, false
	}
	return *fragment, true
}

func TestTableExtensionCycleIsRejected(t *testing.T) {
	t.Parallel()

	renderer, err := New(Fonts{
		Glyphs:  &fakeGlyphSource{glyphs: map[string]Glyph{}},
		Metrics: fakeStyleFont{},
	}, Options{Tables: cyclicTableLayouter{}})
	if err != nil {
		t.Fatal(err)
	}
	table := ir.NewTable(source.NewRange(0, 1), ir.TableRow{
		Cells: []ir.Node{glyph("x", ir.ClassNormal)},
	})
	if _, err := renderer.Layout(table, ir.Inline, testStyle(16)); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("Layout() error = %v, want ErrInvalidInput", err)
	}
}

type repeatedReferenceTableLayouter struct{}

func (repeatedReferenceTableLayouter) LayoutTable(req TableRequest) (Fragment, bool) {
	child, err := NewFragment(
		Metrics{
			Width:   geometry.Em(geometry.EmScale),
			Ascent:  geometry.Em(geometry.EmScale / 2),
			Descent: geometry.Em(geometry.EmScale / 2),
		},
		ir.ClassInner,
		req.Style.Size,
		geometry.Em(geometry.EmScale),
	)
	if err != nil {
		return Fragment{}, false
	}
	root, err := NewFragment(
		Metrics{
			Width:   geometry.Em(geometry.EmScale),
			Ascent:  geometry.Em(geometry.EmScale / 2),
			Descent: geometry.Em(geometry.EmScale / 2),
		},
		ir.ClassInner,
		req.Style.Size,
		geometry.Em(geometry.EmScale),
	)
	if err != nil {
		return Fragment{}, false
	}
	for range 10 {
		if err := root.AddGroup(0, 0, child); err != nil {
			return Fragment{}, false
		}
	}
	return *root, true
}

func TestTableExtensionRepeatedReferencesUseVisitBudget(t *testing.T) {
	t.Parallel()

	renderer, err := New(Fonts{
		Glyphs:  &fakeGlyphSource{glyphs: map[string]Glyph{}},
		Metrics: fakeStyleFont{},
	}, Options{
		Tables: repeatedReferenceTableLayouter{},
		Limits: source.Limits{MaxNodes: 4, MaxNesting: 10, MaxTableCells: 10},
	})
	if err != nil {
		t.Fatal(err)
	}
	table := ir.NewTable(source.NewRange(0, 1), ir.TableRow{
		Cells: []ir.Node{glyph("x", ir.ClassNormal)},
	})
	if _, err := renderer.Layout(table, ir.Inline, testStyle(16)); !errors.Is(err, ErrLimit) {
		t.Fatalf("Layout() error = %v, want ErrLimit", err)
	}
}
