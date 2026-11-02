package display

import (
	"encoding/json"
	"math"
	"strings"
	"testing"

	"github.com/caelyreth/untie/geometry"
	"github.com/caelyreth/untie/source"
)

func TestCloneIndependent(t *testing.T) {
	t.Parallel()

	box := &Box{
		Width: 10,
		Items: []Item{{
			Kind: ItemGlyphPath,
			Path: geometry.Path{{Op: geometry.SegMoveTo}},
		}},
		Hits: []Hit{{
			Source: source.NewRange(0, 1),
			Area:   geometry.Rect{Width: 1, Height: 1},
		}},
		Breaks: []Break{{X: 1, Kind: BreakRelation}},
		Alt:    Alt{Text: "x", Speech: "x"},
		Lines: []LineBox{{
			Baseline: 1,
			OffsetX:  2.5,
			Box: &Box{
				Width: 1,
				Items: []Item{{
					Kind: ItemGlyphPath,
					Path: geometry.Path{{Op: geometry.SegMoveTo}},
				}},
			},
		}},
	}
	got := box.Clone()
	got.Items[0].Path[0].Op = geometry.SegClose
	got.Hits[0].Area.Width = 2
	got.Breaks[0].X = 2
	got.Lines[0].Box.Items[0].Path[0].Op = geometry.SegLineTo
	if box.Items[0].Path[0].Op != geometry.SegMoveTo {
		t.Fatal("Clone shared path storage")
	}
	if box.Hits[0].Area.Width != 1 {
		t.Fatal("Clone shared hit storage")
	}
	if box.Breaks[0].X != 1 {
		t.Fatal("Clone shared break storage")
	}
	if box.Lines[0].Box.Items[0].Path[0].Op != geometry.SegMoveTo {
		t.Fatal("Clone shared line box storage")
	}
	if got.Lines[0].OffsetX != 2.5 {
		t.Fatalf("Clone OffsetX = %v, want 2.5", got.Lines[0].OffsetX)
	}
	if box.Alt != (Alt{Text: "x", Speech: "x"}) {
		t.Fatal("Clone changed alternative text")
	}
}

func TestMarshalCanonical(t *testing.T) {
	t.Parallel()

	box := &Box{
		Width:   1,
		Ascent:  2,
		Descent: 1,
		Italic:  -0.5,
		Items: []Item{{
			Kind:  ItemRule,
			Paint: Paint{Role: RoleInk},
			Bound: geometry.Rect{Y: 0.9, Width: 1, Height: 0.1},
			Rect:  geometry.Rect{Y: 0.9, Width: 1, Height: 0.1},
			At:    geometry.Point{Y: 1},
		}},
		Hits: []Hit{{
			Source: source.NewRange(0, 1),
			Area:   geometry.Rect{Y: 1, Width: 1, Height: 1},
			Depth:  1,
			Kind:   HitAtom,
		}},
		Breaks: []Break{{X: 0.5, Kind: BreakRelation}},
		Alt:    Alt{Text: "x", Speech: "x"},
		Lines: []LineBox{{
			Baseline: 1,
			Box: &Box{
				Width:  1,
				Ascent: 1,
				Italic: -0.25,
				Items: []Item{{
					Kind:  ItemRule,
					Paint: Paint{Role: RoleInk},
					Bound: geometry.Rect{Y: -0.1, Width: 1, Height: 0.1},
					Rect:  geometry.Rect{Y: -0.1, Width: 1, Height: 0.1},
				}},
				Hits: []Hit{{
					Source: source.NewRange(0, 1),
					Area:   geometry.Rect{Width: 1, Height: 1},
					Depth:  1,
					Kind:   HitAtom,
				}},
			},
		}},
	}
	first, err := box.MarshalCanonical()
	if err != nil {
		t.Fatalf("MarshalCanonical() error = %v", err)
	}
	second, err := box.MarshalCanonical()
	if err != nil {
		t.Fatalf("MarshalCanonical() error = %v", err)
	}
	if string(first) != string(second) {
		t.Fatalf("non-deterministic output:\n%s\n%s", first, second)
	}
	if !json.Valid(first) {
		t.Fatalf("invalid JSON: %s", first)
	}
	if !strings.Contains(string(first), `"hits"`) ||
		!strings.Contains(string(first), `"breaks"`) ||
		!strings.Contains(string(first), `"alt":{"speech":"x","text":"x"}`) ||
		!strings.Contains(string(first), `"italic":-0.5`) ||
		!strings.Contains(string(first), `"italic":-0.25`) ||
		!strings.Contains(string(first), `"lines":[{"baseline":1,"offsetX":0,"box":`) {
		t.Fatalf("canonical output omitted hits, breaks, alt, signed italic, or lines: %s", first)
	}
	if strings.Contains(string(first), `"penalty"`) {
		t.Fatalf("canonical output retained a break penalty: %s", first)
	}
	if _, err := (*Box)(nil).MarshalCanonical(); err == nil {
		t.Fatal("nil box MarshalCanonical() error = nil")
	}
}

func TestAltCloneValidateAndMarshalCanonical(t *testing.T) {
	t.Parallel()

	alt := Alt{
		Speech: "x sub i squared",
		Text:   "x_i²",
	}
	if got := alt.Clone(); got != alt {
		t.Fatalf("Clone() = %#v, want %#v", got, alt)
	}
	if err := alt.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
	first, err := alt.MarshalCanonical()
	if err != nil {
		t.Fatalf("MarshalCanonical() error = %v", err)
	}
	second, err := alt.Clone().MarshalCanonical()
	if err != nil {
		t.Fatalf("clone MarshalCanonical() error = %v", err)
	}
	if string(first) != string(second) {
		t.Fatalf("canonical output is nondeterministic:\n%s\n%s", first, second)
	}
	const want = `{"speech":"x sub i squared","text":"x_i²"}`
	if string(first) != want {
		t.Fatalf("MarshalCanonical() = %s, want %s", first, want)
	}
}

func TestAltValidationRejectsMalformedAndDeferredOutput(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		alt  Alt
	}{
		{name: "speech UTF-8", alt: Alt{Speech: string([]byte{0xff})}},
		{name: "text UTF-8", alt: Alt{Text: string([]byte{0xff})}},
		{name: "NUL", alt: Alt{Text: "x\x00y"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if err := tc.alt.Validate(); err == nil {
				t.Fatal("Validate() error = nil")
			}
			if _, err := tc.alt.MarshalCanonical(); err == nil {
				t.Fatal("MarshalCanonical() error = nil")
			}
		})
	}
}

func TestValidateBox(t *testing.T) {
	t.Parallel()

	box := &Box{
		Width: 10,
		Items: []Item{{
			Kind:  ItemGlyphPath,
			Bound: geometry.Rect{Width: 1, Height: 1},
			Path:  geometry.Path{{Op: geometry.SegMoveTo, Points: [3]geometry.Point{{X: 1, Y: 2}}}},
		}},
	}
	if err := box.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
	box.Items[0].Path[0].Op = geometry.SegOp(99)
	if err := box.Validate(); err == nil {
		t.Fatal("Validate() error = nil for invalid path")
	}
}

func TestBoxItalicCorrectionIsSignedAndFinite(t *testing.T) {
	t.Parallel()

	box := &Box{Italic: -0.75}
	if err := box.Validate(); err != nil {
		t.Fatalf("negative italic Validate() error = %v", err)
	}
	canonical, err := box.MarshalCanonical()
	if err != nil {
		t.Fatalf("negative italic MarshalCanonical() error = %v", err)
	}
	if !strings.Contains(string(canonical), `"italic":-0.75`) {
		t.Fatalf("canonical output lost signed italic correction: %s", canonical)
	}

	for _, value := range []float32{float32(math.NaN()), float32(math.Inf(1)), float32(math.Inf(-1))} {
		box.Italic = value
		if err := box.Validate(); err == nil {
			t.Fatalf("italic %v Validate() error = nil", value)
		}
		if _, err := box.MarshalCanonical(); err == nil {
			t.Fatalf("italic %v MarshalCanonical() error = nil", value)
		}
	}
}

func TestValidateBoxRejectsUnknownAndAmbiguousPaint(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		paint Paint
	}{
		{name: "unknown role", paint: Paint{Role: RolePlaceholder + 1}},
		{
			name: "explicit color with non-ink role",
			paint: Paint{
				Role:        RoleError,
				Explicit:    RGBA{R: 1, G: 2, B: 3, A: 4},
				HasExplicit: true,
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			box := &Box{Items: []Item{{
				Kind:  ItemRule,
				Paint: tc.paint,
				Rect:  geometry.Rect{Width: 1},
			}}}
			if err := box.Validate(); err == nil {
				t.Fatal("Validate() error = nil")
			}
			if _, err := box.MarshalCanonical(); err == nil {
				t.Fatal("MarshalCanonical() error = nil")
			}
		})
	}

	box := &Box{Items: []Item{{
		Kind: ItemRule,
		Paint: Paint{
			Role:        RoleInk,
			Explicit:    RGBA{R: 1, G: 2, B: 3, A: 4},
			HasExplicit: true,
		},
		Rect: geometry.Rect{Width: 1},
	}}}
	if err := box.Validate(); err != nil {
		t.Fatalf("explicit ink paint Validate() error = %v", err)
	}
}

func TestBoxAtPrefersInnermostThenSmallest(t *testing.T) {
	t.Parallel()

	box := &Box{
		Hits: []Hit{
			{Source: source.NewRange(0, 3), Area: geometry.Rect{Width: 10, Height: 10}, Depth: 1, Kind: HitGroup},
			{Source: source.NewRange(1, 2), Area: geometry.Rect{X: 4, Width: 2, Height: 2}, Depth: 2, Kind: HitAtom},
			{Source: source.NewRange(2, 3), Area: geometry.Rect{X: 3, Width: 4, Height: 4}, Depth: 2, Kind: HitGroup},
		},
	}
	got, ok := box.At(geometry.Point{X: 4.5, Y: 1})
	if !ok {
		t.Fatal("At() found no hit")
	}
	if got.Kind != HitAtom || got.Source != source.NewRange(1, 2) {
		t.Fatalf("At() = %#v, want inner atom", got)
	}
	if _, ok := box.At(geometry.Point{X: 11, Y: 1}); ok {
		t.Fatal("At() found a hit outside its bounds")
	}
	if _, ok := (*Box)(nil).At(geometry.Point{}); ok {
		t.Fatal("nil At() found a hit")
	}
}

func TestBoxCoveringSortsAndMerges(t *testing.T) {
	t.Parallel()

	box := &Box{
		Hits: []Hit{
			{Source: source.NewRange(3, 5), Area: geometry.Rect{X: 1, Y: 20, Width: 1, Height: 5}},
			{Source: source.NewRange(2, 4), Area: geometry.Rect{X: 2, Y: 0, Width: 2, Height: 10}},
			{Source: source.Detached(), Area: geometry.Rect{X: 8, Y: 30, Width: 1, Height: 1}},
			{Source: source.NewRange(0, 2), Area: geometry.Rect{X: 0, Y: 0, Width: 2, Height: 10}},
		},
	}
	got := box.Covering(source.NewRange(1, 4))
	want := []geometry.Rect{
		{X: 0, Y: 0, Width: 4, Height: 10},
		{X: 1, Y: 20, Width: 1, Height: 5},
	}
	if len(got) != len(want) {
		t.Fatalf("Covering() = %#v, want %#v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("Covering()[%d] = %#v, want %#v", i, got[i], want[i])
		}
	}
	if got := box.Covering(source.Detached()); got != nil {
		t.Fatalf("detached Covering() = %#v, want nil", got)
	}
}

func TestValidateBoxRejectsInvalidHitsAndBreaks(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		box  *Box
	}{
		{
			name: "hit source",
			box:  &Box{Hits: []Hit{{Source: source.Range{Start: 2, End: 1}, Area: geometry.Rect{}}}},
		},
		{
			name: "hit area",
			box:  &Box{Hits: []Hit{{Source: source.NewRange(0, 1), Area: geometry.Rect{Width: -1}}}},
		},
		{
			name: "hit depth",
			box:  &Box{Hits: []Hit{{Source: source.NewRange(0, 1), Depth: -1}}},
		},
		{
			name: "hit kind",
			box:  &Box{Hits: []Hit{{Source: source.NewRange(0, 1), Kind: HitOther + 1}}},
		},
		{
			name: "break x",
			box:  &Box{Breaks: []Break{{X: float32(math.Inf(1))}}},
		},
		{
			name: "break kind",
			box:  &Box{Breaks: []Break{{Kind: BreakExplicit + 1}}},
		},
		{
			name: "alt",
			box:  &Box{Alt: Alt{Text: "x\x00y"}},
		},
		{
			name: "negative line baseline",
			box:  &Box{Lines: []LineBox{{Baseline: -1, Box: &Box{}}}},
		},
		{
			name: "negative line offset",
			box:  &Box{Lines: []LineBox{{OffsetX: -1, Box: &Box{}}}},
		},
		{
			name: "non-finite line offset",
			box:  &Box{Lines: []LineBox{{OffsetX: float32(math.Inf(1)), Box: &Box{}}}},
		},
		{
			name: "nil line box",
			box:  &Box{Lines: []LineBox{{}}},
		},
		{
			name: "nested lines",
			box: &Box{Lines: []LineBox{{
				Box: &Box{Lines: []LineBox{{Box: &Box{}}}},
			}}},
		},
		{
			name: "line wider than parent without overflow",
			box:  &Box{Lines: []LineBox{{Box: &Box{Width: 1}}}},
		},
		{
			name: "overfull line with nonzero offset",
			box: &Box{Lines: []LineBox{{
				OffsetX: 1,
				Box:     &Box{Width: 2, Overflow: 1},
			}}},
		},
		{
			name: "fitting line exceeds parent frame",
			box: &Box{Lines: []LineBox{{
				OffsetX: 2,
				Box:     &Box{Width: 2},
			}}},
		},
		{
			name: "decreasing baselines",
			box: &Box{
				Width: 1, Ascent: 2,
				Lines: []LineBox{
					{Baseline: 1, Box: &Box{}},
					{Baseline: 0, Box: &Box{}},
				},
			},
		},
		{
			name: "inconsistent baseline spacing",
			box: &Box{
				Width: 1, Ascent: 4, Descent: 4,
				Lines: []LineBox{
					{Baseline: 0, Box: &Box{Ascent: 1, Descent: 2}},
					{Baseline: 4, Box: &Box{Ascent: 1}},
				},
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if err := tc.box.Validate(); err == nil {
				t.Fatal("Validate() error = nil")
			}
		})
	}
}

func TestValidateWrappedBoxAggregates(t *testing.T) {
	t.Parallel()

	item := Item{
		Kind:  ItemRule,
		Paint: Paint{Role: RoleInk},
		Rect:  geometry.Rect{Width: 1},
	}
	hit := Hit{
		Source: source.NewRange(0, 1),
		Area:   geometry.Rect{Width: 1, Height: 1},
		Kind:   HitAtom,
	}
	line := func() *Box {
		return &Box{
			Width:  1,
			Ascent: 1,
			Items:  []Item{item},
			Hits:   []Hit{hit},
		}
	}
	valid := &Box{
		Width:  1,
		Ascent: 1,
		Items:  []Item{item},
		Hits:   []Hit{hit},
		Lines:  []LineBox{{Box: line()}},
	}
	if err := valid.Validate(); err != nil {
		t.Fatalf("aggregate Validate() error = %v", err)
	}

	emptyLine := &Box{Lines: []LineBox{{Box: &Box{}}}}
	if err := emptyLine.Validate(); err != nil {
		t.Fatalf("empty line Validate() error = %v", err)
	}

	alignedLocalItem := Item{
		Kind:  ItemRule,
		Paint: Paint{Role: RoleInk},
		Bound: geometry.Rect{X: 1, Y: -1, Width: 1, Height: 1},
		Rect:  geometry.Rect{X: 1, Y: -1, Width: 1, Height: 1},
	}
	alignedLocalHit := Hit{
		Source: source.NewRange(0, 1),
		Area:   geometry.Rect{X: 1, Y: -1, Width: 1, Height: 1},
		Kind:   HitAtom,
	}
	aligned := &Box{
		Width: 4, Ascent: 1,
		Items: []Item{translateTestItem(alignedLocalItem, 2, 0)},
		Hits:  []Hit{translateTestHit(alignedLocalHit, 2, 0)},
		Lines: []LineBox{{
			OffsetX: 2,
			Box: &Box{
				Width: 2, Ascent: 1,
				Items: []Item{alignedLocalItem},
				Hits:  []Hit{alignedLocalHit},
			},
		}},
	}
	if err := aligned.Validate(); err != nil {
		t.Fatalf("aligned aggregate Validate() error = %v", err)
	}

	tests := []struct {
		name string
		box  *Box
	}{
		{
			name: "missing parent items",
			box: &Box{
				Width: 1, Ascent: 1, Hits: []Hit{hit},
				Lines: []LineBox{{Box: line()}},
			},
		},
		{
			name: "missing parent hits",
			box: &Box{
				Width: 1, Ascent: 1, Items: []Item{item},
				Lines: []LineBox{{Box: line()}},
			},
		},
		{
			name: "extra parent item",
			box: &Box{
				Width: 1, Ascent: 1, Items: []Item{item, item}, Hits: []Hit{hit},
				Lines: []LineBox{{Box: line()}},
			},
		},
		{
			name: "extra parent hit",
			box: &Box{
				Width: 1, Ascent: 1, Items: []Item{item}, Hits: []Hit{hit, hit},
				Lines: []LineBox{{Box: line()}},
			},
		},
		{
			name: "parent item coordinate differs",
			box: &Box{
				Width: 4, Ascent: 1,
				Items: []Item{{Kind: ItemRule, Paint: Paint{Role: RoleInk}, Rect: geometry.Rect{X: 9}}},
				Hits:  []Hit{translateTestHit(alignedLocalHit, 2, 0)},
				Lines: []LineBox{{
					OffsetX: 2,
					Box:     &Box{Width: 2, Ascent: 1, Items: []Item{alignedLocalItem}, Hits: []Hit{alignedLocalHit}},
				}},
			},
		},
		{
			name: "parent hit coordinate differs",
			box: &Box{
				Width: 4, Ascent: 1,
				Items: []Item{translateTestItem(alignedLocalItem, 2, 0)},
				Hits:  []Hit{{Source: alignedLocalHit.Source, Area: geometry.Rect{X: 9}, Kind: HitAtom}},
				Lines: []LineBox{{
					OffsetX: 2,
					Box:     &Box{Width: 2, Ascent: 1, Items: []Item{alignedLocalItem}, Hits: []Hit{alignedLocalHit}},
				}},
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if err := tc.box.Validate(); err == nil {
				t.Fatal("Validate() error = nil")
			}
			if _, err := tc.box.MarshalCanonical(); err == nil {
				t.Fatal("MarshalCanonical() error = nil")
			}
		})
	}
}

func TestValidateWrappedOverflowCertification(t *testing.T) {
	t.Parallel()

	rule := Item{
		Kind:  ItemRule,
		Paint: Paint{Role: RoleInk},
		Bound: geometry.Rect{Y: -1, Width: 4, Height: 1},
		Rect:  geometry.Rect{Y: -1, Width: 4, Height: 1},
	}
	wrapped := func(line *Box) *Box {
		box := &Box{
			Width:  4,
			Ascent: 1,
			Lines:  []LineBox{{Box: line}},
		}
		for _, item := range line.Items {
			box.Items = append(box.Items, translateTestItem(item, 0, 0))
		}
		return box
	}

	valid := wrapped(&Box{Width: 4, Ascent: 1, Items: []Item{rule}})
	if err := valid.Validate(); err != nil {
		t.Fatalf("valid wrapped box error = %v", err)
	}

	lineLogical := wrapped(&Box{Width: 6, Ascent: 1, Items: []Item{rule}})
	lineLogical.Overflow = 1
	if err := lineLogical.Validate(); err == nil {
		t.Fatal("line logical overflow was not rejected")
	}

	parentLogical := wrapped(&Box{
		Width: 6, Ascent: 1, Overflow: 1,
		Items: []Item{rule},
	})
	if err := parentLogical.Validate(); err == nil {
		t.Fatal("parent logical overflow was not rejected")
	}

	wideRule := rule
	wideRule.Bound.Width = 5
	wideRule.Rect.Width = 5
	exactInk := wrapped(&Box{Width: 4, Ascent: 1, Overflow: 1, Items: []Item{wideRule}})
	exactInk.Overflow = 1
	if err := exactInk.Validate(); err != nil {
		t.Fatalf("exact ink overflow error = %v", err)
	}

	lineInk := wrapped(&Box{Width: 4, Ascent: 1, Items: []Item{wideRule}})
	lineInk.Overflow = 1
	if err := lineInk.Validate(); err == nil {
		t.Fatal("line ink overflow was not rejected")
	}

	parentInk := wrapped(&Box{Width: 4, Ascent: 1, Overflow: 0.5, Items: []Item{wideRule}})
	if err := parentInk.Validate(); err == nil {
		t.Fatal("parent ink overflow was not rejected")
	}

	path := geometry.Path{
		{Op: geometry.SegMoveTo, Points: [3]geometry.Point{{X: 0, Y: -1}}},
		{Op: geometry.SegLineTo, Points: [3]geometry.Point{{X: 6, Y: -1}}},
	}
	pathItem := Item{
		Kind:  ItemGlyphPath,
		Paint: Paint{Role: RoleInk},
		Bound: geometry.Rect{Y: -1, Width: 4, Height: 1},
		Path:  path,
	}
	linePath := wrapped(&Box{Width: 4, Ascent: 1, Items: []Item{pathItem}})
	linePath.Overflow = 2
	if err := linePath.Validate(); err == nil {
		t.Fatal("line path extrema overflow was not rejected")
	}

	parentPath := wrapped(&Box{Width: 4, Ascent: 1, Overflow: 2, Items: []Item{pathItem}})
	parentPath.Overflow = 1
	if err := parentPath.Validate(); err == nil {
		t.Fatal("parent path extrema overflow was not rejected")
	}

	tallRule := rule
	tallRule.Bound.Y = -2
	parentVertical := wrapped(&Box{Width: 4, Ascent: 1, Overflow: 1, Items: []Item{tallRule}})
	if err := parentVertical.Validate(); err == nil {
		t.Fatal("parent vertical ink overflow was not rejected")
	}
}

func translateTestItem(item Item, dx, dy float32) Item {
	item.Bound.X += dx
	item.Bound.Y += dy
	item.Rect.X += dx
	item.Rect.Y += dy
	item.At.X += dx
	item.At.Y += dy
	for i := range item.Path {
		for j := range item.Path[i].Points {
			item.Path[i].Points[j].X += dx
			item.Path[i].Points[j].Y += dy
		}
	}
	return item
}

func translateTestHit(hit Hit, dx, dy float32) Hit {
	hit.Area.X += dx
	hit.Area.Y += dy
	return hit
}
