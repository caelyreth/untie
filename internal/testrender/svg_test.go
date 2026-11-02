package testrender

import (
	"encoding/xml"
	"strings"
	"testing"

	"github.com/caelyreth/untie/display"
	"github.com/caelyreth/untie/geometry"
)

func TestSVG(t *testing.T) {
	t.Parallel()

	box := &display.Box{
		Width:   10,
		Ascent:  8,
		Descent: 2,
		Items: []display.Item{
			{
				Kind: display.ItemGlyphPath,
				Path: geometry.Path{
					{Op: geometry.SegMoveTo, Points: [3]geometry.Point{{X: 0, Y: -5}}},
					{Op: geometry.SegQuadTo, Points: [3]geometry.Point{{X: 2, Y: -8}, {X: 4, Y: -5}}},
					{Op: geometry.SegClose},
				},
			},
			{
				Kind:  display.ItemRule,
				Rect:  geometry.Rect{X: 2, Y: 0, Width: 5, Height: 0.5},
				Paint: display.Paint{Role: display.RoleError},
			},
		},
	}
	got := SVG(box, Options{Padding: 2, Title: "x < y"})
	var doc any
	if err := xml.Unmarshal([]byte(got), &doc); err != nil {
		t.Fatalf("SVG is not valid XML: %v\n%s", err, got)
	}
	if !strings.Contains(got, "<title>x &lt; y</title>") {
		t.Fatalf("title not escaped: %s", got)
	}
	if !strings.Contains(got, "<path ") || !strings.Contains(got, "<rect ") {
		t.Fatalf("missing drawing items: %s", got)
	}
}

func TestSVGNil(t *testing.T) {
	t.Parallel()

	if got := SVG(nil, Options{}); got == "" {
		t.Fatal("SVG(nil) is empty")
	}
}
