package main

import (
	"strings"
	"testing"

	"github.com/caelyreth/untie"
	"github.com/egoist/mygo/ui"
)

func TestViewSwitchesDialect(t *testing.T) {
	model, err := newApp(mathFont)
	if err != nil {
		t.Fatal(err)
	}
	tester := ui.NewTester(model.view, 960, 720)
	if !tester.HasText("TeX") || !tester.HasText("Preview") {
		t.Fatalf("texts = %q", tester.Texts())
	}
	if err := tester.Click("Typst"); err != nil {
		t.Fatal(err)
	}
	if model.dialect != dialectTypst {
		t.Fatalf("dialect = %d, want Typst", model.dialect)
	}
	if !strings.Contains(model.typstSource, "sqrt") {
		t.Fatalf("Typst source = %q", model.typstSource)
	}
	if model.box == nil {
		t.Fatalf("Typst render failed: %s", model.message)
	}
}

func TestFitMathIncludesOverflow(t *testing.T) {
	content := ui.Rect{X: 10, Y: 20, W: 100, H: 80}
	box := &untie.Box{
		Width:    200,
		Ascent:   40,
		Descent:  20,
		Overflow: 30,
	}
	placement := fitMath(content, box)
	if placement.scale >= 1 {
		t.Fatalf("scale = %v, want scaled down", placement.scale)
	}
	left := placement.originX - box.Overflow*placement.scale
	right := placement.originX + (box.Width+box.Overflow)*placement.scale
	top := placement.originY - (box.Ascent+box.Overflow)*placement.scale
	bottom := placement.originY + (box.Descent+box.Overflow)*placement.scale
	if left < content.X-1e-4 || right > content.X+content.W+1e-4 {
		t.Fatalf("horizontal fit = [%v,%v], content = [%v,%v]", left, right, content.X, content.X+content.W)
	}
	if top < content.Y-1e-4 || bottom > content.Y+content.H+1e-4 {
		t.Fatalf("vertical fit = [%v,%v], content = [%v,%v]", top, bottom, content.Y, content.Y+content.H)
	}
}
