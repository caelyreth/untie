package stream_test

import (
	"testing"
	"time"

	"github.com/caelyreth/untie"
	"github.com/caelyreth/untie/stream"
)

func BenchmarkTransitionWalkFinal(b *testing.B) {
	renderer := newTestRenderer(b)
	before := renderBox(b, renderer, `\frac{a}{b}`)
	after := renderBox(b, renderer, `\frac{a+b}{c}+x^2`)
	started := time.Unix(1100, 0)
	transition, err := stream.NewTransition(
		before,
		after,
		started,
		stream.TransitionOptions{Duration: 180 * time.Millisecond},
	)
	if err != nil {
		b.Fatal(err)
	}
	at := started.Add(180 * time.Millisecond)
	b.ReportAllocs()
	for b.Loop() {
		if err := transition.Walk(at, func(stream.Command) error {
			return nil
		}); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkTransitionWalkMid(b *testing.B) {
	renderer := newTestRenderer(b)
	before := renderBox(b, renderer, `\frac{a}{b}`)
	after := renderBox(b, renderer, `\frac{a+b}{c}+x^2`)
	started := time.Unix(1200, 0)
	transition, err := stream.NewTransition(
		before,
		after,
		started,
		stream.TransitionOptions{Duration: 180 * time.Millisecond},
	)
	if err != nil {
		b.Fatal(err)
	}
	at := started.Add(90 * time.Millisecond)
	b.ReportAllocs()
	for b.Loop() {
		if err := transition.Walk(at, func(stream.Command) error {
			return nil
		}); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkSessionCoalescedUpdate(b *testing.B) {
	renderer := newTestRenderer(b)
	session, err := stream.NewSession(b.Context(), renderer, stream.SessionOptions{
		Request: untie.RenderRequest{
			Mode:  untie.Display,
			Style: untie.Style{Size: 30},
		},
	})
	if err != nil {
		b.Fatal(err)
	}
	now := time.Unix(1300, 0)
	if _, _, err := session.Update(b.Context(), `x`, now); err != nil {
		b.Fatal(err)
	}
	sources := [...]string{`x^2`, `x^2+1`}
	b.ReportAllocs()
	for i := 0; b.Loop(); i++ {
		if _, changed, err := session.Update(
			b.Context(),
			sources[i&1],
			now.Add(time.Millisecond),
		); err != nil {
			b.Fatal(err)
		} else if changed {
			b.Fatal("coalesced update unexpectedly rendered")
		}
	}
}

func TestTransitionWalkDoesNotAllocate(t *testing.T) {
	renderer := newTestRenderer(t)
	before := renderBox(t, renderer, `\frac{a}{b}`)
	after := renderBox(t, renderer, `\frac{a+b}{c}+x^2`)
	started := time.Unix(1400, 0)
	transition, err := stream.NewTransition(
		before,
		after,
		started,
		stream.TransitionOptions{Duration: 180 * time.Millisecond},
	)
	if err != nil {
		t.Fatal(err)
	}
	at := started.Add(90 * time.Millisecond)
	allocs := testing.AllocsPerRun(1000, func() {
		if err := transition.Walk(at, func(stream.Command) error {
			return nil
		}); err != nil {
			panic(err)
		}
	})
	if allocs != 0 {
		t.Fatalf("mid-transition Walk allocations = %v, want 0", allocs)
	}
}
