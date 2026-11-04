package untiemygo

import (
	"testing"
	"time"

	"github.com/caelyreth/untie"
	"github.com/egoist/mygo/ui"
)

func TestStreamingMathReservesCompletedStageAndDraws(t *testing.T) {
	renderer := newTestRenderer(t, Options{
		DisplaySize: 30,
		Draw:        DrawOptions{Padding: 8},
	})
	target := `\frac{a+b}{c}`
	streamed, err := renderer.NewStreamingMath(t.Context(), StreamingMathOptions{
		TargetSource:      target,
		AnimationDuration: 100 * time.Millisecond,
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

	tester := ui.NewTester(func(c *ui.Context) {
		streamed.Element(c, `\frac{a`)
	}, 400, 180)
	tester.Advance(200 * time.Millisecond)
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
		TargetSource:      `x^2`,
		AnimationDuration: 100 * time.Millisecond,
	})
	if err != nil {
		t.Fatal(err)
	}
	started := time.Now()
	tester := ui.NewTester(func(c *ui.Context) {
		// Keep the build timestamp fixed. MyGo may rebuild the view, but the
		// transition must still advance from the painter's frame clock.
		streamed.element(c, `x`, started)
	}, 240, 120)
	tester.Advance(300 * time.Millisecond)
	if tester.FramePending() {
		t.Fatal("entrance animation froze on its build timestamp")
	}
}

func TestStreamingMathSchedulesPendingSourceCommit(t *testing.T) {
	renderer := newTestRenderer(t, Options{DisplaySize: 30})
	streamed, err := renderer.NewStreamingMath(t.Context(), StreamingMathOptions{
		TargetSource:      `x^2`,
		AnimationDuration: 100 * time.Millisecond,
	})
	if err != nil {
		t.Fatal(err)
	}
	source := `x`
	tester := ui.NewTester(func(c *ui.Context) {
		streamed.Element(c, source)
	}, 240, 120)

	// Offer newer source inside the active transition. Painter frames animate,
	// while Context.After must schedule the full rebuild that commits it.
	source = `x^2`
	tester.Frame()
	if frame := streamed.Frame(); frame == nil || frame.Source() != `x` {
		t.Fatalf("prematurely committed frame = %v", frame)
	}
	tester.Advance(300 * time.Millisecond)
	frame := streamed.Frame()
	if frame == nil || frame.Source() != `x^2` {
		t.Fatalf("pending source did not commit: %v", frame)
	}
	if tester.FramePending() {
		t.Fatal("stream did not settle after committing pending source")
	}
}

func TestStreamTimingCurvesAreMonotonicAndNonLinear(t *testing.T) {
	t.Parallel()

	for _, timing := range []StreamTiming{StreamEaseOutCubic, StreamEaseInOutCubic} {
		if got := timing.apply(0); got != 0 {
			t.Fatalf("timing %d at zero = %v", timing, got)
		}
		if got := timing.apply(1); got != 1 {
			t.Fatalf("timing %d at one = %v", timing, got)
		}
		previous := float32(0)
		for i := 1; i <= 100; i++ {
			value := timing.apply(float32(i) / 100)
			if value < previous {
				t.Fatalf("timing %d is not monotonic at %d: %v < %v",
					timing, i, value, previous)
			}
			previous = value
		}
		if timing.apply(0.25) == 0.25 {
			t.Fatalf("timing %d is linear", timing)
		}
	}
	if StreamEaseOutCubic.apply(0.25) <= StreamEaseInOutCubic.apply(0.25) {
		t.Fatal("ease-out does not lead ease-in-out early in the transition")
	}
}

func TestNewStreamingMathValidatesAnimationOptions(t *testing.T) {
	renderer := newTestRenderer(t, Options{})
	for _, options := range []StreamingMathOptions{
		{AnimationDuration: -time.Second},
		{Timing: StreamTiming(255)},
		{AppearanceScale: -1},
		{AppearanceScale: 2},
	} {
		if _, err := renderer.NewStreamingMath(t.Context(), options); err == nil {
			t.Fatalf("invalid animation options accepted: %+v", options)
		}
	}
}
