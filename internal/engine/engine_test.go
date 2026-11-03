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

func TestLayoutTeXAndTypst(t *testing.T) {
	t.Parallel()

	bytes, err := os.ReadFile("../../testdata/upstream/fonts/NewCMMath-Book.otf")
	if err != nil {
		t.Fatalf("read font: %v", err)
	}
	set, err := font.LoadFontSet(font.FontSetConfig{Math: font.FontFace{Bytes: bytes, ID: "NewCMMath-Book"}})
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	engine, err := New(set, source.Limits{})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	span := source.NewRange(0, 4)
	root := ir.NewRow(span,
		ir.NewScripts(span,
			ir.NewGlyph(source.NewRange(0, 1), "x"),
		),
	)
	root.Items[0].(*ir.Scripts).Top = ir.NewGlyph(source.NewRange(2, 3), "2")
	box, err := engine.Layout(context.Background(), root, ir.Inline, display.Style{Size: 16})
	if err != nil {
		t.Fatalf("Layout() error = %v", err)
	}
	if err := box.Validate(); err != nil {
		t.Fatalf("box.Validate() error = %v", err)
	}
	if box.Width <= 0 || len(box.Items) == 0 {
		t.Fatalf("empty output: %+v", box)
	}
}

func TestParseAndLayoutBothDialects(t *testing.T) {
	t.Parallel()

	bytes, err := os.ReadFile("../../testdata/upstream/fonts/NewCMMath-Book.otf")
	if err != nil {
		t.Fatalf("read font: %v", err)
	}
	set, err := font.LoadFontSet(font.FontSetConfig{Math: font.FontFace{Bytes: bytes, ID: "NewCMMath-Book"}})
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	engine, err := New(set, source.Limits{})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	results := []struct {
		name  string
		root  ir.Node
		diags []source.Diagnostic
	}{
		{name: "tex", root: tex.Parse(`x^2+\frac{1}{2}`, ir.Inline, tex.Options{}).Root, diags: tex.Parse(`x^2+\frac{1}{2}`, ir.Inline, tex.Options{}).Diagnostics},
		{name: "typst", root: typst.Parse(`x^2+1/2`, ir.Inline, typst.Options{}).Root, diags: typst.Parse(`x^2+1/2`, ir.Inline, typst.Options{}).Diagnostics},
	}
	for _, result := range results {
		t.Run(result.name, func(t *testing.T) {
			for _, diagnostic := range result.diags {
				if diagnostic.Severity == source.SeverityError {
					t.Fatalf("unexpected diagnostic: %+v", diagnostic)
				}
			}
			box, err := engine.Layout(context.Background(), result.root, ir.Inline, display.Style{Size: 16})
			if err != nil {
				t.Fatalf("Layout() error = %v", err)
			}
			if box.Width <= 0 || len(box.Items) == 0 {
				t.Fatalf("empty output: %+v", box)
			}
			if err := box.Validate(); err != nil {
				t.Fatalf("Validate() error = %v", err)
			}
		})
	}
}
