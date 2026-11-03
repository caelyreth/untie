package layout

import (
	"context"
	"testing"

	"github.com/caelyreth/untie/display"
	"github.com/caelyreth/untie/identity"
	"github.com/caelyreth/untie/ir"
	"github.com/caelyreth/untie/source"
)

func TestNodeIDsSurviveDisplayLinesAndSourceEdits(t *testing.T) {
	t.Parallel()

	renderer, _, _ := newTestRenderer(t)
	a := glyph("a", ir.ClassNormal)
	a.Span = source.NewRange(0, 1)
	a.Origin = a.Span
	a.NodeID = identity.NodeID(101)
	b := glyph("b", ir.ClassNormal)
	b.Span = source.NewRange(2, 3)
	b.Origin = b.Span
	b.NodeID = identity.NodeID(102)
	root := ir.NewRow(source.NewRange(0, 3), a,
		ir.NewBreak(source.NewRange(1, 2), ir.BreakExplicit), b)
	root.NodeID = identity.NodeID(100)
	style := testStyle(16)
	style.MaxWidth = 9

	box, err := renderer.Layout(root, ir.Display, style)
	if err != nil {
		t.Fatalf("Layout() error = %v", err)
	}
	if len(box.Lines) < 2 {
		t.Fatalf("lines = %d, want wrapping", len(box.Lines))
	}
	for _, line := range box.Lines {
		foundRow := false
		for i := line.Hits.Start; i < line.Hits.End(); i++ {
			hit := box.Hits[i]
			if hit.Kind == display.HitRow {
				foundRow = true
				if hit.NodeID != root.NodeID {
					t.Fatalf("line row hit NodeID = %d, want %d", hit.NodeID, root.NodeID)
				}
			}
		}
		if !foundRow {
			t.Fatalf("line %#v has no row hit", line)
		}
	}

	a.Span = source.NewRange(20, 21)
	a.Origin = a.Span
	edited, err := renderer.Layout(root, ir.Inline, testStyle(16))
	if err != nil {
		t.Fatalf("Layout(edited) error = %v", err)
	}
	foundEdited := false
	for _, hit := range edited.Hits {
		if hit.NodeID == a.NodeID {
			foundEdited = true
			if hit.Source != a.Span {
				t.Fatalf("edited hit source = %v, want %v", hit.Source, a.Span)
			}
		}
	}
	if !foundEdited {
		t.Fatalf("source edit lost NodeID %d: %#v", a.NodeID, edited.Hits)
	}
}

func TestNodeIDsReachEquationTagGeometry(t *testing.T) {
	t.Parallel()

	renderer, _, _ := newTestRenderer(t)
	root := glyph("x", ir.ClassNormal)
	root.NodeID = identity.NodeID(201)
	tag := glyph("1", ir.ClassNormal)
	tag.Span = source.NewRange(1, 2)
	tag.Origin = tag.Span
	tag.NodeID = identity.NodeID(202)
	formula := taggedTestFormula(root, tag)

	box, err := renderer.LayoutFormulaContext(
		context.Background(),
		formula,
		ir.Display,
		testStyle(16),
	)
	if err != nil {
		t.Fatalf("LayoutFormulaContext() error = %v", err)
	}
	if box.Tag == nil {
		t.Fatal("tag metadata missing")
	}
	found := false
	for i := box.Tag.Items.Start; i < box.Tag.Items.End(); i++ {
		if box.Items[i].NodeID == tag.NodeID {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("tag items lost NodeID %d: %#v", tag.NodeID, box.Items)
	}
}
