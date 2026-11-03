package untie

import (
	"context"
	"strings"
	"testing"

	"github.com/caelyreth/untie/display"
	"github.com/caelyreth/untie/source"
)

func TestRenderEmitsTextClusterHitsForBothDialects(t *testing.T) {
	t.Parallel()

	renderer, err := NewRenderer(loadTestFonts(t), RendererOptions{})
	if err != nil {
		t.Fatalf("NewRenderer() error = %v", err)
	}
	tests := []struct {
		name    string
		dialect Dialect
		source  string
	}{
		{name: "tex", dialect: TeX, source: `\text{ab}`},
		{name: "typst", dialect: Typst, source: `text("ab")`},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			result, err := renderer.Render(context.Background(), test.source, RenderRequest{
				Dialect: test.dialect,
				Style:   display.Style{Size: 16},
			})
			if err != nil {
				t.Fatalf("Render() error = %v", err)
			}
			a := strings.Index(test.source, "a")
			b := strings.Index(test.source, "b")
			aHit, aOK := findAtomHit(result.Box.Hits, source.NewRange(a, a+1))
			bHit, bOK := findAtomHit(result.Box.Hits, source.NewRange(b, b+1))
			if !aOK || !bOK {
				t.Fatalf("cluster hits = %#v, want atom hits for a and b", result.Box.Hits)
			}
			if aHit.Area.X >= bHit.Area.X || aHit.Area.Width <= 0 || bHit.Area.Width <= 0 {
				t.Fatalf("cluster geometry a=%#v b=%#v", aHit.Area, bHit.Area)
			}
		})
	}
}

func findAtomHit(hits []display.Hit, span source.Range) (display.Hit, bool) {
	for _, hit := range hits {
		if hit.Kind == display.HitAtom && hit.Source == span {
			return hit, true
		}
	}
	return display.Hit{}, false
}
