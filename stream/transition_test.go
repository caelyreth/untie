package stream_test

import (
	"os"
	"testing"
	"time"

	"github.com/caelyreth/untie"
	"github.com/caelyreth/untie/render"
	"github.com/caelyreth/untie/stream"
)

func TestTransitionFinalFrameMatchesCurrentBox(t *testing.T) {
	t.Parallel()

	renderer := newTestRenderer(t)
	before := renderBox(t, renderer, `\frac{a}{b}`)
	after := renderBox(t, renderer, `\frac{a}{b}+x`)
	started := time.Unix(100, 0)
	transition, err := stream.NewTransition(
		before,
		after,
		started,
		stream.TransitionOptions{Duration: 200 * time.Millisecond},
	)
	if err != nil {
		t.Fatal(err)
	}
	var got []stream.Command
	if err := transition.Walk(
		started.Add(200*time.Millisecond),
		func(command stream.Command) error {
			got = append(got, command)
			return nil
		},
	); err != nil {
		t.Fatal(err)
	}
	var want []render.Command
	if err := render.Walk(after, func(command render.Command) error {
		want = append(want, command)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if len(got) != len(want) {
		t.Fatalf("final commands = %d, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i].Opacity != 1 {
			t.Fatalf("command %d opacity = %v", i, got[i].Opacity)
		}
		if int(got[i].Kind) != int(want[i].Kind) ||
			got[i].Paint != want[i].Paint ||
			got[i].At != want[i].At ||
			got[i].Em != want[i].Em ||
			got[i].Rect != want[i].Rect ||
			got[i].Snap != want[i].Snap ||
			!sameTemplate(got[i].Path, want[i].Path) {
			t.Fatalf("command %d = %+v, want %+v", i, got[i], want[i])
		}
	}
}

func TestTransitionMovesSharedGlyphByTransform(t *testing.T) {
	t.Parallel()

	renderer := newTestRenderer(t)
	before := renderBox(t, renderer, `x`)
	after := renderBox(t, renderer, `x+y`)
	started := time.Unix(200, 0)
	transition, err := stream.NewTransition(
		before,
		after,
		started,
		stream.TransitionOptions{Duration: 200 * time.Millisecond},
	)
	if err != nil {
		t.Fatal(err)
	}
	var moved, entered bool
	if err := transition.Walk(
		started.Add(100*time.Millisecond),
		func(command stream.Command) error {
			if command.Kind != stream.Path {
				return nil
			}
			if command.Opacity == 1 && command.Em > 0 {
				moved = true
			}
			if command.Opacity > 0 && command.Opacity < 1 {
				entered = true
			}
			return nil
		},
	); err != nil {
		t.Fatal(err)
	}
	if !moved || !entered {
		t.Fatalf("moved=%v entered=%v, want shared and entering paths", moved, entered)
	}
}

func TestTransitionEnteringPathFadesAndScales(t *testing.T) {
	t.Parallel()

	renderer := newTestRenderer(t)
	after := renderBox(t, renderer, `x`)
	started := time.Unix(300, 0)
	transition, err := stream.NewTransition(
		nil,
		after,
		started,
		stream.TransitionOptions{
			Duration:             200 * time.Millisecond,
			AppearanceStartScale: 0.5,
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	var command stream.Command
	found := false
	if err := transition.Walk(
		started.Add(100*time.Millisecond),
		func(next stream.Command) error {
			command, found = next, true
			return nil
		},
	); err != nil {
		t.Fatal(err)
	}
	if !found {
		t.Fatal("transition emitted no entering command")
	}
	if command.Opacity <= 0 || command.Opacity >= 1 {
		t.Fatalf("opacity = %v, want partial fade", command.Opacity)
	}
	if command.Em <= 0 || command.Em >= after.Items[0].Em {
		t.Fatalf("entering em = %v, final = %v", command.Em, after.Items[0].Em)
	}
}

func TestTransitionInterpolatesRuleBounds(t *testing.T) {
	t.Parallel()

	renderer := newTestRenderer(t)
	before := renderBox(t, renderer, `\frac{a}{b}`)
	after := renderBox(t, renderer, `\frac{a+b}{c}`)
	started := time.Unix(400, 0)
	transition, err := stream.NewTransition(
		before,
		after,
		started,
		stream.TransitionOptions{Duration: 200 * time.Millisecond},
	)
	if err != nil {
		t.Fatal(err)
	}
	var rule *stream.Command
	if err := transition.Walk(
		started.Add(100*time.Millisecond),
		func(command stream.Command) error {
			if command.Kind == stream.Rule && command.Opacity == 1 {
				copy := command
				rule = &copy
			}
			return nil
		},
	); err != nil {
		t.Fatal(err)
	}
	if rule == nil || rule.Rect.Width <= 0 {
		t.Fatalf("interpolated rule = %+v", rule)
	}
}

func TestNewTransitionRejectsInvalidOptions(t *testing.T) {
	t.Parallel()

	renderer := newTestRenderer(t)
	box := renderBox(t, renderer, `x`)
	if _, err := stream.NewTransition(nil, nil, time.Time{}, stream.TransitionOptions{}); err == nil {
		t.Fatal("nil current box accepted")
	}
	if _, err := stream.NewTransition(nil, box, time.Time{}, stream.TransitionOptions{
		Duration: -time.Second,
	}); err == nil {
		t.Fatal("negative duration accepted")
	}
	if _, err := stream.NewTransition(nil, box, time.Time{}, stream.TransitionOptions{
		AppearanceStartScale: 2,
	}); err == nil {
		t.Fatal("appearance scale above one accepted")
	}
}

func newTestRenderer(t *testing.T) *untie.Renderer {
	t.Helper()
	data, err := os.ReadFile("../testdata/upstream/fonts/NewCMMath-Book.otf")
	if err != nil {
		t.Fatal(err)
	}
	fonts, err := untie.LoadMathFont(data)
	if err != nil {
		t.Fatal(err)
	}
	renderer, err := untie.NewRenderer(fonts, untie.RendererOptions{
		DefaultStyle: untie.Style{Size: 30},
		CacheEntries: 64,
	})
	if err != nil {
		t.Fatal(err)
	}
	return renderer
}

func renderBox(t *testing.T, renderer *untie.Renderer, source string) *untie.Box {
	t.Helper()
	result, err := renderer.Render(t.Context(), source, untie.RenderRequest{
		Mode:  untie.Display,
		Style: untie.Style{Size: 30},
	})
	if err != nil {
		t.Fatal(err)
	}
	if result == nil || result.Box == nil {
		t.Fatalf("render %q produced no box", source)
	}
	return result.Box
}

func sameTemplate(a, b untie.PathTemplate) bool {
	if len(a.Segments) != len(b.Segments) {
		return false
	}
	for i := range a.Segments {
		if a.Segments[i] != b.Segments[i] {
			return false
		}
	}
	return true
}
