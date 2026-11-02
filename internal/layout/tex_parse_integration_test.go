//go:build layout_tex_integration

package layout

import (
	"testing"

	"github.com/caelyreth/untie/internal/tex"
	"github.com/caelyreth/untie/ir"
	"github.com/caelyreth/untie/source"
)

// This tagged test crosses the public TeX parser boundary without weakening
// the default architecture allowlist. CI can run it in a dedicated job:
//
//	go test -tags=layout_tex_integration ./internal/layout
func TestPublicTeXParseIRLayout(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		source string
		want   ir.NodeKind
	}{
		{"matrix", `\begin{matrix}a&b\\c&d\end{matrix}`, ir.KindTable},
		{"cases", `\begin{cases}x&x>0\\-x&x<0\end{cases}`, ir.KindFenced},
		{"aligned", `\begin{aligned}x&=1\\y&=2\end{aligned}`, ir.KindMultiline},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			parsed := tex.Parse(test.source, ir.Display, tex.Options{})
			if len(parsed.Diagnostics) != 0 {
				t.Fatalf("parse diagnostics = %v", parsed.Diagnostics)
			}
			found := false
			if err := ir.WalkWithLimits(parsed.Root, source.Limits{}, func(node ir.Node) bool {
				if node.Kind() == test.want {
					found = true
				}
				return true
			}); err != nil {
				t.Fatal(err)
			}
			if !found {
				t.Fatalf("root = %T, want a node with kind %v", parsed.Root, test.want)
			}
			renderer, _, _ := newTestRenderer(t)
			style := testStyle(16)
			style.GlyphRefs = true
			box, err := renderer.Layout(parsed.Root, ir.Display, style)
			if err != nil {
				t.Fatal(err)
			}
			if err := box.Validate(); err != nil {
				t.Fatal(err)
			}
			if len(box.Items) == 0 || box.Width <= 0 || box.Ascent <= 0 {
				t.Fatalf("box = %+v", box)
			}
		})
	}
}
