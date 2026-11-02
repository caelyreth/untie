package tex

import (
	"testing"

	"github.com/caelyreth/untie/ir"
)

func TestExplicitMathSpacingCommandsUseExactMuLengths(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		source string
		wantMu int64
	}{
		{name: "comma", source: `a\,b`, wantMu: 3},
		{name: "thinspace", source: `a\thinspace b`, wantMu: 3},
		{name: "colon", source: `a\:b`, wantMu: 4},
		{name: "greater-than", source: `a\>b`, wantMu: 4},
		{name: "medspace", source: `a\medspace b`, wantMu: 4},
		{name: "semicolon", source: `a\;b`, wantMu: 5},
		{name: "thickspace", source: `a\thickspace b`, wantMu: 5},
		{name: "bang", source: `a\!b`, wantMu: -3},
		{name: "negthinspace", source: `a\negthinspace b`, wantMu: -3},
		{name: "negmedspace", source: `a\negmedspace b`, wantMu: -4},
		{name: "negthickspace", source: `a\negthickspace b`, wantMu: -5},
		{name: "enspace", source: `a\enspace b`, wantMu: 9},
		{name: "enskip", source: `a\enskip b`, wantMu: 9},
		{name: "quad", source: `a\quad b`, wantMu: 18},
		{name: "qquad", source: `a\qquad b`, wantMu: 36},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			result := parseTest(t, tc.source, Options{})
			if len(result.Diagnostics) != 0 {
				t.Fatalf("unexpected diagnostics: %v", result.Diagnostics)
			}
			items := rowItems(t, result.Root)
			if len(items) != 3 {
				t.Fatalf("top-level items = %d, want 3", len(items))
			}
			space := requireKind[*ir.Space](t, items[1])
			if space.Type != ir.SpaceExplicit {
				t.Fatalf("space type = %v, want %v", space.Type, ir.SpaceExplicit)
			}
			if want := ir.RawLength(ir.LengthMu, tc.wantMu*1_000_000); space.Amount != want {
				t.Fatalf("space amount = %+v, want %+v", space.Amount, want)
			}
		})
	}
}

func TestWordSpacingCommandsRemainInterword(t *testing.T) {
	t.Parallel()

	for _, source := range []string{`a\ b`, `a\space b`, `a\nobreakspace b`} {
		source := source
		t.Run(source, func(t *testing.T) {
			t.Parallel()

			result := parseTest(t, source, Options{})
			items := rowItems(t, result.Root)
			space := requireKind[*ir.Space](t, items[1])
			if space.Type != ir.SpaceInterword {
				t.Fatalf("space type = %v, want %v", space.Type, ir.SpaceInterword)
			}
			if !space.Amount.IsZero() {
				t.Fatalf("interword amount = %+v, want zero", space.Amount)
			}
		})
	}
}
