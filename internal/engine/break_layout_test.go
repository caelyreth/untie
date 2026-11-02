package engine

import (
	"context"
	"os"
	"testing"

	"github.com/caelyreth/untie/display"
	"github.com/caelyreth/untie/font"
	"github.com/caelyreth/untie/internal/tex"
	"github.com/caelyreth/untie/internal/typst"
	"github.com/caelyreth/untie/ir"
	"github.com/caelyreth/untie/source"
)

func TestBreakMetadataEquivalentAcrossFrontends(t *testing.T) {
	t.Parallel()

	bytes, err := os.ReadFile("../../testdata/upstream/fonts/NewCMMath-Book.otf")
	if err != nil {
		t.Fatalf("read font: %v", err)
	}
	set, err := font.Load(font.Fonts{Math: font.FontData{Bytes: bytes, ID: "NewCMMath-Book"}})
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	engine, err := New(set, source.Limits{})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	tests := []struct {
		name string
		tex  string
		typ  string
		kind display.BreakKind
	}{
		{name: "relation", tex: `x=y`, typ: `x = y`, kind: display.BreakRelation},
		{name: "binary", tex: `x+y`, typ: `x + y`, kind: display.BreakBinary},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			texBox := layoutParsedForBreakTest(t, engine, tex.Parse(test.tex, ir.Inline, tex.Options{}), "tex")
			typstBox := layoutParsedForBreakTest(t, engine, typst.Parse(test.typ, ir.Inline, typst.Options{}), "typst")

			for _, result := range []struct {
				name string
				box  *display.Box
			}{
				{name: "tex", box: texBox},
				{name: "typst", box: typstBox},
			} {
				if len(result.box.Breaks) != 1 {
					t.Fatalf("%s Breaks = %+v, want one", result.name, result.box.Breaks)
				}
				if got := result.box.Breaks[0]; got.Kind != test.kind {
					t.Fatalf("%s break = %+v, want kind %v", result.name, got, test.kind)
				}
			}
			if texBox.Breaks[0] != typstBox.Breaks[0] {
				t.Fatalf("TeX break %+v != Typst break %+v", texBox.Breaks[0], typstBox.Breaks[0])
			}
		})
	}
}

func TestTeXBreakMarkersAndGroups(t *testing.T) {
	t.Parallel()

	bytes, err := os.ReadFile("../../testdata/upstream/fonts/NewCMMath-Book.otf")
	if err != nil {
		t.Fatalf("read font: %v", err)
	}
	set, err := font.Load(font.Fonts{Math: font.FontData{Bytes: bytes, ID: "NewCMMath-Book"}})
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	engine, err := New(set, source.Limits{})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	box := layoutParsedForBreakTest(t, engine, tex.Parse(`x\allowbreak y`, ir.Inline, tex.Options{}), "tex")
	if len(box.Breaks) != 1 {
		t.Fatalf("Breaks = %+v, want one", box.Breaks)
	}
	if got := box.Breaks[0]; got.Kind != display.BreakExplicit {
		t.Fatalf("allowbreak = %+v, want explicit", got)
	}

	grouped := layoutParsedForBreakTest(t, engine, tex.Parse(`{x=y}`, ir.Inline, tex.Options{}), "tex")
	if len(grouped.Breaks) != 0 {
		t.Fatalf("grouped Breaks = %+v, want no leakage", grouped.Breaks)
	}
}

func layoutParsedForBreakTest(t *testing.T, engine *Engine, parsed any, name string) *display.Box {
	t.Helper()
	var root ir.Node
	var diagnostics []source.Diagnostic
	switch result := parsed.(type) {
	case tex.Result:
		root, diagnostics = result.Root, result.Diagnostics
	case typst.Result:
		root, diagnostics = result.Root, result.Diagnostics
	default:
		t.Fatalf("%s result type = %T", name, parsed)
	}
	for _, diagnostic := range diagnostics {
		if diagnostic.Severity == source.SeverityError {
			t.Fatalf("%s diagnostic = %+v", name, diagnostic)
		}
	}
	box, err := engine.Layout(context.Background(), root, ir.Inline, display.Style{Size: 16})
	if err != nil {
		t.Fatalf("%s Layout() error = %v", name, err)
	}
	return box
}
