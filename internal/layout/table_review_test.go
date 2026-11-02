package layout

import (
	"testing"

	"github.com/caelyreth/untie/ir"
	"github.com/caelyreth/untie/source"
)

func TestTableRequestPreservesStyleLinesTagsAndCellStyle(t *testing.T) {
	t.Parallel()

	glyphs := &fakeGlyphSource{glyphs: map[string]Glyph{}}
	tables := &fakeTableLayouter{}
	renderer, err := New(Fonts{Glyphs: glyphs, Metrics: fakeStyleFont{}}, Options{Tables: tables})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	table := ir.NewTable(source.NewRange(0, 3), ir.TableRow{
		Cells: []ir.Node{glyph("x", ir.ClassNormal)},
		Tag:   glyph("1", ir.ClassNormal),
	})
	table.Horizontal = []ir.TableLine{{Position: 0, Count: 1, Dashed: true, Thickness: ir.EmLength(0.02)}}
	table.Vertical = []ir.TableLine{{Position: 1, Count: 2, Thickness: ir.EmLength(0.03)}}
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
	if tables.req.Style.Size != Script || !tables.req.Style.Cramped {
		t.Fatalf("request style = %+v, want script/cramped", tables.req.Style)
	}
	if len(tables.req.Rows) != 1 || len(tables.req.Rows[0].Cells) != 1 {
		t.Fatalf("rows = %+v", tables.req.Rows)
	}
	if tables.req.Rows[0].Cells[0].Size != Script {
		t.Fatalf("cell size = %v, want script", tables.req.Rows[0].Cells[0].Size)
	}
	if tables.req.Rows[0].Tag == nil || tables.req.Rows[0].Tag.Size != Script {
		t.Fatalf("tag = %+v, want measured tag in inherited style", tables.req.Rows[0].Tag)
	}
	if len(tables.req.Horizontal) != 1 || !tables.req.Horizontal[0].Dashed || tables.req.Horizontal[0].Thickness <= 0 {
		t.Fatalf("horizontal lines = %+v", tables.req.Horizontal)
	}
	if len(tables.req.Vertical) != 1 || tables.req.Vertical[0].Position != 1 || tables.req.Vertical[0].Count != 2 {
		t.Fatalf("vertical lines = %+v", tables.req.Vertical)
	}
}
