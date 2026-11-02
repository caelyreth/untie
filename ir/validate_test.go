package ir

import (
	"encoding/json"
	"strings"
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

func TestValidateRejectsInvalidSemanticRoles(t *testing.T) {
	t.Parallel()

	span := source.NewRange(0, 1)
	glyph := NewGlyph(span, "x")
	fraction := NewFraction(span, glyph, NewGlyph(span, "y"))
	fraction.Role = FractionRoleStacked

	scripts := NewScripts(span, glyph)
	scripts.Role = ScriptRoleBraceLabel + 1

	cancel := NewCancel(span, glyph)
	cancel.Role = CancelRoleNegation
	cancel.Cross = true

	table := NewTable(span, TableRow{Cells: []Node{glyph}})
	table.Role = TableRoleVector + 1

	for name, root := range map[string]Node{
		"fraction role without no-bar style": fraction,
		"script role":                        scripts,
		"negation with strike":               cancel,
		"table role":                         table,
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			if err := Validate(&Formula{Root: root}, source.Limits{}); err == nil {
				t.Fatal("Validate() error = nil")
			}
		})
	}
}

func TestDisplayOnlyLimitsAndNaturalFitContract(t *testing.T) {
	t.Parallel()

	span := source.NewRange(0, 3)
	scripts := NewScripts(span, NewGlyph(source.NewRange(0, 1), "x"))
	scripts.Top = NewGlyph(source.NewRange(2, 3), "2")
	scripts.Limits = LimitsDisplay
	if err := Validate(&Formula{Root: scripts}, source.Limits{}); err != nil {
		t.Fatalf("LimitsDisplay validation error = %v", err)
	}
	scripts.Limits = LimitsDisplay + 1
	if err := Validate(&Formula{Root: scripts}, source.Limits{}); err == nil {
		t.Fatal("invalid limits placement was accepted")
	}

	scripts.Limits = LimitsDisplay
	accent := NewAccent(span, NewGlyph(source.NewRange(0, 1), "x"), NewGlyph(source.NewRange(1, 2), "^"), PositionAbove)
	accent.Stretchy = false
	accent.NaturalFit = true
	data, err := MarshalCanonical(&Formula{Root: accent})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), `"natural-fit":true`) {
		t.Fatalf("canonical accent = %s, want natural-fit", data)
	}
}

func TestSizedDelimiterContract(t *testing.T) {
	t.Parallel()

	span := source.NewRange(0, 7)
	relative, err := PercentLength(150)
	if err != nil {
		t.Fatal(err)
	}
	for name, sizing := range map[string]DelimiterSizing{
		"absolute": AbsoluteDelimiterSizing(RawLength(LengthEm, 2_400_000)),
		"relative": RelativeDelimiterSizing(relative),
	} {
		sizing := sizing
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			node := NewSizedDelimiter(span, "(", sizing)
			node.Class = ClassOpening
			if err := Validate(&Formula{Root: node}, source.Limits{}); err != nil {
				t.Fatalf("Validate() error = %v", err)
			}
			encoded, err := MarshalCanonical(&Formula{Root: node})
			if err != nil {
				t.Fatalf("MarshalCanonical() error = %v", err)
			}
			for _, want := range []string{
				`"kind":"sized-delimiter"`,
				`"sizing":`,
				`"text":"("`,
			} {
				if !strings.Contains(string(encoded), want) {
					t.Fatalf("canonical node = %s, want %s", encoded, want)
				}
			}
			clone, err := CloneFormula(&Formula{Root: node})
			if err != nil {
				t.Fatalf("CloneFormula() error = %v", err)
			}
			cloned := clone.Root.(*SizedDelimiter)
			if cloned == node || cloned.Text != node.Text || cloned.Sizing != node.Sizing ||
				cloned.Span != node.Span || cloned.Origin != node.Origin {
				t.Fatalf("clone = %#v, want deep copy of %#v", cloned, node)
			}
		})
	}

	absolute := AbsoluteDelimiterSizing(RawLength(LengthEm, 1_200_000))
	tests := map[string]*SizedDelimiter{
		"empty text": {
			Base:   NewBase(KindSizedDelimiter, span),
			Sizing: absolute,
		},
		"multi-rune text": {
			Base:   NewBase(KindSizedDelimiter, span),
			Text:   "()",
			Sizing: absolute,
		},
		"content policy": {
			Base: NewBase(KindSizedDelimiter, span),
			Text: "(",
		},
		"invalid kind": {
			Base:   NewBase(KindSizedDelimiter, span),
			Text:   "(",
			Sizing: DelimiterSizing{Kind: DelimiterSizingAbsolute + 1},
		},
		"wrong relative unit": {
			Base:   NewBase(KindSizedDelimiter, span),
			Text:   "(",
			Sizing: RelativeDelimiterSizing(RawLength(LengthEm, 1_000_000)),
		},
		"wrong absolute unit": {
			Base:   NewBase(KindSizedDelimiter, span),
			Text:   "(",
			Sizing: AbsoluteDelimiterSizing(relative),
		},
		"negative absolute": {
			Base:   NewBase(KindSizedDelimiter, span),
			Text:   "(",
			Sizing: AbsoluteDelimiterSizing(RawLength(LengthEm, -1_000_000)),
		},
		"malformed UTF-8": {
			Base:   NewBase(KindSizedDelimiter, span),
			Text:   string([]byte{0xff}),
			Sizing: absolute,
		},
		"invalid class": {
			Base: func() Base {
				base := NewBase(KindSizedDelimiter, span)
				base.Class = ClassBinary
				return base
			}(),
			Text:   "(",
			Sizing: absolute,
		},
	}
	for name, node := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			if err := Validate(&Formula{Root: node}, source.Limits{}); err == nil {
				t.Fatal("Validate() error = nil")
			}
		})
	}
}

func TestDelimiterSizingAndUTFPolicies(t *testing.T) {
	t.Parallel()

	span := source.NewRange(0, 3)
	relative, err := PercentLength(125)
	if err != nil {
		t.Fatal(err)
	}
	validFences := []Fenced{
		{Base: NewBase(KindFenced, span), Body: []Node{NewGlyph(span, "x")}, Sizing: ContentDelimiterSizing()},
		{Base: NewBase(KindFenced, span), Body: []Node{NewGlyph(span, "x")}, Sizing: RelativeDelimiterSizing(relative)},
		{Base: NewBase(KindFenced, span), Body: []Node{NewGlyph(span, "x")}, Sizing: AbsoluteDelimiterSizing(RawLength(LengthEm, 2_000_000))},
	}
	for i := range validFences {
		if err := Validate(&Formula{Root: &validFences[i]}, source.Limits{}); err != nil {
			t.Fatalf("valid fence sizing %d error = %v", i, err)
		}
	}
	encoded, err := MarshalCanonical(&Formula{Root: &validFences[1]})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(encoded), `"sizing":{"kind":1,"value":[6,125000000]}`) {
		t.Fatalf("canonical fence = %s, want relative sizing", encoded)
	}
	cloned, err := CloneFormula(&Formula{Root: &validFences[1]})
	if err != nil {
		t.Fatal(err)
	}
	if cloned.Root.(*Fenced).Sizing != validFences[1].Sizing {
		t.Fatalf("cloned fence sizing = %#v, want %#v", cloned.Root.(*Fenced).Sizing, validFences[1].Sizing)
	}
	invalidFence := validFences[0]
	invalidFence.Sizing = RelativeDelimiterSizing(RawLength(LengthEm, 1_000_000))
	if err := Validate(&Formula{Root: &invalidFence}, source.Limits{}); err == nil {
		t.Fatal("relative fence sizing accepted a non-percent value")
	}

	invalidUTF8 := string([]byte{0xff})
	for name, root := range map[string]Node{
		"glyph":       NewGlyph(span, invalidUTF8),
		"text":        NewText(span, invalidUTF8),
		"operator":    NewOperator(span, invalidUTF8),
		"error":       NewError(span, invalidUTF8, source.CodeSyntax),
		"placeholder": NewPlaceholder(span, invalidUTF8),
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			if err := Validate(&Formula{Root: root}, source.Limits{}); err == nil {
				t.Fatalf("Validate() accepted malformed UTF-8 in %s", name)
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

func TestBreakCanonicalHasNoPenalty(t *testing.T) {
	t.Parallel()

	encoded, err := MarshalCanonical(&Formula{
		Root: NewBreak(source.NewRange(0, 1), BreakExplicit),
	})
	if err != nil {
		t.Fatalf("MarshalCanonical() error = %v", err)
	}
	if strings.Contains(string(encoded), "penalty") {
		t.Fatalf("canonical break retained a penalty: %s", encoded)
	}
}

func TestMarshalCanonicalIncludesSemanticRoles(t *testing.T) {
	t.Parallel()

	span := source.NewRange(0, 1)
	fraction := NewFraction(span, NewGlyph(span, "n"), NewGlyph(span, "k"))
	fraction.Role = FractionRoleBinomial
	fraction.Style = FractionNoBar
	fraction.Bar = false
	scripts := NewScripts(span, NewGlyph(span, "x"))
	scripts.Role = ScriptRoleOverlay
	scripts.Top = NewGlyph(span, "a")
	cancel := NewCancel(span, NewGlyph(span, "="))
	cancel.Role = CancelRoleNegation
	table := NewTable(span, TableRow{Cells: []Node{NewGlyph(span, "x")}})
	table.Role = TableRoleCases

	for _, root := range []Node{fraction, scripts, cancel, table} {
		encoded, err := MarshalCanonical(&Formula{Root: root})
		if err != nil {
			t.Fatalf("MarshalCanonical(%T) error = %v", root, err)
		}
		if !strings.Contains(string(encoded), `"role":`) {
			t.Fatalf("MarshalCanonical(%T) omitted role: %s", root, encoded)
		}
	}
}
