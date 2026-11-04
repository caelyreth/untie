package main

import (
	"context"
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
	if len(model.mathStreams) != 1 || model.mathStreams[0] == nil {
		t.Fatalf("display math sessions = %v", model.mathStreams)
	}
	tester.Advance(2 * time.Second)
	frame := model.mathStreams[0].Frame()
	if model.shown == 0 || frame == nil {
		t.Fatalf("stream did not reach math: shown=%d frame=%v", model.shown, frame)
	}
	tester.Advance(6 * time.Second)
	if !model.complete || model.streaming {
		t.Fatalf("stream state = complete:%v streaming:%v", model.complete, model.streaming)
	}
	frame = model.mathStreams[0].Frame()
	if frame == nil || frame.Source() != mathTarget() {
		t.Fatalf("final streamed source = %q", frame.Source())
	}
	if !tester.HasText("complete") {
		t.Fatalf("completed texts = %q", tester.Texts())
	}
	if err := tester.Click("Edit source"); err != nil {
		t.Fatal(err)
	}
	if model.page != editorPage || len(model.mathStreams) != 0 {
		t.Fatalf("edit state = page:%v sessions:%d", model.page, len(model.mathStreams))
	}
}

func TestPrepareDisplayMathUsesCompletedTargetStage(t *testing.T) {
	model, err := newApp(mathFont)
	if err != nil {
		t.Fatal(err)
	}
	model.source = "Before\n\n$$\n\\frac{a}{b}\n$$\n\nAfter\n"
	model.prepareDisplayMath()
	if len(model.mathStreams) != 1 || model.mathStreams[0] == nil {
		t.Fatalf("display math sessions = %v", model.mathStreams)
	}
	frame, changed, err := model.mathStreams[0].Update(
		context.Background(),
		`\frac{a`,
		time.Unix(1000, 0),
	)
	if err != nil || !changed {
		t.Fatalf("stream update changed=%v err=%v", changed, err)
	}
	completed, err := model.renderer.Render(
		context.Background(),
		`\frac{a}{b}`,
		untie.Display,
		untie.Style{},
	)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := frame.Stage().Bounds(), completed.Box.Bounds(); got != want {
		t.Fatalf("stage bounds = %+v, want %+v", got, want)
	}
}

func TestPrepareDisplayMathSupportsMultipleBlocks(t *testing.T) {
	model, err := newApp(mathFont)
	if err != nil {
		t.Fatal(err)
	}
	model.source = "$$\nx\n$$\n\n$$\ny\n$$\n"
	model.prepareDisplayMath()
	if len(model.mathStreams) != 2 {
		t.Fatalf("display math sessions = %d, want 2", len(model.mathStreams))
	}
	for i, streamed := range model.mathStreams {
		if streamed == nil {
			t.Fatalf("display math session %d is nil", i)
		}
	}
}

func mathTarget() string {
	return `G_{\mu\nu} + \Lambda g_{\mu\nu}
= \frac{8\pi G}{c^4} T_{\mu\nu}`
}
