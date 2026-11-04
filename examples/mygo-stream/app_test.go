package main

import (
	"testing"
	"time"

	"github.com/caelyreth/untie"
	"github.com/egoist/mygo/ui"
)

func TestStreamRendersMarkdownAndMath(t *testing.T) {
	model, err := newApp(mathFont)
	if err != nil {
		t.Fatal(err)
	}
	tester := ui.NewTester(model.view, 980, 760)
	tester.Frame()
	if !tester.HasText("Streaming Markdown") || !tester.HasText("Render stream") {
		t.Fatalf("initial texts = %q", tester.Texts())
	}
	if err := tester.Click("Render stream"); err != nil {
		t.Fatal(err)
	}
	if model.page != renderPage {
		t.Fatal("render button did not open the render page")
	}
	tester.Advance(2 * time.Second)
	if model.shown == 0 || model.morph.current == nil {
		t.Fatalf("stream did not reach math: shown=%d current=%v", model.shown, model.morph.current != nil)
	}
	tester.Advance(6 * time.Second)
	if !model.complete || model.streaming {
		t.Fatalf("stream state = complete:%v streaming:%v", model.complete, model.streaming)
	}
	if !tester.HasText("complete") {
		t.Fatalf("completed texts = %q", tester.Texts())
	}
	if err := tester.Click("Edit source"); err != nil {
		t.Fatal(err)
	}
	if model.page != editorPage {
		t.Fatal("edit button did not return to the editor page")
	}
}

func TestFormulaMorphKeepsPreviousBox(t *testing.T) {
	t.Parallel()

	first := &untie.Box{Width: 20, Ascent: 10}
	second := &untie.Box{Width: 40, Ascent: 12}
	now := time.Now()
	var morph formulaMorph
	morph.update(first, `x`, now)
	morph.update(second, `x^2`, now.Add(16*time.Millisecond))
	if morph.previous != first || morph.current != second {
		t.Fatalf("morph boxes = previous:%p current:%p", morph.previous, morph.current)
	}
	if morph.source != `x^2` || morph.started.IsZero() {
		t.Fatalf("morph state = %+v", morph)
	}
}
