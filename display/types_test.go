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
		Items: []Item{{
			Kind:  ItemRule,
			Paint: Paint{Role: RoleInk},
			Rect:  geometry.Rect{Width: 1, Height: 0.1},
		}},
		Hits: []Hit{{
			Source: source.NewRange(0, 1),
			Area:   geometry.Rect{Width: 1, Height: 1},
			Depth:  1,
			Kind:   HitAtom,
		}},
		Breaks: []Break{{X: 0.5, Penalty: 10, Kind: BreakRelation}},
		Alt:    Alt{Text: "x", Speech: "x"},
		Lines: []LineBox{{
			Baseline: 1,
			Box: &Box{
				Width: 1,
				Items: []Item{{
					Kind:  ItemRule,
					Paint: Paint{Role: RoleInk},
					Rect:  geometry.Rect{Width: 1, Height: 0.1},
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
		!strings.Contains(string(first), `"alt":{"speech":"x","text":"x","mathml":""}`) ||
		!strings.Contains(string(first), `"lines":[{"baseline":1,"box":`) {
		t.Fatalf("canonical output omitted hits, breaks, alt, or lines: %s", first)
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
	const want = `{"speech":"x sub i squared","text":"x_i²","mathml":""}`
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
		{name: "deferred MathML", alt: Alt{MathML: "<math/>"}},
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
			name: "line wider than parent",
			box:  &Box{Lines: []LineBox{{Box: &Box{Width: 1}}}},
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
