package ir

import (
	"bytes"
	"testing"

	"github.com/caelyreth/untie/source"
)

func TestFontSizeOverlayAndRaiseValidateCloneAndSerialize(t *testing.T) {
	t.Parallel()

	span := source.NewRange(0, 1)
	fontSize, err := PercentLength(150)
	if err != nil {
		t.Fatal(err)
	}
	raiseAmount, err := PtLength(-2.5)
	if err != nil {
		t.Fatal(err)
	}
	body := NewGlyph(span, "x")
	style := NewStyle(span, body)
	style.SetFontSize = true
	style.FontSize = fontSize
	raise := NewRaise(span, style, raiseAmount)
	overlay := NewOverlay(span, raise, OverlayCenter)
	formula := &Formula{Root: NewRow(span, overlay)}

	if err := Validate(formula, source.Limits{}); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
	encoded, err := MarshalCanonical(formula)
	if err != nil {
		t.Fatalf("MarshalCanonical() error = %v", err)
	}
	for _, want := range [][]byte{
		[]byte(`"kind":"overlay"`),
		[]byte(`"alignment":1`),
		[]byte(`"kind":"raise"`),
		[]byte(`"amount":[4,-2500000]`),
		[]byte(`"set-font-size":true`),
		[]byte(`"font-size":[6,150000000]`),
	} {
		if !bytes.Contains(encoded, want) {
			t.Errorf("canonical output missing %s: %s", want, encoded)
		}
	}

	cloned, err := CloneFormula(formula)
	if err != nil {
		t.Fatalf("CloneFormula() error = %v", err)
	}
	clonedOverlay := cloned.Root.(*Row).Items[0].(*Overlay)
	clonedRaise := clonedOverlay.Body.(*Raise)
	clonedStyle := clonedRaise.Body.(*Style)
	if clonedOverlay == overlay || clonedRaise == raise || clonedStyle == style || clonedStyle.Body == body {
		t.Fatal("CloneFormula shared node storage")
	}
	if clonedOverlay.Alignment != OverlayCenter || clonedRaise.Amount != raiseAmount ||
		clonedStyle.FontSize != fontSize {
		t.Fatal("CloneFormula did not preserve new primitive fields")
	}
}

func TestNewPrimitiveValidationRejectsMalformedFields(t *testing.T) {
	t.Parallel()

	span := source.NewRange(0, 1)
	body := NewGlyph(span, "x")
	tests := []struct {
		name string
		root Node
	}{
		{
			name: "font size without flag",
			root: func() Node {
				node := NewStyle(span, body)
				percent, _ := PercentLength(120)
				node.FontSize = percent
				return node
			}(),
		},
		{
			name: "non-percent font size",
			root: func() Node {
				node := NewStyle(span, body)
				node.SetFontSize = true
				em, _ := EmLength(1.2)
				node.FontSize = em
				return node
			}(),
		},
		{
			name: "zero font size",
			root: func() Node {
				node := NewStyle(span, body)
				node.SetFontSize = true
				node.FontSize = RawLength(LengthPercent, 0)
				return node
			}(),
		},
		{
			name: "invalid overlay alignment",
			root: NewOverlay(span, body, OverlayEnd+1),
		},
		{
			name: "percentage raise",
			root: NewRaise(span, body, RawLength(LengthPercent, 100_000_000)),
		},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			if err := Validate(&Formula{Root: test.root}, source.Limits{}); err == nil {
				t.Fatal("Validate() accepted malformed primitive")
			}
		})
	}
}

func TestNewPrimitiveKindNamesAreStable(t *testing.T) {
	t.Parallel()

	if got := KindName(KindOverlay); got != "overlay" {
		t.Fatalf("KindName(KindOverlay) = %q", got)
	}
	if got := KindName(KindRaise); got != "raise" {
		t.Fatalf("KindName(KindRaise) = %q", got)
	}
}
