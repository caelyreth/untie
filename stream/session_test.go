package stream_test

import (
	"context"
	"testing"
	"time"

	"github.com/caelyreth/untie"
	"github.com/caelyreth/untie/stream"
)

type countingRenderer struct {
	renderer *untie.Renderer
	calls    int
}

func (r *countingRenderer) Render(
	ctx context.Context,
	source string,
	request untie.RenderRequest,
) (*untie.RenderResult, error) {
	r.calls++
	return r.renderer.Render(ctx, source, request)
}

func TestSessionCoalescesBeforeLayout(t *testing.T) {
	t.Parallel()

	renderer := &countingRenderer{renderer: newTestRenderer(t)}
	session, err := stream.NewSession(t.Context(), renderer, stream.SessionOptions{
		Request: untie.RenderRequest{
			Mode:  untie.Display,
			Style: untie.Style{Size: 30},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	now := time.Unix(500, 0)
	frame, changed, err := session.Update(t.Context(), `x`, now)
	if err != nil || !changed || frame == nil {
		t.Fatalf("first update: frame=%v changed=%v err=%v", frame, changed, err)
	}
	if renderer.calls != 1 {
		t.Fatalf("renderer calls = %d, want 1", renderer.calls)
	}
	if _, changed, err := session.Update(t.Context(), `x^2`, now.Add(20*time.Millisecond)); err != nil || changed {
		t.Fatalf("second update changed=%v err=%v", changed, err)
	}
	if _, changed, err := session.Update(t.Context(), `x^2+1`, now.Add(40*time.Millisecond)); err != nil || changed {
		t.Fatalf("third update changed=%v err=%v", changed, err)
	}
	if renderer.calls != 1 || !session.Pending() {
		t.Fatalf("coalesced calls=%d pending=%v", renderer.calls, session.Pending())
	}
	delay, ok := session.NextCommitIn(now.Add(40 * time.Millisecond))
	if !ok || delay != 140*time.Millisecond {
		t.Fatalf("next commit delay = %v, %v, want 140ms", delay, ok)
	}
	frame, changed, err = session.Update(
		t.Context(),
		`x^2+1`,
		now.Add(stream.DefaultCommitInterval),
	)
	if err != nil || !changed {
		t.Fatalf("commit: changed=%v err=%v", changed, err)
	}
	if renderer.calls != 2 || frame.Source() != `x^2+1` {
		t.Fatalf("commit calls=%d source=%q", renderer.calls, frame.Source())
	}
	if frame.Previous() == nil {
		t.Fatal("committed frame has no previous box")
	}
	if _, ok := session.NextCommitIn(now.Add(stream.DefaultCommitInterval)); ok {
		t.Fatal("next commit remains scheduled after commit")
	}
}

func TestSessionUsesCompletedTargetStage(t *testing.T) {
	t.Parallel()

	renderer := &countingRenderer{renderer: newTestRenderer(t)}
	target := `\frac{a+b}{c}`
	session, err := stream.NewSession(t.Context(), renderer, stream.SessionOptions{
		Request: untie.RenderRequest{
			Mode:  untie.Display,
			Style: untie.Style{Size: 30},
		},
		TargetSource: target,
	})
	if err != nil {
		t.Fatal(err)
	}
	if renderer.calls != 1 {
		t.Fatalf("target calls = %d, want 1", renderer.calls)
	}
	frame, _, err := session.Update(t.Context(), `\frac{a`, time.Unix(600, 0))
	if err != nil {
		t.Fatal(err)
	}
	want := renderBox(t, renderer.renderer, target).Bounds()
	if got := frame.Stage().Bounds(); got != want {
		t.Fatalf("stage bounds = %+v, want %+v", got, want)
	}
	if frame.Previous() != nil {
		t.Fatal("first frame has a previous box")
	}
}

func TestSessionDoesNotRenderIdenticalSource(t *testing.T) {
	t.Parallel()

	renderer := &countingRenderer{renderer: newTestRenderer(t)}
	session, err := stream.NewSession(t.Context(), renderer, stream.SessionOptions{
		Request: untie.RenderRequest{Mode: untie.Display},
	})
	if err != nil {
		t.Fatal(err)
	}
	now := time.Unix(700, 0)
	if _, _, err := session.Update(t.Context(), `x`, now); err != nil {
		t.Fatal(err)
	}
	if _, changed, err := session.Update(t.Context(), `x`, now.Add(time.Second)); err != nil || changed {
		t.Fatalf("identical update changed=%v err=%v", changed, err)
	}
	if renderer.calls != 1 {
		t.Fatalf("renderer calls = %d, want 1", renderer.calls)
	}
}

func TestSessionUpdatesAfterCommitInterval(t *testing.T) {
	t.Parallel()

	renderer := &countingRenderer{renderer: newTestRenderer(t)}
	session, err := stream.NewSession(t.Context(), renderer, stream.SessionOptions{
		Request: untie.RenderRequest{Mode: untie.Display},
	})
	if err != nil {
		t.Fatal(err)
	}
	now := time.Unix(800, 0)
	if _, _, err := session.Update(t.Context(), `x`, now); err != nil {
		t.Fatal(err)
	}
	if _, changed, err := session.Update(
		t.Context(),
		`x+y`,
		now.Add(stream.DefaultCommitInterval),
	); err != nil || !changed {
		t.Fatalf("post-interval update changed=%v err=%v", changed, err)
	}
	if renderer.calls != 2 {
		t.Fatalf("renderer calls = %d, want 2", renderer.calls)
	}
}
