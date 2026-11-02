package main

import (
	"strings"
	"testing"

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
