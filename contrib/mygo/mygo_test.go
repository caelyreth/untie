package untiemygo

import (
	"context"
	"errors"
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
	if len(first.Diagnostics[0].Args) > 0 {
		first.Diagnostics[0].Args[0] = "MUTATED_ARG"
	}
	if len(first.Diagnostics[0].Hints) > 0 {
		first.Diagnostics[0].Hints[0] = "MUTATED_HINT"
	}

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
	if (len(second.Diagnostics[0].Args) > 0 && second.Diagnostics[0].Args[0] == "MUTATED_ARG") ||
		(len(second.Diagnostics[0].Hints) > 0 && second.Diagnostics[0].Hints[0] == "MUTATED_HINT") {
		t.Fatalf("cached diagnostic slices were mutated: %+v", second.Diagnostics[0])
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

func TestCanceledRenderDoesNotUseCache(t *testing.T) {
	renderer := newTestRenderer(t, Options{})
	ctx := context.Background()
	if _, err := renderer.Render(ctx, `x`, untie.Inline, untie.Style{Size: 16}); err != nil {
		t.Fatal(err)
	}
	stats := renderer.CacheStats()

	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := renderer.Render(canceled, `x`, untie.Inline, untie.Style{Size: 16}); !errors.Is(err, context.Canceled) {
		t.Fatalf("Render(canceled) error = %v, want context.Canceled", err)
	}
	if _, err := renderer.Render(canceled, `x`, untie.Mode(255), untie.Style{Size: 16}); !errors.Is(err, context.Canceled) {
		t.Fatalf("Render(canceled, invalid mode) error = %v, want context.Canceled", err)
	}
	if got := renderer.CacheStats(); got != stats {
		t.Fatalf("canceled cache lookup changed stats: before %+v after %+v", stats, got)
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

func TestElementBuildsStandaloneDrawable(t *testing.T) {
	renderer := newTestRenderer(t, Options{
		Draw: DrawOptions{Padding: 6, Align: render.AlignCenter},
	})
	tester := ui.NewTester(func(c *ui.Context) {
		ui.Column(c).Fill().Padding(12).Children(func() {
			renderer.Element(c, `\int_0^1 x^2\,dx`, untie.Display).FillWidth()
		})
	}, 360, 180)
	stats := renderer.CacheStats()
	if stats.Entries != 1 || stats.Misses != 1 {
		t.Fatalf("standalone element cache = %+v", stats)
	}
	if tester.FramePending() {
		t.Fatal("standalone element left a frame pending")
	}
	tester.Frame()
	if got := renderer.CacheStats(); got.Hits <= stats.Hits {
		t.Fatalf("second frame did not reuse cached element: before %+v after %+v", stats, got)
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
		t.Fatal("standalone element painted no visible content")
	}
}

func TestElementUsesSourceFallback(t *testing.T) {
	renderer := newTestRenderer(t, Options{})
	tester := ui.NewTester(func(c *ui.Context) {
		renderer.Element(c, `\frac{1}{`, untie.Mode(99))
	}, 240, 120)
	if !tester.HasText(`\frac{1}{`) {
		t.Fatalf("fallback texts = %q", tester.Texts())
	}
}

func TestMissingGlyphKeepsPartialFormula(t *testing.T) {
	renderer := newTestRenderer(t, Options{})
	const source = `x+\text{中}+y`
	result, err := renderer.Render(
		context.Background(),
		source,
		untie.Display,
		untie.Style{Size: 24},
	)
	if err != nil {
		t.Fatalf("Render() error = %v, want visible recovery", err)
	}
	if result.Box == nil || len(result.Box.Items) == 0 {
		t.Fatal("Render() returned no visible partial box")
	}
	if len(result.Diagnostics) == 0 ||
		result.Diagnostics[0].Code != untie.CodeMissingGlyph {
		t.Fatalf("diagnostics = %+v, want missing-glyph", result.Diagnostics)
	}
	if inline, ok := renderer.InlineMath(source, 16); !ok || inline.Width <= 0 {
		t.Fatalf("InlineMath() = %+v, %v, want visible partial object", inline, ok)
	}

	before := renderer.CacheStats()
	tester := ui.NewTester(func(c *ui.Context) {
		renderer.Element(c, source, untie.Display)
	}, 320, 120)
	if tester.FramePending() {
		t.Fatal("Element() left a frame pending")
	}
	if after := renderer.CacheStats(); after.Hits <= before.Hits {
		t.Fatalf("Element() did not reuse the recovered partial box: before %+v after %+v", before, after)
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

func TestDrawBoxPaintPolicies(t *testing.T) {
	explicit := untie.Paint{
		HasExplicit: true,
		Explicit:    untie.RGBA{R: 10, G: 20, B: 30, A: 255},
	}
	box := &untie.Box{
		Width:   40,
		Ascent:  20,
		Descent: 0,
		Items: []untie.Item{
			{
				Kind:  untie.ItemRule,
				Paint: untie.Paint{Role: untie.RoleInk},
				Rect:  untie.Rect{X: 0, Y: -20, Width: 20, Height: 20},
			},
			{
				Kind:  untie.ItemRule,
				Paint: explicit,
				Rect:  untie.Rect{X: 20, Y: -20, Width: 20, Height: 20},
			},
		},
	}

	t.Run("default theme resolver preserves explicit colors", func(t *testing.T) {
		tester := ui.NewTester(func(c *ui.Context) {
			ui.Box(c).Size(40, 20).Draw(func(painter *ui.Painter, rect ui.Rect) {
				if err := DrawBox(painter, box, rect, DrawOptions{Scale: ScaleNatural}); err != nil {
					t.Errorf("DrawBox: %v", err)
				}
			})
		}, 40, 20)
		lightSemantic := tester.Image().RGBAAt(10, 10)
		lightExplicit := tester.Image().RGBAAt(30, 10)
		if lightExplicit.R != 10 || lightExplicit.G != 20 || lightExplicit.B != 30 {
			t.Fatalf("light explicit pixel = %#v", lightExplicit)
		}

		tester.SetDark(true)
		darkSemantic := tester.Image().RGBAAt(10, 10)
		darkExplicit := tester.Image().RGBAAt(30, 10)
		if darkSemantic == lightSemantic {
			t.Fatalf("semantic role did not follow theme: light %#v dark %#v",
				lightSemantic, darkSemantic)
		}
		if darkExplicit != lightExplicit {
			t.Fatalf("explicit color changed with theme: light %#v dark %#v",
				lightExplicit, darkExplicit)
		}
	})

	t.Run("custom resolver can remap semantic and explicit paints", func(t *testing.T) {
		var seen []untie.Paint
		resolver := func(_ *ui.Painter, paint untie.Paint) ui.Color {
			seen = append(seen, paint)
			if paint.HasExplicit {
				return ui.RGB(1, 2, 3)
			}
			return ui.RGB(4, 5, 6)
		}
		tester := ui.NewTester(func(c *ui.Context) {
			ui.Box(c).Size(40, 20).Draw(func(painter *ui.Painter, rect ui.Rect) {
				err := DrawBox(painter, box, rect, DrawOptions{
					Scale: ScaleNatural,
					Paint: resolver,
				})
				if err != nil {
					t.Errorf("DrawBox: %v", err)
				}
			})
		}, 40, 20)
		semanticPixel := tester.Image().RGBAAt(10, 10)
		explicitPixel := tester.Image().RGBAAt(30, 10)
		if semanticPixel.R != 4 || semanticPixel.G != 5 || semanticPixel.B != 6 {
			t.Fatalf("custom semantic pixel = %#v", semanticPixel)
		}
		if explicitPixel.R != 1 || explicitPixel.G != 2 || explicitPixel.B != 3 {
			t.Fatalf("custom explicit pixel = %#v", explicitPixel)
		}
		seenSemantic, seenExplicit := false, false
		for _, paint := range seen {
			if paint.HasExplicit {
				seenExplicit = paint.Explicit == explicit.Explicit
				continue
			}
			seenSemantic = paint.Role == untie.RoleInk
		}
		if !seenSemantic || !seenExplicit {
			t.Fatalf("resolver paints = %+v", seen)
		}
	})
}

func TestDrawBoxScaleModes(t *testing.T) {
	box := &untie.Box{
		Width:  100,
		Ascent: 20,
		Items: []untie.Item{{
			Kind: untie.ItemRule,
			Paint: untie.Paint{
				HasExplicit: true,
				Explicit:    untie.RGBA{R: 255, A: 255},
			},
			Rect: untie.Rect{X: 0, Y: -20, Width: 50, Height: 20},
		}},
	}
	drawPixel := func(scale ScaleMode) uint8 {
		t.Helper()
		tester := ui.NewTester(func(c *ui.Context) {
			ui.Box(c).Size(50, 20).Draw(func(painter *ui.Painter, rect ui.Rect) {
				err := DrawBox(painter, box, rect, DrawOptions{
					Align: render.AlignStart,
					Scale: scale,
				})
				if err != nil {
					t.Errorf("DrawBox: %v", err)
				}
			})
		}, 80, 40)
		background := tester.Image().RGBAAt(70, 30)
		pixel := tester.Image().RGBAAt(40, 10)
		if pixel == background {
			return 0
		}
		return pixel.R
	}
	if got := drawPixel(ScaleFit); got != 0 {
		t.Fatalf("ScaleFit painted scaled-down rule at x=40, red=%d", got)
	}
	if got := drawPixel(ScaleNatural); got == 0 {
		t.Fatal("ScaleNatural did not preserve native rule width")
	}
}

func TestDrawBoxDoesNotMutateCachedBox(t *testing.T) {
	renderer := newTestRenderer(t, Options{CacheEntries: 4})
	ctx := context.Background()
	first, err := renderer.Render(ctx, `x^2+y`, untie.Display, untie.Style{Size: 24})
	if err != nil {
		t.Fatal(err)
	}
	second, err := renderer.Render(ctx, `x^2+y`, untie.Display, untie.Style{Size: 24})
	if err != nil {
		t.Fatal(err)
	}
	if second.Box != first.Box {
		t.Fatal("cache hit returned a different box")
	}
	before, err := first.Box.MarshalCanonical()
	if err != nil {
		t.Fatal(err)
	}
	calls := 0
	tester := ui.NewTester(func(c *ui.Context) {
		ui.Box(c).Size(160, 80).Draw(func(painter *ui.Painter, rect ui.Rect) {
			err := DrawBox(painter, first.Box, rect, DrawOptions{
				Paint: func(_ *ui.Painter, paint untie.Paint) ui.Color {
					calls++
					return PaintColor(ui.LightTheme(), paint)
				},
			})
			if err != nil {
				t.Errorf("DrawBox: %v", err)
			}
		})
	}, 160, 80)
	tester.Frame()
	if calls == 0 {
		t.Fatal("custom resolver was not called")
	}
	after, err := first.Box.MarshalCanonical()
	if err != nil {
		t.Fatal(err)
	}
	if string(before) != string(after) {
		t.Fatal("DrawBox mutated the cached box")
	}
}

func TestDisplayMathWidthWrapsAtRequestedWidth(t *testing.T) {
	const (
		width   = 68
		padding = 4
		frame   = width - 2*padding
		source  = `a+b+c+d`
	)
	renderer := newTestRenderer(t, Options{
		DisplaySize: 24,
		Draw: DrawOptions{
			Padding: padding,
			Align:   render.AlignCenter,
		},
	})
	tester := ui.NewTester(func(c *ui.Context) {
		renderer.DisplayMathWidth(c, source, width)
	}, width, 400)
	rect, ok := tester.Find(source)
	if !ok {
		t.Fatalf("width-aware element not found in %q", tester.Texts())
	}
	if rect.W != width {
		t.Fatalf("element width = %v, want %v", rect.W, width)
	}

	stats := renderer.CacheStats()
	result, err := renderer.Render(context.Background(), source, untie.Display, untie.Style{
		Size:     24,
		MaxWidth: frame,
		Align:    untie.AlignCenter,
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := renderer.CacheStats(); got.Hits <= stats.Hits {
		t.Fatalf("width-aware element did not reuse constrained layout: before %+v after %+v",
			stats, got)
	}
	box := result.Box
	if box.Width != frame {
		t.Fatalf("box frame = %v, want %v", box.Width, frame)
	}
	if len(box.Lines) < 2 {
		t.Fatalf("wrapped lines = %d, want at least 2", len(box.Lines))
	}
	if rect.H != box.Bounds().Height+2*padding {
		t.Fatalf("element height = %v, want %v", rect.H, box.Bounds().Height+2*padding)
	}
	centered := false
	for _, line := range box.Lines {
		if line.OffsetX > 0 {
			centered = true
			break
		}
	}
	if !centered {
		t.Fatalf("center-aligned wrapped lines have no horizontal offset: %+v", box.Lines)
	}
}

func TestDisplayMathWidthWideContentAndFallback(t *testing.T) {
	const (
		width   = 300
		padding = 4
		frame   = width - 2*padding
		source  = `a+b+c+d`
	)
	renderer := newTestRenderer(t, Options{
		DisplaySize: 24,
		Draw:        DrawOptions{Padding: padding},
	})
	tester := ui.NewTester(func(c *ui.Context) {
		renderer.DisplayMathWidth(c, source, width)
	}, width, 200)
	rect, ok := tester.Find(source)
	if !ok {
		t.Fatalf("wide element not found in %q", tester.Texts())
	}
	if rect.W != width {
		t.Fatalf("wide element width = %v, want %v", rect.W, width)
	}
	result, err := renderer.Render(context.Background(), source, untie.Display, untie.Style{
		Size:     24,
		MaxWidth: frame,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Box.Lines) != 1 {
		t.Fatalf("wide content lines = %d, want 1", len(result.Box.Lines))
	}

	narrow := ui.NewTester(func(c *ui.Context) {
		renderer.DisplayMathWidth(c, source, 1)
	}, 120, 80)
	if !narrow.HasText(source) {
		t.Fatalf("invalid width did not fall back to source: %q", narrow.Texts())
	}
}

func TestCacheSeparatesWidthAwareLayouts(t *testing.T) {
	renderer := newTestRenderer(t, Options{DisplaySize: 24, CacheEntries: 4})
	ctx := context.Background()
	source := `a+b+c+d`
	narrow, err := renderer.Render(ctx, source, untie.Display, untie.Style{
		Size:     24,
		MaxWidth: 60,
	})
	if err != nil {
		t.Fatal(err)
	}
	wide, err := renderer.Render(ctx, source, untie.Display, untie.Style{
		Size:     24,
		MaxWidth: 260,
	})
	if err != nil {
		t.Fatal(err)
	}
	if narrow.Box == wide.Box {
		t.Fatal("different MaxWidth values shared one cached box")
	}
	if len(narrow.Box.Lines) < 2 || len(wide.Box.Lines) != 1 {
		t.Fatalf("line counts = narrow %d, wide %d",
			len(narrow.Box.Lines), len(wide.Box.Lines))
	}
	again, err := renderer.Render(ctx, source, untie.Display, untie.Style{
		Size:     24,
		MaxWidth: 60,
	})
	if err != nil {
		t.Fatal(err)
	}
	if again.Box != narrow.Box {
		t.Fatal("narrow layout was not reused from cache")
	}
	stats := renderer.CacheStats()
	if stats.Hits != 1 || stats.Misses != 2 || stats.Entries != 2 {
		t.Fatalf("cache stats = %+v, want 2 misses, 1 hit, 2 entries", stats)
	}
}

func TestCacheSeparatesColorPolicies(t *testing.T) {
	renderer := newTestRenderer(t, Options{CacheEntries: 4})
	ctx := context.Background()
	source := `\textcolor{red}{x}`
	preserved, err := renderer.Render(ctx, source, untie.Inline, untie.Style{Size: 24})
	if err != nil {
		t.Fatal(err)
	}
	mapped, err := renderer.Render(ctx, source, untie.Inline, untie.Style{
		Size: 24,
		ColorPolicy: untie.ColorPolicy{
			Mode: untie.ColorMapToRoles,
			Palette: untie.ColorRolePalette{
				Ink:         untie.RGBA{A: 0xff},
				Error:       untie.RGBA{R: 0xff, A: 0xff},
				Placeholder: untie.RGBA{G: 0xff, A: 0xff},
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if preserved.Box == mapped.Box {
		t.Fatal("different color policies shared one cached box")
	}
	again, err := renderer.Render(ctx, source, untie.Inline, untie.Style{Size: 24})
	if err != nil {
		t.Fatal(err)
	}
	if again.Box != preserved.Box {
		t.Fatal("preserve policy was not reused from cache")
	}
	if stats := renderer.CacheStats(); stats.Hits != 1 || stats.Misses != 2 || stats.Entries != 2 {
		t.Fatalf("cache stats = %+v, want 2 misses, 1 hit, 2 entries", stats)
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
	if err := DrawBox(nil, &untie.Box{}, ui.Rect{W: 10, H: 10}, DrawOptions{}); !strings.Contains(err.Error(), "painter") {
		t.Fatalf("DrawBox nil painter error = %v", err)
	}
	if err := DrawBox(&ui.Painter{}, nil, ui.Rect{W: 10, H: 10}, DrawOptions{}); err != ErrInvalidBox {
		t.Fatalf("DrawBox nil box error = %v", err)
	}
	if err := DrawBox(&ui.Painter{}, &untie.Box{}, ui.Rect{W: 10, H: 10}, DrawOptions{
		Scale: ScaleMode(255),
	}); !strings.Contains(err.Error(), "scale mode") {
		t.Fatalf("DrawBox invalid scale error = %v", err)
	}
}
