package untiemygo

import (
	"testing"
	"time"

	"github.com/caelyreth/untie"
	"github.com/caelyreth/untie/stream"
	"github.com/egoist/mygo/ui"
)

func TestStreamingMathReservesCompletedStageAndDraws(t *testing.T) {
	renderer := newTestRenderer(t, Options{
		DisplaySize: 30,
		Draw:        DrawOptions{Padding: 8},
	})
	target := `\frac{a+b}{c}`
	streamed, err := renderer.NewStreamingMath(t.Context(), StreamingMathOptions{
		TargetSource: target,
		Transition: stream.TransitionOptions{
			Duration: 100 * time.Millisecond,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	started := time.Unix(900, 0)
	frame, changed, err := streamed.Update(t.Context(), `\frac{a`, started)
	if err != nil || !changed {
		t.Fatalf("update changed=%v err=%v", changed, err)
	}
	completed, err := renderer.Render(t.Context(), target, untie.Display, untie.Style{})
	if err != nil {
		t.Fatal(err)
	}
	if got, want := frame.Stage().Bounds(), completed.Box.Bounds(); got != want {
		t.Fatalf("stage bounds = %+v, want %+v", got, want)
	}

	final := started.Add(100 * time.Millisecond)
	tester := ui.NewTester(func(c *ui.Context) {
		streamed.Element(c, `\frac{a`, final)
	}, 400, 180)
	tester.Frame()
	if tester.FramePending() {
		t.Fatal("completed stream frame still requests animation")
	}
	image := tester.Image()
	background := image.RGBAAt(0, 0)
	painted := false
	for y := 0; y < image.Bounds().Dy() && !painted; y++ {
		for x := 0; x < image.Bounds().Dx(); x++ {
			if image.RGBAAt(x, y) != background {
				painted = true
				break
			}
		}
	}
	if !painted {
		t.Fatal("streaming math drew no ink")
	}
}

func TestNewStreamingMathValidatesRenderer(t *testing.T) {
	var renderer *Renderer
	if _, err := renderer.NewStreamingMath(t.Context(), StreamingMathOptions{}); err == nil {
		t.Fatal("nil renderer accepted")
	}
}

func TestStreamingMathAdvancesDuringPainterRepaint(t *testing.T) {
	renderer := newTestRenderer(t, Options{DisplaySize: 30})
	streamed, err := renderer.NewStreamingMath(t.Context(), StreamingMathOptions{
		TargetSource: `x^2`,
		Transition: stream.TransitionOptions{
			Duration: 100 * time.Millisecond,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	started := time.Now()
	tester := ui.NewTester(func(c *ui.Context) {
		// Keep the build timestamp fixed. MyGo may rebuild the view, but the
		// transition must still advance from the painter's frame clock.
		streamed.Element(c, `x`, started)
	}, 240, 120)
	tester.Advance(200 * time.Millisecond)
	if tester.FramePending() {
		t.Fatal("entrance animation froze on its build timestamp")
	}
}
