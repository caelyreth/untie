package layout

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/caelyreth/untie/display"
	"github.com/caelyreth/untie/geometry"
	"github.com/caelyreth/untie/ir"
	"github.com/caelyreth/untie/source"
)

func TestDisplayLineOffsetsAndAggregateTranslation(t *testing.T) {
	t.Parallel()

	const maxWidth = float32(10)
	widths := []float32{10, 4, 5, 11}
	tests := []struct {
		align display.Align
		want  []float32
	}{
		{align: display.AlignStart, want: []float32{0, 0, 0, 0}},
		{align: display.AlignCenter, want: []float32{0, 3, 2.5, 0}},
		{align: display.AlignEnd, want: []float32{0, 6, 5, 0}},
	}
	for _, tc := range tests {
		t.Run(alignName(tc.align), func(t *testing.T) {
			t.Parallel()
			state := alignmentLayoutState(tc.align, maxWidth)
			lines := make([]display.LineBox, len(widths))
			for i, width := range widths {
				lines[i] = alignmentLine(width, maxWidth)
			}
			box, err := state.assembleDisplayLines(&display.Box{}, lines)
			if err != nil {
				t.Fatalf("assembleDisplayLines() error = %v", err)
			}
			if box.Width != maxWidth {
				t.Fatalf("Width = %v, want alignment frame %v", box.Width, maxWidth)
			}
			if len(box.Items) != len(lines) || len(box.Hits) != len(lines) {
				t.Fatalf("aggregate = %d items/%d hits, want %d", len(box.Items), len(box.Hits), len(lines))
			}
			for i, line := range box.Lines {
				if line.OffsetX != tc.want[i] {
					t.Fatalf("line %d OffsetX = %v, want %v", i, line.OffsetX, tc.want[i])
				}
				if line.Box.Items[0].Bound.X != 0 || line.Box.Hits[0].Area.X != 0 {
					t.Fatalf("line %d local coordinates were mutated: item=%v hit=%v",
						i, line.Box.Items[0].Bound.X, line.Box.Hits[0].Area.X)
				}
				if got := box.Items[i].Bound.X; got != tc.want[i] {
					t.Fatalf("line %d parent item x = %v, want %v", i, got, tc.want[i])
				}
				if got := box.Hits[i].Area.X; got != tc.want[i] {
					t.Fatalf("line %d parent hit x = %v, want %v", i, got, tc.want[i])
				}
			}
			if !closeHost(box.Overflow, 1) {
				t.Fatalf("Overflow = %v, want overfull-line extent 1", box.Overflow)
			}
			canonical, err := box.MarshalCanonical()
			if err != nil {
				t.Fatalf("MarshalCanonical() error = %v", err)
			}
			if !strings.Contains(string(canonical), `"offsetX":`) {
				t.Fatalf("canonical output omitted line OffsetX: %s", canonical)
			}
			clone := box.Clone()
			if clone.Lines[1].OffsetX != box.Lines[1].OffsetX {
				t.Fatalf("clone OffsetX = %v, want %v", clone.Lines[1].OffsetX, box.Lines[1].OffsetX)
			}
		})
	}
}

func TestDisplayAlignmentRealWrappedLines(t *testing.T) {
	t.Parallel()

	renderer, _, _ := newTestRenderer(t)
	root := ir.NewRow(source.NewRange(0, 7),
		glyph("a", ir.ClassNormal),
		glyph("+", ir.ClassBinary),
		glyph("b", ir.ClassNormal),
		glyph("+", ir.ClassBinary),
		glyph("c", ir.ClassNormal),
		glyph("+", ir.ClassBinary),
		glyph("ddddddddddd", ir.ClassNormal),
	)
	inline, err := renderer.Layout(root, ir.Inline, testStyle(16))
	if err != nil {
		t.Fatal(err)
	}
	style := testStyle(16)
	style.MaxWidth = inline.Breaks[0].X
	style.Align = display.AlignStart
	start, err := renderer.Layout(root, ir.Display, style)
	if err != nil {
		t.Fatal(err)
	}
	if len(start.Lines) < 2 {
		t.Fatalf("Lines = %d, want a wrapped fixture", len(start.Lines))
	}

	for _, align := range []display.Align{display.AlignCenter, display.AlignEnd} {
		t.Run(alignName(align), func(t *testing.T) {
			alignedStyle := style
			alignedStyle.Align = align
			box, err := renderer.Layout(root, ir.Display, alignedStyle)
			if err != nil {
				t.Fatalf("Layout() error = %v", err)
			}
			if !closeHost(box.Width, alignedStyle.MaxWidth) {
				t.Fatalf("Width = %v, want frame %v", box.Width, alignedStyle.MaxWidth)
			}
			parentItem := 0
			parentHit := 0
			for i, line := range box.Lines {
				wantOffset := float32(0)
				if line.Box.Width < alignedStyle.MaxWidth {
					remaining := alignedStyle.MaxWidth - line.Box.Width
					if align == display.AlignCenter {
						wantOffset = remaining / 2
					} else {
						wantOffset = remaining
					}
				}
				if !closeHost(line.OffsetX, wantOffset) {
					t.Fatalf("line %d OffsetX = %v, want %v", i, line.OffsetX, wantOffset)
				}
				if i == 0 && line.Box.Width == alignedStyle.MaxWidth && line.OffsetX != 0 {
					t.Fatalf("exact-boundary line OffsetX = %v, want 0", line.OffsetX)
				}
				for j, item := range line.Box.Items {
					if got, want := box.Items[parentItem+j].Bound.X, item.Bound.X+wantOffset; !closeHost(got, want) {
						t.Fatalf("line %d item %d parent x = %v, want %v", i, j, got, want)
					}
					if len(item.Path) > 0 {
						got := box.Items[parentItem+j].Path[0].Points[0].X
						want := item.Path[0].Points[0].X + wantOffset
						if !closeHost(got, want) {
							t.Fatalf("line %d item %d path x = %v, want %v", i, j, got, want)
						}
					}
				}
				for j, hit := range line.Box.Hits {
					if got, want := box.Hits[parentHit+j].Area.X, hit.Area.X+wantOffset; !closeHost(got, want) {
						t.Fatalf("line %d hit %d parent x = %v, want %v", i, j, got, want)
					}
				}
				parentItem += len(line.Box.Items)
				parentHit += len(line.Box.Hits)
			}
			if err := box.Validate(); err != nil {
				t.Fatalf("Validate() error = %v", err)
			}
		})
	}
}

func TestDisplayAlignmentOneLineAndOverfullPolicy(t *testing.T) {
	t.Parallel()

	renderer, _, _ := newTestRenderer(t)
	root := ir.NewRow(source.NewRange(0, 3),
		glyph("x", ir.ClassNormal),
		glyph("=", ir.ClassRelation),
		glyph("y", ir.ClassNormal),
	)
	base, err := renderer.Layout(root, ir.Display, testStyle(16))
	if err != nil {
		t.Fatal(err)
	}
	for _, align := range []display.Align{display.AlignStart, display.AlignCenter, display.AlignEnd} {
		t.Run(alignName(align), func(t *testing.T) {
			style := testStyle(16)
			style.MaxWidth = base.Width + 20
			style.Align = align
			box, err := renderer.Layout(root, ir.Display, style)
			if err != nil {
				t.Fatalf("fitting Layout() error = %v", err)
			}
			if len(box.Lines) != 1 {
				t.Fatalf("fitting Lines = %d, want 1", len(box.Lines))
			}
			wantOffset := float32(0)
			switch align {
			case display.AlignCenter:
				wantOffset = 10
			case display.AlignEnd:
				wantOffset = 20
			}
			if got := box.Lines[0].OffsetX; !closeHost(got, wantOffset) {
				t.Fatalf("fitting OffsetX = %v, want %v", got, wantOffset)
			}
			if got := box.Items[0].Bound.X; !closeHost(got, wantOffset) {
				t.Fatalf("fitting parent item x = %v, want %v", got, wantOffset)
			}

			style.MaxWidth = base.Width / 2
			box, err = renderer.Layout(root, ir.Display, style)
			if err != nil {
				t.Fatalf("overfull Layout() error = %v", err)
			}
			if got := box.Lines[0].OffsetX; got != 0 {
				t.Fatalf("overfull OffsetX = %v, want 0", got)
			}
			if box.Overflow <= 0 {
				t.Fatalf("overfull Overflow = %v, want positive", box.Overflow)
			}
		})
	}
}

func TestDisplayAlignmentFrameHonorsMaxDimension(t *testing.T) {
	t.Parallel()

	state := alignmentLayoutState(display.AlignCenter, 10)
	state.limits.MaxDimension = 9
	_, err := state.assembleDisplayLines(&display.Box{}, []display.LineBox{alignmentLine(4, 10)})
	if !errors.Is(err, ErrLimit) {
		t.Fatalf("assembleDisplayLines() error = %v, want ErrLimit", err)
	}
}

func TestDisplayLogicalOverflowWithoutInk(t *testing.T) {
	t.Parallel()

	renderer, _, _ := newTestRenderer(t)
	trailingSpace := ir.NewRow(source.NewRange(0, 2),
		glyph("x", ir.ClassNormal),
		ir.NewSpace(source.NewRange(1, 2), ir.SpaceExplicit, emLength(t, 4)),
	)
	spaceOnly := ir.NewSpace(source.NewRange(0, 1), ir.SpaceExplicit, emLength(t, 4))
	tests := []struct {
		name string
		root ir.Node
	}{
		{name: "space only", root: spaceOnly},
		{name: "trailing space", root: trailingSpace},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			unwrapped, err := renderer.Layout(tc.root, ir.Display, testStyle(16))
			if err != nil {
				t.Fatalf("unwrapped Layout() error = %v", err)
			}
			style := testStyle(16)
			style.MaxWidth = 8
			box, err := renderer.Layout(tc.root, ir.Display, style)
			if err != nil {
				t.Fatalf("constrained Layout() error = %v", err)
			}
			if len(box.Lines) != 1 {
				t.Fatalf("Lines = %d, want one constrained line", len(box.Lines))
			}
			want := unwrapped.Width - style.MaxWidth
			if want <= 0 {
				t.Fatalf("fixture is not overfull: width %v max %v", unwrapped.Width, style.MaxWidth)
			}
			if !closeHost(box.Lines[0].Box.Overflow, want) {
				t.Fatalf("line Overflow = %v, want logical excess %v", box.Lines[0].Box.Overflow, want)
			}
			if !closeHost(box.Overflow, want) {
				t.Fatalf("parent Overflow = %v, want logical excess %v", box.Overflow, want)
			}
			if err := box.Validate(); err != nil {
				t.Fatalf("Validate() error = %v", err)
			}
		})
	}
}

func alignmentLayoutState(align display.Align, maxWidth float32) *layoutState {
	return &layoutState{
		ctx:      context.Background(),
		request:  display.Style{MaxWidth: maxWidth, Align: align},
		hostSize: hostSubunits,
		limits:   source.Limits{}.WithDefaults(),
	}
}

func alignmentLine(width, maxWidth float32) display.LineBox {
	overflow := maxFloat(0, width-maxWidth)
	return display.LineBox{Box: &display.Box{
		Width:    width,
		Ascent:   1,
		Overflow: overflow,
		Items: []display.Item{{
			Kind:  display.ItemRule,
			Bound: geometry.Rect{Y: -1, Width: width, Height: 1},
			Rect:  geometry.Rect{Y: -1, Width: width, Height: 1},
		}},
		Hits: []display.Hit{{
			Source: source.NewRange(0, 1),
			Area:   geometry.Rect{Y: -1, Width: width, Height: 1},
			Kind:   display.HitAtom,
		}},
	}}
}

func alignName(align display.Align) string {
	switch align {
	case display.AlignStart:
		return "start"
	case display.AlignCenter:
		return "center"
	case display.AlignEnd:
		return "end"
	default:
		return "invalid"
	}
}
