package layout

import (
	"context"
	"errors"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/caelyreth/untie/display"
	"github.com/caelyreth/untie/geometry"
	"github.com/caelyreth/untie/ir"
	"github.com/caelyreth/untie/source"
)

func TestTopLevelRelationAndBinaryBreaks(t *testing.T) {
	t.Parallel()

	renderer, _, _ := newTestRenderer(t)
	tests := []struct {
		name   string
		root   ir.Node
		kind   display.BreakKind
		nextAt int
	}{
		{
			name:   "relation",
			root:   ir.NewRow(source.NewRange(0, 3), glyph("x", ir.ClassNormal), glyph("=", ir.ClassRelation), glyph("y", ir.ClassNormal)),
			kind:   display.BreakRelation,
			nextAt: 2,
		},
		{
			name:   "binary",
			root:   ir.NewRow(source.NewRange(0, 3), glyph("x", ir.ClassNormal), glyph("+", ir.ClassBinary), glyph("y", ir.ClassNormal)),
			kind:   display.BreakBinary,
			nextAt: 2,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			box, err := renderer.Layout(test.root, ir.Inline, testStyle(16))
			if err != nil {
				t.Fatalf("Layout() error = %v", err)
			}
			if len(box.Breaks) != 1 {
				t.Fatalf("Breaks = %+v, want one", box.Breaks)
			}
			got := box.Breaks[0]
			if got.Kind != test.kind || got.Penalty != 0 {
				t.Fatalf("break = %+v, want kind %v and penalty 0", got, test.kind)
			}
			// The post-operator automatic space belongs to the preceding piece.
			if want := box.Items[test.nextAt].At.X; !closeHost(got.X, want) {
				t.Fatalf("break X = %v, want post-operator glue boundary %v", got.X, want)
			}
		})
	}
}

func TestBreakMetadataIsInlineOnly(t *testing.T) {
	t.Parallel()

	renderer, _, _ := newTestRenderer(t)
	root := ir.NewRow(source.NewRange(0, 3),
		glyph("x", ir.ClassNormal),
		glyph("=", ir.ClassRelation),
		glyph("y", ir.ClassNormal),
	)
	box, err := renderer.Layout(root, ir.Display, testStyle(16))
	if err != nil {
		t.Fatalf("Layout() error = %v", err)
	}
	if len(box.Breaks) != 0 {
		t.Fatalf("display Breaks = %+v, want none", box.Breaks)
	}
}

func TestTopLevelBreaksAreSourceOrdered(t *testing.T) {
	t.Parallel()

	renderer, _, _ := newTestRenderer(t)
	root := ir.NewRow(source.NewRange(0, 5),
		glyph("a", ir.ClassNormal),
		glyph("=", ir.ClassRelation),
		glyph("b", ir.ClassNormal),
		glyph("+", ir.ClassBinary),
		glyph("c", ir.ClassNormal),
	)
	box, err := renderer.Layout(root, ir.Inline, testStyle(16))
	if err != nil {
		t.Fatalf("Layout() error = %v", err)
	}
	wantKinds := []display.BreakKind{display.BreakRelation, display.BreakBinary}
	if len(box.Breaks) != len(wantKinds) {
		t.Fatalf("Breaks = %+v, want %d", box.Breaks, len(wantKinds))
	}
	for i, want := range wantKinds {
		if box.Breaks[i].Kind != want {
			t.Fatalf("Breaks[%d].Kind = %v, want %v", i, box.Breaks[i].Kind, want)
		}
	}
	if want := box.Items[2].At.X; !closeHost(box.Breaks[0].X, want) {
		t.Fatalf("relation break X = %v, want %v", box.Breaks[0].X, want)
	}
	if want := box.Items[4].At.X; !closeHost(box.Breaks[1].X, want) {
		t.Fatalf("binary break X = %v, want %v", box.Breaks[1].X, want)
	}
	if box.Breaks[0].X >= box.Breaks[1].X {
		t.Fatalf("breaks are not ordered: %+v", box.Breaks)
	}
}

func TestExplicitBreakIncludesFollowingSpaceAndWinsTies(t *testing.T) {
	t.Parallel()

	renderer, _, _ := newTestRenderer(t)
	before := ir.NewSpace(source.NewRange(1, 2), ir.SpaceExplicit, emLength(t, 0.25))
	after := ir.NewSpace(source.NewRange(3, 4), ir.SpaceExplicit, emLength(t, 0.75))
	root := ir.NewRow(source.NewRange(0, 4),
		glyph("x", ir.ClassNormal),
		before,
		ir.NewBreak(source.NewRange(2, 3), ir.BreakExplicit, 123),
		after,
		glyph("y", ir.ClassNormal),
	)
	box, err := renderer.Layout(root, ir.Inline, testStyle(16))
	if err != nil {
		t.Fatalf("Layout() error = %v", err)
	}
	if len(box.Breaks) != 1 {
		t.Fatalf("Breaks = %+v, want one", box.Breaks)
	}
	if got := box.Breaks[0]; got.Kind != display.BreakExplicit || got.Penalty != 123 {
		t.Fatalf("break = %+v, want explicit with source penalty", got)
	}
	if want := box.Items[1].At.X; !closeHost(box.Breaks[0].X, want) {
		t.Fatalf("explicit break X = %v, want %v after following space", box.Breaks[0].X, want)
	}

	tied := ir.NewRow(source.NewRange(0, 4),
		glyph("x", ir.ClassNormal),
		glyph("=", ir.ClassRelation),
		ir.NewBreak(source.NewRange(2, 3), ir.BreakExplicit, 123),
		glyph("y", ir.ClassNormal),
	)
	box, err = renderer.Layout(tied, ir.Inline, testStyle(16))
	if err != nil {
		t.Fatalf("Layout(tied) error = %v", err)
	}
	if len(box.Breaks) != 1 || box.Breaks[0].Kind != display.BreakExplicit || box.Breaks[0].Penalty != 123 {
		t.Fatalf("tied Breaks = %+v, want one explicit opportunity with source penalty", box.Breaks)
	}

	semantic := ir.NewRow(source.NewRange(0, 3),
		glyph("x", ir.ClassNormal),
		ir.NewBreak(source.NewRange(1, 2), ir.BreakRelation, 0),
		glyph("y", ir.ClassNormal),
	)
	box, err = renderer.Layout(semantic, ir.Inline, testStyle(16))
	if err != nil {
		t.Fatalf("Layout(semantic) error = %v", err)
	}
	if len(box.Breaks) != 1 || box.Breaks[0].Kind != display.BreakRelation {
		t.Fatalf("semantic Breaks = %+v, want one relation opportunity", box.Breaks)
	}
}

func TestBinaryDemotionSuppressesBreak(t *testing.T) {
	t.Parallel()

	renderer, _, _ := newTestRenderer(t)
	tests := []struct {
		name string
		root ir.Node
		want []display.BreakKind
	}{
		{
			name: "trailing binary",
			root: ir.NewRow(source.NewRange(0, 2), glyph("x", ir.ClassNormal), glyph("+", ir.ClassBinary)),
		},
		{
			name: "binary before relation",
			root: ir.NewRow(source.NewRange(0, 4), glyph("x", ir.ClassNormal), glyph("+", ir.ClassBinary), glyph("=", ir.ClassRelation), glyph("y", ir.ClassNormal)),
			want: []display.BreakKind{display.BreakRelation},
		},
		{
			name: "binary before closing",
			root: ir.NewRow(source.NewRange(0, 3), glyph("(", ir.ClassOpening), glyph("+", ir.ClassBinary), glyph(")", ir.ClassClosing)),
		},
		{
			name: "leading binary is unary",
			root: ir.NewRow(source.NewRange(0, 2), glyph("+", ir.ClassBinary), glyph("x", ir.ClassNormal)),
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			box, err := renderer.Layout(test.root, ir.Inline, testStyle(16))
			if err != nil {
				t.Fatalf("Layout() error = %v", err)
			}
			got := make([]display.BreakKind, len(box.Breaks))
			for i := range box.Breaks {
				got[i] = box.Breaks[i].Kind
			}
			if !slices.Equal(got, test.want) {
				t.Fatalf("break kinds = %v, want %v", got, test.want)
			}
		})
	}
}

func TestTypstBreakEligibilityGuards(t *testing.T) {
	t.Parallel()

	renderer, _, _ := newTestRenderer(t)
	tests := []struct {
		name string
		root ir.Node
	}{
		{
			name: "relation before closing",
			root: ir.NewRow(source.NewRange(0, 3), glyph("x", ir.ClassNormal), glyph("=", ir.ClassRelation), glyph(")", ir.ClassClosing)),
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			box, err := renderer.Layout(test.root, ir.Inline, testStyle(16))
			if err != nil {
				t.Fatalf("Layout() error = %v", err)
			}
			if len(box.Breaks) != 0 {
				t.Fatalf("Breaks = %+v, want none", box.Breaks)
			}
		})
	}

	// A relation after a relation still offers a break after the final relation.
	root := ir.NewRow(source.NewRange(0, 4),
		glyph("x", ir.ClassNormal),
		glyph("=", ir.ClassRelation),
		glyph("=", ir.ClassRelation),
		glyph("y", ir.ClassNormal),
	)
	box, err := renderer.Layout(root, ir.Inline, testStyle(16))
	if err != nil {
		t.Fatalf("Layout(relation run) error = %v", err)
	}
	if len(box.Breaks) != 1 || box.Breaks[0].Kind != display.BreakRelation {
		t.Fatalf("relation-run Breaks = %+v, want final relation", box.Breaks)
	}
}

func TestNestedConstructsDoNotLeakBreaks(t *testing.T) {
	t.Parallel()

	renderer, _, _ := newTestRenderer(t)
	rowWithRelation := func() ir.Node {
		return ir.NewRow(source.NewRange(0, 3), glyph("x", ir.ClassNormal), glyph("=", ir.ClassRelation), glyph("y", ir.ClassNormal))
	}
	rowWithExplicit := func() ir.Node {
		return ir.NewRow(source.NewRange(0, 3), glyph("x", ir.ClassNormal), ir.NewBreak(source.NewRange(1, 2), ir.BreakExplicit, 0), glyph("y", ir.ClassNormal))
	}
	tests := []struct {
		name string
		root ir.Node
		mode ir.Mode
	}{
		{
			name: "nested row",
			root: ir.NewRow(source.NewRange(0, 5), glyph("a", ir.ClassNormal), rowWithRelation(), glyph("b", ir.ClassNormal)),
			mode: ir.Inline,
		},
		{
			name: "fenced group",
			root: ir.NewFenced(source.NewRange(0, 5), glyph("x", ir.ClassNormal), glyph("=", ir.ClassRelation), glyph("y", ir.ClassNormal)),
			mode: ir.Inline,
		},
		{
			name: "fraction",
			root: ir.NewFraction(source.NewRange(0, 5), rowWithRelation(), glyph("z", ir.ClassNormal)),
			mode: ir.Inline,
		},
		{
			name: "scripts",
			root: func() ir.Node {
				scripts := ir.NewScripts(source.NewRange(0, 5), rowWithRelation())
				scripts.TopRight = glyph("2", ir.ClassNormal)
				return scripts
			}(),
			mode: ir.Inline,
		},
		{
			name: "table cell relation",
			root: ir.NewTable(source.NewRange(0, 5), ir.TableRow{Cells: []ir.Node{rowWithRelation()}}),
			mode: ir.Inline,
		},
		{
			name: "table cell explicit",
			root: ir.NewTable(source.NewRange(0, 5), ir.TableRow{Cells: []ir.Node{rowWithExplicit()}}),
			mode: ir.Inline,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			box, err := renderer.Layout(test.root, test.mode, testStyle(16))
			if err != nil {
				t.Fatalf("Layout() error = %v", err)
			}
			if len(box.Breaks) != 0 {
				t.Fatalf("nested Breaks leaked: %+v", box.Breaks)
			}
		})
	}
}

func TestBreakMetadataCanonicalCloneAndDeterminism(t *testing.T) {
	t.Parallel()

	renderer, _, _ := newTestRenderer(t)
	root := ir.NewRow(source.NewRange(0, 5),
		glyph("a", ir.ClassNormal),
		glyph("=", ir.ClassRelation),
		glyph("b", ir.ClassNormal),
		glyph("+", ir.ClassBinary),
		glyph("c", ir.ClassNormal),
	)
	box, err := renderer.Layout(root, ir.Inline, testStyle(16))
	if err != nil {
		t.Fatalf("Layout() error = %v", err)
	}
	if err := box.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
	canonical, err := box.MarshalCanonical()
	if err != nil {
		t.Fatalf("MarshalCanonical() error = %v", err)
	}
	if !strings.Contains(string(canonical), `"kind":"relation"`) ||
		!strings.Contains(string(canonical), `"kind":"binary"`) {
		t.Fatalf("canonical output omitted break kinds: %s", canonical)
	}

	clone := box.Clone()
	clone.Breaks[0].X++
	if clone.Breaks[0].X == box.Breaks[0].X {
		t.Fatal("Clone shared break storage")
	}
	for i := 0; i < 16; i++ {
		next, err := renderer.Layout(root, ir.Inline, testStyle(16))
		if err != nil {
			t.Fatalf("Layout(%d) error = %v", i, err)
		}
		if !reflect.DeepEqual(next.Breaks, box.Breaks) {
			t.Fatalf("Layout(%d) Breaks = %+v, want %+v", i, next.Breaks, box.Breaks)
		}
	}
}

func TestBreakOutputLimits(t *testing.T) {
	t.Parallel()

	limited := layoutState{limits: source.Limits{MaxOutputItems: 1}.WithDefaults()}
	_, err := limited.displayBreaks(context.Background(), []fragmentBreak{
		{x: 0, kind: display.BreakRelation},
		{x: 1, kind: display.BreakBinary},
	})
	if !errors.Is(err, ErrLimit) {
		t.Fatalf("displayBreaks() error = %v, want limit", err)
	}

	dimensionLimited := layoutState{
		hostSize: 16 * hostSubunits,
		limits:   source.Limits{MaxDimension: 1}.WithDefaults(),
	}
	_, err = dimensionLimited.displayBreaks(context.Background(), []fragmentBreak{{
		x:    geometry.Em(geometry.EmScale),
		kind: display.BreakRelation,
	}})
	if !errors.Is(err, ErrLimit) {
		t.Fatalf("displayBreaks() dimension error = %v, want limit", err)
	}
}
