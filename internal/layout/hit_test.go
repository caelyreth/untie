package layout

import (
	"testing"

	"github.com/caelyreth/untie/display"
	"github.com/caelyreth/untie/geometry"
	"github.com/caelyreth/untie/identity"
	"github.com/caelyreth/untie/ir"
	"github.com/caelyreth/untie/source"
)

func TestSourceHitsFollowPlacedAtoms(t *testing.T) {
	t.Parallel()

	renderer, _, _ := newTestRenderer(t)
	x := glyph("x", ir.ClassNormal)
	x.Span = source.NewRange(0, 1)
	x.Origin = x.Span
	y := glyph("y", ir.ClassNormal)
	y.Span = source.NewRange(2, 3)
	y.Origin = y.Span
	root := ir.NewRow(source.NewRange(0, 3), x, y)

	box, err := renderer.Layout(root, ir.Inline, testStyle(16))
	if err != nil {
		t.Fatalf("Layout() error = %v", err)
	}
	if len(box.Hits) != 3 {
		t.Fatalf("hits = %#v, want row and two atom hits", box.Hits)
	}

	var atoms []display.Hit
	for _, hit := range box.Hits {
		if hit.Kind == display.HitAtom {
			atoms = append(atoms, hit)
		}
	}
	if len(atoms) != 2 {
		t.Fatalf("atom hits = %#v, want 2", atoms)
	}
	if atoms[0].Source != source.NewRange(0, 1) || atoms[1].Source != source.NewRange(2, 3) {
		t.Fatalf("atom hit sources = %v, %v", atoms[0].Source, atoms[1].Source)
	}
	if atoms[0].Area.X != 0 || !closeHost(atoms[0].Area.Width, 8) {
		t.Fatalf("first atom area = %#v, want x=0 width=8", atoms[0].Area)
	}
	if !closeHost(atoms[1].Area.X, 8) || !closeHost(atoms[1].Area.Width, 8) {
		t.Fatalf("second atom area = %#v, want x=8 width=8", atoms[1].Area)
	}
	if atoms[0].Depth <= box.Hits[0].Depth {
		t.Fatalf("atom depth %d is not nested below row depth %d", atoms[0].Depth, box.Hits[0].Depth)
	}

	hit, ok := box.At(geometry.Point{X: 4, Y: 0})
	if !ok || hit.Source != source.NewRange(0, 1) || hit.Kind != display.HitAtom {
		t.Fatalf("At() = (%#v, %v), want first atom", hit, ok)
	}
	areas := box.Covering(source.NewRange(0, 3))
	if len(areas) != 1 || !closeHost(areas[0].Width, 16) {
		t.Fatalf("Covering() = %#v, want one merged atom row", areas)
	}
}

func TestFractionHitsIdentifyParts(t *testing.T) {
	t.Parallel()

	renderer, _, _ := newTestRenderer(t)
	numerator := glyph("1", ir.ClassNormal)
	numerator.Span = source.NewRange(1, 2)
	numerator.Origin = numerator.Span
	denominator := glyph("2", ir.ClassNormal)
	denominator.Span = source.NewRange(3, 4)
	denominator.Origin = denominator.Span
	fraction := ir.NewFraction(source.NewRange(0, 5), numerator, denominator)

	box, err := renderer.Layout(fraction, ir.Inline, testStyle(20))
	if err != nil {
		t.Fatalf("Layout() error = %v", err)
	}
	var foundNumerator, foundDenominator bool
	for _, hit := range box.Hits {
		switch hit.Kind {
		case display.HitFractionNumerator:
			foundNumerator = true
			if hit.Source != numerator.Span {
				t.Fatalf("numerator source = %v, want %v", hit.Source, numerator.Span)
			}
		case display.HitFractionDenominator:
			foundDenominator = true
			if hit.Source != denominator.Span {
				t.Fatalf("denominator source = %v, want %v", hit.Source, denominator.Span)
			}
		}
	}
	if !foundNumerator || !foundDenominator {
		t.Fatalf("fraction hits = %#v, want numerator and denominator", box.Hits)
	}
}

func TestNodeIDsPropagateToSemanticHitsAndItems(t *testing.T) {
	t.Parallel()

	renderer, _, _ := newTestRenderer(t)
	numerator := glyph("1", ir.ClassNormal)
	numerator.Span = source.NewRange(1, 2)
	numerator.Origin = numerator.Span
	numerator.NodeID = identity.NodeID(11)
	denominator := glyph("2", ir.ClassNormal)
	denominator.Span = source.NewRange(3, 4)
	denominator.Origin = denominator.Span
	denominator.NodeID = identity.NodeID(12)
	fraction := ir.NewFraction(source.NewRange(0, 5), numerator, denominator)
	fraction.NodeID = identity.NodeID(13)
	style := ir.NewStyle(source.NewRange(0, 5), fraction)
	style.NodeID = identity.NodeID(14)
	row := ir.NewRow(source.NewRange(0, 5), style)
	row.NodeID = identity.NodeID(15)

	box, err := renderer.Layout(row, ir.Inline, testStyle(20))
	if err != nil {
		t.Fatalf("Layout() error = %v", err)
	}

	hitIDs := make(map[identity.NodeID]display.HitKind)
	for _, hit := range box.Hits {
		hitIDs[hit.NodeID] = hit.Kind
	}
	for id, want := range map[identity.NodeID]display.HitKind{
		11: display.HitFractionNumerator,
		12: display.HitFractionDenominator,
		13: display.HitOther,
		14: display.HitGroup,
		15: display.HitRow,
	} {
		if got := hitIDs[id]; got != want {
			t.Fatalf("hit NodeID %d kind = %d, want %d; hits=%#v", id, got, want, box.Hits)
		}
	}

	itemIDs := make(map[identity.NodeID]int)
	for _, item := range box.Items {
		itemIDs[item.NodeID]++
	}
	for _, id := range []identity.NodeID{11, 12, 13} {
		if itemIDs[id] == 0 {
			t.Fatalf("items for NodeID %d missing: %#v", id, box.Items)
		}
	}
	if itemIDs[14] != 0 {
		t.Fatalf("wrapper NodeID replaced descendant item identity: %#v", box.Items)
	}
}

func TestSourceHitUsesOriginAndSkipsDetachedNodes(t *testing.T) {
	t.Parallel()

	renderer, _, _ := newTestRenderer(t)
	node := glyph("x", ir.ClassNormal)
	node.Span = source.NewRange(0, 1)
	node.Origin = source.NewRange(4, 5)
	box, err := renderer.Layout(node, ir.Inline, testStyle(16))
	if err != nil {
		t.Fatalf("Layout() error = %v", err)
	}
	if len(box.Hits) != 1 || box.Hits[0].Source != node.Origin {
		t.Fatalf("origin hits = %#v, want %v", box.Hits, node.Origin)
	}

	node.Origin = source.Detached()
	box, err = renderer.Layout(node, ir.Inline, testStyle(16))
	if err != nil {
		t.Fatalf("Layout(span fallback) error = %v", err)
	}
	if len(box.Hits) != 1 || box.Hits[0].Source != node.Span {
		t.Fatalf("span fallback hits = %#v, want %v", box.Hits, node.Span)
	}

	node.Span = source.Detached()
	box, err = renderer.Layout(node, ir.Inline, testStyle(16))
	if err != nil {
		t.Fatalf("Layout(detached) error = %v", err)
	}
	if len(box.Hits) != 0 {
		t.Fatalf("detached hits = %#v, want none", box.Hits)
	}
}

func TestFencedAndScriptHitsIdentifySemanticRoles(t *testing.T) {
	t.Parallel()

	renderer, _, _ := newTestRenderer(t)
	open := glyph("(", ir.ClassOpening)
	open.Span = source.NewRange(0, 1)
	open.Origin = open.Span
	body := glyph("x", ir.ClassNormal)
	body.Span = source.NewRange(1, 2)
	body.Origin = body.Span
	close := glyph(")", ir.ClassClosing)
	close.Span = source.NewRange(2, 3)
	close.Origin = close.Span
	fenced := ir.NewFenced(source.NewRange(0, 3), body)
	fenced.Open = open
	fenced.Close = close
	fenced.Balanced = true

	box, err := renderer.Layout(fenced, ir.Inline, testStyle(16))
	if err != nil {
		t.Fatalf("Layout(fenced) error = %v", err)
	}
	var openHit, closeHit bool
	for _, hit := range box.Hits {
		if hit.Kind != display.HitFence {
			continue
		}
		openHit = openHit || hit.Source == open.Span
		closeHit = closeHit || hit.Source == close.Span
	}
	if !openHit || !closeHit {
		t.Fatalf("fence hits = %#v, want both delimiters", box.Hits)
	}

	base := glyph("x", ir.ClassNormal)
	base.Span = source.NewRange(0, 1)
	base.Origin = base.Span
	script := glyph("2", ir.ClassNormal)
	script.Span = source.NewRange(2, 3)
	script.Origin = script.Span
	scripts := ir.NewScripts(source.NewRange(0, 3), base)
	scripts.TopRight = script
	box, err = renderer.Layout(scripts, ir.Inline, testStyle(16))
	if err != nil {
		t.Fatalf("Layout(scripts) error = %v", err)
	}
	var scriptHit bool
	for _, hit := range box.Hits {
		if hit.Kind == display.HitScript && hit.Source == script.Span {
			scriptHit = true
			break
		}
	}
	if !scriptHit {
		t.Fatalf("script hits = %#v, want script source %v", box.Hits, script.Span)
	}
}

func TestTableHitsIdentifyCells(t *testing.T) {
	t.Parallel()

	renderer, _, _ := newTestRenderer(t)
	cell := ir.NewRow(source.NewRange(0, 1), glyph("x", ir.ClassNormal))
	table := ir.NewTable(source.NewRange(0, 1), ir.TableRow{Cells: []ir.Node{cell}})
	box, err := renderer.Layout(table, ir.Inline, testStyle(16))
	if err != nil {
		t.Fatalf("Layout(table) error = %v", err)
	}
	for _, hit := range box.Hits {
		if hit.Kind == display.HitTableCell && hit.Source == cell.Span {
			return
		}
	}
	t.Fatalf("table hits = %#v, want cell source %v", box.Hits, cell.Span)
}
