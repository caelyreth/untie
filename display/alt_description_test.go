package display

import (
	"bytes"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/caelyreth/untie/source"
)

func TestAltDescriptionCloneValidateAndCanonical(t *testing.T) {
	t.Parallel()

	description := testAltDescription()
	if err := description.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
	first, err := description.MarshalCanonical()
	if err != nil {
		t.Fatalf("MarshalCanonical() error = %v", err)
	}
	clone := description.Clone()
	second, err := clone.MarshalCanonical()
	if err != nil {
		t.Fatalf("clone MarshalCanonical() error = %v", err)
	}
	if !bytes.Equal(first, second) {
		t.Fatalf("clone changed canonical JSON:\n%s\n%s", first, second)
	}
	const want = `{"version":2,"root":{"kind":"fraction","fractionRole":"ordinary","fractionStyle":"vertical","parts":[{"role":"numerator","node":{"kind":"glyph","text":"1"}},{"role":"denominator","node":{"kind":"fraction","fractionRole":"binomial","fractionStyle":"no-bar","parts":[{"role":"numerator","node":{"kind":"glyph","text":"n"}},{"role":"denominator","node":{"kind":"glyph","text":"k"}}]}}]}}`
	if string(first) != want {
		t.Fatalf("MarshalCanonical() = %s, want %s", first, want)
	}

	clone.Root.Parts[0].Node.Text = "changed"
	clone.Tags = append(clone.Tags, AltTag{Manual: true, Line: 0, Body: &AltNode{
		Kind: AltKindGlyph,
		Text: "tag",
	}})
	if description.Root.Parts[0].Node.Text != "1" || len(description.Tags) != 0 {
		t.Fatal("Clone shared nested description storage")
	}
	if !description.Equal(testAltDescription()) {
		t.Fatal("Equal() rejected structurally equal descriptions")
	}
}

func TestAltDescriptionJSONRoundTrip(t *testing.T) {
	t.Parallel()

	description := testAltDescription()
	description.Tags = []AltTag{{
		Manual:  true,
		Starred: true,
		Line:    FormulaTagLine,
		Body: &AltNode{
			Kind: AltKindGlyph,
			Text: "1",
		},
	}}
	encoded, err := description.MarshalCanonical()
	if err != nil {
		t.Fatalf("MarshalCanonical() error = %v", err)
	}
	var decoded AltDescription
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}
	if err := decoded.Validate(); err != nil {
		t.Fatalf("decoded Validate() error = %v", err)
	}
	if !decoded.Equal(description) {
		t.Fatalf("JSON round trip changed description:\noriginal: %#v\ndecoded:  %#v",
			description, decoded)
	}
}

func TestAltDescriptionValidationRejectsMalformedTrees(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		edit func(*AltDescription)
	}{
		{name: "version", edit: func(d *AltDescription) { d.Version++ }},
		{name: "kind", edit: func(d *AltDescription) { d.Root.Kind = "" }},
		{name: "text UTF-8", edit: func(d *AltDescription) { d.Root.Parts[0].Node.Text = string([]byte{0xff}) }},
		{name: "text NUL", edit: func(d *AltDescription) { d.Root.Parts[0].Node.Text = "x\x00y" }},
		{name: "fraction role", edit: func(d *AltDescription) { d.Root.FractionRole = "" }},
		{name: "fraction payload", edit: func(d *AltDescription) { d.Root.Parts = d.Root.Parts[:1] }},
		{name: "text on wrapper", edit: func(d *AltDescription) { d.Root.Text = "unexpected" }},
		{name: "flag on wrapper", edit: func(d *AltDescription) {
			d.Root.Flags = []AltFlag{AltFlagBalanced}
		}},
		{name: "error code", edit: func(d *AltDescription) {
			d.Root = AltNode{Kind: AltKindError, Text: "bad"}
		}},
		{name: "tag body", edit: func(d *AltDescription) {
			d.Tags = []AltTag{{Manual: true, Line: FormulaTagLine}}
		}},
		{name: "suppressed tag body", edit: func(d *AltDescription) {
			d.Tags = []AltTag{{
				Suppressed: true,
				Line:       FormulaTagLine,
				Body:       &AltNode{Kind: AltKindGlyph, Text: "1"},
			}}
		}},
		{name: "nonmanual tag", edit: func(d *AltDescription) {
			d.Tags = []AltTag{{
				Line: FormulaTagLine,
				Body: &AltNode{Kind: AltKindGlyph, Text: "1"},
			}}
		}},
		{name: "mixed tag scopes", edit: func(d *AltDescription) {
			d.Tags = []AltTag{
				{Manual: true, Line: FormulaTagLine, Body: &AltNode{Kind: AltKindGlyph, Text: "1"}},
				{Manual: true, Line: 0, Body: &AltNode{Kind: AltKindGlyph, Text: "2"}},
			}
		}},
		{name: "unordered row tags", edit: func(d *AltDescription) {
			d.Tags = []AltTag{
				{Manual: true, Line: 1, Body: &AltNode{Kind: AltKindGlyph, Text: "1"}},
				{Manual: true, Line: 0, Body: &AltNode{Kind: AltKindGlyph, Text: "2"}},
			}
		}},
		{name: "duplicate row tags", edit: func(d *AltDescription) {
			d.Tags = []AltTag{
				{Suppressed: true, Line: 0},
				{Manual: true, Line: 0, Body: &AltNode{Kind: AltKindGlyph, Text: "1"}},
			}
		}},
		{name: "overlay alignment", edit: func(d *AltDescription) {
			d.Root = AltNode{Kind: AltKindOverlay, Parts: []AltPart{
				{Role: AltPartBody, Node: AltNode{Kind: AltKindGlyph, Text: "x"}},
			}}
		}},
		{name: "raise direction", edit: func(d *AltDescription) {
			d.Root = AltNode{Kind: AltKindRaise, Parts: []AltPart{
				{Role: AltPartBody, Node: AltNode{Kind: AltKindGlyph, Text: "x"}},
			}}
		}},
		{name: "font size", edit: func(d *AltDescription) {
			d.Root = AltNode{
				Kind:     AltKindStyle,
				FontSize: &AltFontSize{Percent: 0},
				Parts: []AltPart{{
					Role: AltPartBody,
					Node: AltNode{Kind: AltKindGlyph, Text: "x"},
				}},
			}
		}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			description := testAltDescription()
			tc.edit(description)
			if err := description.Validate(); !errors.Is(err, ErrInvalidAltDescription) {
				t.Fatalf("Validate() error = %v, want ErrInvalidAltDescription", err)
			}
			if _, err := description.MarshalCanonical(); !errors.Is(err, ErrInvalidAltDescription) {
				t.Fatalf("MarshalCanonical() error = %v, want ErrInvalidAltDescription", err)
			}
			if err := (Alt{Description: description}).Validate(); !errors.Is(err, ErrInvalidAlt) ||
				!errors.Is(err, ErrInvalidAltDescription) {
				t.Fatalf("Alt.Validate() error = %v, want ErrInvalidAlt and ErrInvalidAltDescription", err)
			}
		})
	}
}

func TestAltDescriptionLimits(t *testing.T) {
	t.Parallel()

	description := testAltDescription()
	if err := description.ValidateWithLimits(source.Limits{MaxNodes: 2}); !errors.Is(err, ErrInvalidAltDescription) ||
		!strings.Contains(err.Error(), "node limit") {
		t.Fatalf("ValidateWithLimits(nodes) error = %v", err)
	}
	if err := description.ValidateWithLimits(source.Limits{MaxNesting: 1}); !errors.Is(err, ErrInvalidAltDescription) ||
		!strings.Contains(err.Error(), "nesting limit") {
		t.Fatalf("ValidateWithLimits(nesting) error = %v", err)
	}
	if err := description.ValidateWithLimits(source.Limits{MaxTextBytes: 2}); !errors.Is(err, ErrInvalidAltDescription) ||
		!strings.Contains(err.Error(), "text payload") {
		t.Fatalf("ValidateWithLimits(text) error = %v", err)
	}
	alt := Alt{Description: description}
	if err := alt.ValidateWithLimits(source.Limits{MaxNodes: 2}); !errors.Is(err, ErrInvalidAlt) ||
		!errors.Is(err, ErrInvalidAltDescription) {
		t.Fatalf("Alt.ValidateWithLimits() error = %v", err)
	}
}

func TestAltCloneEqualAndValidation(t *testing.T) {
	t.Parallel()

	alt := Alt{
		Speech:      "1 over 2",
		Text:        "1/2",
		Description: testAltDescription(),
	}
	clone := alt.Clone()
	if !clone.Equal(alt) {
		t.Fatalf("Clone() = %#v, want %#v", clone, alt)
	}
	clone.Description.Root.Parts[0].Node.Text = "changed"
	if alt.Description.Root.Parts[0].Node.Text != "1" {
		t.Fatal("Alt.Clone shared the structured description")
	}
	if clone.Equal(alt) {
		t.Fatal("Equal() accepted a changed description")
	}
	if err := alt.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
	canonical, err := alt.MarshalCanonical()
	if err != nil {
		t.Fatalf("Alt.MarshalCanonical() error = %v", err)
	}
	if !strings.Contains(string(canonical), `"description":{"version":2`) {
		t.Fatalf("Alt.MarshalCanonical() omitted description: %s", canonical)
	}
}

func TestAltDescriptionTeXCompatibilityFields(t *testing.T) {
	t.Parallel()

	description := &AltDescription{
		Version: AltDescriptionVersion,
		Root: AltNode{
			Kind:             AltKindOverlay,
			OverlayAlignment: AltOverlayCenter,
			Parts: []AltPart{{
				Role: AltPartBody,
				Node: AltNode{
					Kind: AltKindStyle,
					FontSize: &AltFontSize{
						Percent: 248.8,
					},
					Parts: []AltPart{{
						Role: AltPartBody,
						Node: AltNode{
							Kind:           AltKindRaise,
							RaiseDirection: AltRaiseDown,
							Parts: []AltPart{{
								Role: AltPartBody,
								Node: AltNode{Kind: AltKindGlyph, Text: "x"},
							}},
						},
					}},
				},
			}},
		},
		Tags: []AltTag{
			{Manual: true, Line: 0, Body: &AltNode{Kind: AltKindGlyph, Text: "1"}},
			{Suppressed: true, Line: 1},
		},
	}
	if err := description.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
	clone := description.Clone()
	clone.Root.Parts[0].Node.FontSize.Percent = 50
	if description.Root.Parts[0].Node.FontSize.Percent != 248.8 {
		t.Fatal("Clone shared font-size metadata")
	}
	encoded, err := description.MarshalCanonical()
	if err != nil {
		t.Fatalf("MarshalCanonical() error = %v", err)
	}
	for _, want := range []string{
		`"kind":"overlay"`,
		`"overlayAlignment":"center"`,
		`"fontSize":{"percent":248.8}`,
		`"kind":"raise"`,
		`"raise":"down"`,
		`"line":0`,
		`"line":1`,
	} {
		if !strings.Contains(string(encoded), want) {
			t.Fatalf("canonical description omitted %s: %s", want, encoded)
		}
	}
}

func testAltDescription() *AltDescription {
	return &AltDescription{
		Version: AltDescriptionVersion,
		Root: AltNode{
			Kind:          AltKindFraction,
			FractionRole:  AltFractionOrdinary,
			FractionStyle: AltFractionVertical,
			Parts: []AltPart{
				{
					Role: AltPartNumerator,
					Node: AltNode{Kind: AltKindGlyph, Text: "1"},
				},
				{
					Role: AltPartDenominator,
					Node: AltNode{
						Kind:          AltKindFraction,
						FractionRole:  AltFractionBinomial,
						FractionStyle: AltFractionNoBar,
						Parts: []AltPart{
							{
								Role: AltPartNumerator,
								Node: AltNode{Kind: AltKindGlyph, Text: "n"},
							},
							{
								Role: AltPartDenominator,
								Node: AltNode{Kind: AltKindGlyph, Text: "k"},
							},
						},
					},
				},
			},
		},
	}
}
