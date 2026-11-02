package display

import (
	"encoding/json"
	"testing"

	"github.com/caelyreth/untie/geometry"
)

func TestCloneIndependent(t *testing.T) {
	t.Parallel()

	box := &Box{
		Width: 10,
		Items: []Item{{
			Kind: ItemGlyphPath,
			Path: geometry.Path{{Op: geometry.SegMoveTo}},
		}},
	}
	got := box.Clone()
	got.Items[0].Path[0].Op = geometry.SegClose
	if box.Items[0].Path[0].Op != geometry.SegMoveTo {
		t.Fatal("Clone shared path storage")
	}
}

func TestMarshalCanonical(t *testing.T) {
	t.Parallel()

	box := &Box{
		Width:  1,
		Ascent: 2,
		Items: []Item{{
			Kind:  ItemRule,
			Paint: Paint{Role: RoleInk},
			Rect:  geometry.Rect{Width: 1, Height: 0.1},
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
	if _, err := (*Box)(nil).MarshalCanonical(); err == nil {
		t.Fatal("nil box MarshalCanonical() error = nil")
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
