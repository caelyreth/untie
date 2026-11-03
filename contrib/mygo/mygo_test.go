package mygo

import (
	"context"
	"math"
	"os"
	"strings"
	"sync"
	"testing"

	"github.com/caelyreth/untie"
	"github.com/caelyreth/untie/render"
	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/markdown"
)

var (
	fixtureOnce  sync.Once
	fixtureFonts *untie.FontSet
	fixtureErr   error
)

func testFonts(t *testing.T) *untie.FontSet {
	t.Helper()
	fixtureOnce.Do(func() {
		data, err := os.ReadFile("../../testdata/upstream/fonts/NewCMMath-Book.otf")
		if err != nil {
			fixtureErr = err
			return
		}
		fixtureFonts, fixtureErr = untie.LoadMathFont(data)
	})
	if fixtureErr != nil {
		t.Fatal(fixtureErr)
	}
	return fixtureFonts
}

func newTestRenderer(t *testing.T, options Options) *Renderer {
	t.Helper()
	renderer, err := New(testFonts(t), options)
	if err != nil {
		t.Fatal(err)
	}
	return renderer
}

func TestNewValidatesOptions(t *testing.T) {
	fonts := testFonts(t)
	for _, test := range []struct {
		name    string
		options Options
	}{
		{name: "negative base", options: Options{BaseSize: -1}},
		{name: "NaN display", options: Options{DisplaySize: float32(math.NaN())}},
		{name: "negative padding", options: Options{Draw: DrawOptions{Padding: -1}}},
		{name: "unknown alignment", options: Options{Draw: DrawOptions{Align: render.AlignEnd + 1}}},
		{name: "too many entries", options: Options{CacheEntries: MaxCacheEntries + 1}},
	} {
		t.Run(test.name, func(t *testing.T) {
			if _, err := New(fonts, test.options); err == nil {
				t.Fatal("New accepted invalid options")
			}
		})
	}
	if _, err := New(nil, Options{}); err == nil {
		t.Fatal("New accepted nil fonts")
	}
}

func TestCacheHitsEvictionAndDiagnosticOwnership(t *testing.T) {
	renderer := newTestRenderer(t, Options{CacheEntries: 2})
	ctx := context.Background()

	first, err := renderer.Render(ctx, `x^`, untie.Inline, untie.Style{Size: 16})
	if err != nil {
		t.Fatal(err)
	}
	if len(first.Diagnostics) == 0 {
		t.Fatal("expected recoverable diagnostic")
	}
	original := first.Diagnostics[0].Message
	first.Diagnostics[0].Message = "mutated"

	second, err := renderer.Render(ctx, `x^`, untie.Inline, untie.Style{Size: 16})
	if err != nil {
		t.Fatal(err)
	}
	if second.Box != first.Box {
		t.Fatal("cache hit returned a different immutable box")
	}
	if second.Diagnostics[0].Message != original {
		t.Fatalf("cached diagnostics were mutated: %q", second.Diagnostics[0].Message)
	}

	for _, source := range []string{`y`, `z`} {
		if _, err := renderer.Render(ctx, source, untie.Inline, untie.Style{Size: 16}); err != nil {
			t.Fatal(err)
		}
	}
	stats := renderer.CacheStats()
	if stats.Entries != 2 || stats.Capacity != 2 {
		t.Fatalf("cache = %+v, want 2/2 entries", stats)
	}
	if stats.Hits != 1 || stats.Misses != 3 || stats.Evictions != 1 {
		t.Fatalf("cache activity = %+v", stats)
	}
}

func TestInlineObjectMapsCompleteBounds(t *testing.T) {
	height, depth := inlineExtents(untie.Rect{
		X:      -3,
		Y:      -13,
		Width:  26,
		Height: 21,
	})
	if height != 13 || depth != 8 {
		t.Fatalf("inline extents = height %v depth %v, want 13 and 8", height, depth)
	}
}

func TestInlineMathRejectsLayoutFailure(t *testing.T) {
	renderer := newTestRenderer(t, Options{})
	if _, ok := renderer.InlineMath(`x`, float32(math.NaN())); ok {
		t.Fatal("InlineMath reported success for an invalid size")
	}
}

func TestMarkdownInlineAndDisplayMath(t *testing.T) {
	renderer := newTestRenderer(t, Options{CacheEntries: 8})
	var options markdown.Options
	renderer.BindMarkdown(&options)
	source := "Before $x^2$ after.\n\n$$\n\\frac{1}{2}\n$$\n"
	tester := ui.NewTester(func(c *ui.Context) {
		ui.Column(c).Fill().Padding(20).Children(func() {
			markdown.View(c, source, &options)
		})
	}, 600, 400)
	if !tester.HasText("Before ￼ after.") {
		t.Fatalf("texts = %q", tester.Texts())
	}
	stats := renderer.CacheStats()
	if stats.Entries < 2 || stats.Misses < 2 {
		t.Fatalf("Markdown did not render inline and display math: %+v", stats)
	}
	tester.Frame()
	if got := renderer.CacheStats(); got.Hits <= stats.Hits {
		t.Fatalf("second frame did not reuse cached math: before %+v after %+v", stats, got)
	}
}

func TestDisplayMathBuildsElement(t *testing.T) {
	renderer := newTestRenderer(t, Options{})
	tester := ui.NewTester(func(c *ui.Context) {
		renderer.DisplayMath(c, `\frac{a}{b}`)
	}, 400, 200)
	if stats := renderer.CacheStats(); stats.Entries != 1 || stats.Misses != 1 {
		t.Fatalf("display cache = %+v", stats)
	}
	if tester.FramePending() {
		t.Fatal("display math left a frame pending")
	}
}

func TestDrawUsesThemeRoleColors(t *testing.T) {
	renderer := newTestRenderer(t, Options{
		Draw: DrawOptions{Align: render.AlignCenter},
	})
	box := &untie.Box{
		Width:   40,
		Ascent:  20,
		Descent: 0,
		Items: []untie.Item{{
			Kind:  untie.ItemRule,
			Paint: untie.Paint{Role: untie.RoleInk},
			Rect:  untie.Rect{X: 0, Y: -20, Width: 40, Height: 20},
		}},
	}
	tester := ui.NewTester(func(c *ui.Context) {
		ui.Box(c).Size(100, 100).Draw(func(p *ui.Painter, rect ui.Rect) {
			if err := renderer.Draw(p, box, rect); err != nil {
				t.Errorf("Draw: %v", err)
			}
		})
	}, 100, 100)
	light := tester.Image().RGBAAt(50, 50)
	background := tester.Image().RGBAAt(0, 0)
	if light == background {
		t.Fatalf("ink did not draw in light theme: image pixel = %#v", light)
	}

	tester.SetDark(true)
	tester.Frame()
	dark := tester.Image().RGBAAt(50, 50)
	if dark == light {
		t.Fatalf("theme change did not recolor cached geometry: light %#v dark %#v", light, dark)
	}
}

func TestPaintColorRoles(t *testing.T) {
	theme := ui.DarkTheme()
	for paint, want := range map[untie.Paint]ui.Color{
		{Role: untie.RoleInk}:         theme.Text,
		{Role: untie.RoleError}:       theme.Danger,
		{Role: untie.RolePlaceholder}: theme.TextMuted,
		{
			HasExplicit: true,
			Explicit:    untie.RGBA{R: 10, G: 20, B: 30, A: 128},
		}: ui.RGBA(10, 20, 30, 128.0/255),
	} {
		if got := PaintColor(theme, paint); got != want {
			t.Errorf("PaintColor(%+v) = %#v, want %#v", paint, got, want)
		}
	}
}

func TestConcurrentInlineMath(t *testing.T) {
	renderer := newTestRenderer(t, Options{CacheEntries: 32})
	var wait sync.WaitGroup
	for worker := 0; worker < 16; worker++ {
		worker := worker
		wait.Add(1)
		go func() {
			defer wait.Done()
			for iteration := 0; iteration < 20; iteration++ {
				source := `x^2`
				if (worker+iteration)%2 == 1 {
					source = `\frac{1}{2}`
				}
				if _, ok := renderer.InlineMath(source, 14+(float32(worker%3))); !ok {
					t.Errorf("InlineMath(%q) failed", source)
					return
				}
			}
		}()
	}
	wait.Wait()
	stats := renderer.CacheStats()
	if stats.Entries == 0 || stats.Hits+stats.Misses == 0 {
		t.Fatalf("cache stats = %+v", stats)
	}
	if stats.Evictions != 0 {
		t.Fatalf("small workload unexpectedly evicted entries: %+v", stats)
	}
}

func TestDrawValidation(t *testing.T) {
	renderer := newTestRenderer(t, Options{})
	if err := renderer.Draw(nil, &untie.Box{}, ui.Rect{W: 10, H: 10}); !strings.Contains(err.Error(), "painter") {
		t.Fatalf("nil painter error = %v", err)
	}
	if err := renderer.Draw(&ui.Painter{}, nil, ui.Rect{W: 10, H: 10}); err != ErrInvalidBox {
		t.Fatalf("nil box error = %v", err)
	}
}
