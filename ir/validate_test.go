package ir

import (
	"encoding/json"
	"testing"

	"github.com/caelyreth/untie/source"
)

func TestValidateTree(t *testing.T) {
	t.Parallel()

	root := NewRow(source.NewRange(0, 6),
		NewGlyph(source.NewRange(0, 1), "x"),
		NewScripts(source.NewRange(0, 3), NewGlyph(source.NewRange(0, 1), "x")),
	)
	scripts := root.Items[1].(*Scripts)
	scripts.Top = NewGlyph(source.NewRange(2, 3), "2")
	if err := Validate(&Formula{Root: root}, source.Limits{}); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
}

func TestValidateRejectsBadNodes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		root Node
	}{
		{name: "nil", root: nil},
		{name: "empty glyph", root: NewGlyph(source.NewRange(0, 0), "")},
		{name: "nil fraction part", root: NewFraction(source.NewRange(0, 1), NewGlyph(source.NewRange(0, 1), "x"), nil)},
		{name: "invalid span", root: NewGlyph(source.Range{Start: 3, End: 2}, "x")},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if err := Validate(&Formula{Root: tc.root}, source.Limits{}); err == nil {
				t.Fatal("Validate() error = nil, want failure")
			}
		})
	}
}

func TestMarshalCanonicalIsStable(t *testing.T) {
	t.Parallel()

	root := NewRow(source.NewRange(0, 3),
		NewGlyph(source.NewRange(0, 1), "x"),
		NewFraction(source.NewRange(1, 3),
			NewGlyph(source.NewRange(1, 2), "a"),
			NewGlyph(source.NewRange(2, 3), "b"),
		),
	)
	a, err := MarshalCanonical(&Formula{Root: root})
	if err != nil {
		t.Fatalf("MarshalCanonical() error = %v", err)
	}
	b, err := MarshalCanonical(&Formula{Root: root})
	if err != nil {
		t.Fatalf("MarshalCanonical() error = %v", err)
	}
	if string(a) != string(b) {
		t.Fatalf("canonical output differs:\n%s\n%s", a, b)
	}
	if !json.Valid(a) {
		t.Fatalf("canonical output is not JSON: %s", a)
	}
}
