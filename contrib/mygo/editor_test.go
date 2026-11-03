package untiemygo

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/caelyreth/untie"
	"github.com/caelyreth/untie/render"
	"github.com/egoist/mygo/transfer"
	"github.com/egoist/mygo/ui"
)

func TestDescribeFormula(t *testing.T) {
	semantics := DescribeFormula(`x_i^2`, untie.Alt{
		Speech: `x sub i squared`,
		Text:   `x_i²`,
	})
	if semantics.Label != `x sub i squared` ||
		semantics.Value != `x_i^2` ||
		semantics.PlainText != `x_i²` {
		t.Fatalf("DescribeFormula() = %+v", semantics)
	}

	fallback := DescribeFormula(`\alpha`, untie.Alt{})
	if fallback.Label != `\alpha` || fallback.PlainText != `\alpha` {
		t.Fatalf("source fallback = %+v", fallback)
	}

	empty := DescribeFormula(``, untie.Alt{})
	if empty.Label != emptyFormulaLabel || empty.Value != "" || empty.PlainText != "" {
		t.Fatalf("empty formula semantics = %+v", empty)
	}
}

func TestHitAtUsesDrawTransform(t *testing.T) {
	box := &untie.Box{
		Width:   20,
		Ascent:  10,
		Descent: 0,
		Hits: []untie.Hit{{
			Source: untie.Range{Start: 0, End: 1},
			Area:   untie.Rect{X: 0, Y: -10, Width: 10, Height: 10},
			Kind:   untie.HitAtom,
		}},
	}
	for _, test := range []struct {
		name     string
		viewport ui.Rect
		options  DrawOptions
		x        float32
		y        float32
	}{
		{
			name:     "natural",
			viewport: ui.Rect{W: 40, H: 20},
			options:  DrawOptions{Scale: ScaleNatural, Align: render.AlignCenter},
			x:        15,
			y:        10,
		},
		{
			name:     "fit without upscale",
			viewport: ui.Rect{W: 80, H: 40},
			options:  DrawOptions{Scale: ScaleFit, Align: render.AlignCenter},
			x:        35,
			y:        20,
		},
		{
			name:     "offset viewport",
			viewport: ui.Rect{X: 10, Y: 5, W: 40, H: 20},
			options:  DrawOptions{Scale: ScaleNatural},
			x:        5,
			y:        5,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			hit, point, found, err := HitAt(
				box,
				test.viewport,
				test.options,
				test.x,
				test.y,
			)
			if err != nil {
				t.Fatal(err)
			}
			if !found || hit.Source != (untie.Range{Start: 0, End: 1}) {
				t.Fatalf("HitAt() = %+v, %v, %v", hit, point, found)
			}
			if point.X < 0 || point.X > 10 || point.Y < -10 || point.Y > 0 {
				t.Fatalf("box-local point = %+v", point)
			}
		})
	}

	if _, _, found, err := HitAt(
		box,
		ui.Rect{W: 40, H: 20},
		DrawOptions{Scale: ScaleNatural},
		35,
		10,
	); err != nil || found {
		t.Fatalf("outside HitAt() = found %v, error %v", found, err)
	}
	if _, _, _, err := HitAt(nil, ui.Rect{W: 1, H: 1}, DrawOptions{}, 0, 0); !errors.Is(err, ErrInvalidBox) {
		t.Fatalf("nil box error = %v", err)
	}
}

func TestElementHitCallbacks(t *testing.T) {
	renderer := newTestRenderer(t, Options{})
	const source = `x`
	result, err := renderer.Render(
		context.Background(),
		source,
		untie.Inline,
		untie.Style{Size: 16, GenerateAlt: true},
	)
	if err != nil {
		t.Fatal(err)
	}
	label := DescribeFormula(source, result.Alt).Label
	var events []HitEvent
	tester := ui.NewTester(func(c *ui.Context) {
		renderer.ElementWithOptions(c, source, untie.Inline, ElementOptions{
			OnHit: func(event HitEvent) {
				events = append(events, event)
			},
		})
	}, 120, 80)
	rect, ok := tester.Find(label)
	if !ok {
		t.Fatalf("formula label %q not found in %q", label, tester.Texts())
	}
	x, y := rect.X+rect.W/2, rect.Y+rect.H/2
	tester.Move(x, y)
	tester.ClickAt(x, y)
	tester.Frame()

	phases := map[HitPhase]bool{}
	for _, event := range events {
		phases[event.Phase] = true
		if event.Source != source || event.Mode != untie.Inline {
			t.Fatalf("event identity = %+v", event)
		}
		if event.Phase != HitClick && !event.Found {
			t.Fatalf("pointer event missed the formula: %+v", event)
		}
	}
	if !phases[HitHover] || !phases[HitPress] || !phases[HitClick] {
		t.Fatalf("hit phases = %v, events = %+v", phases, events)
	}
}

func TestDiagnosticMarksAndPainter(t *testing.T) {
	renderer := newTestRenderer(t, Options{})
	const source = `x+\text{中}+y`
	result, err := renderer.Render(
		context.Background(),
		source,
		untie.Display,
		untie.Style{Size: 24, GenerateAlt: true},
	)
	if err != nil {
		t.Fatal(err)
	}
	marks := DiagnosticMarks(result.Box, result.Diagnostics)
	if len(marks) == 0 {
		t.Fatalf("missing-glyph diagnostics have no mapped areas: %+v", result.Diagnostics)
	}
	if text := DiagnosticTooltipText(result.Diagnostics, func(untie.Diagnostic) string {
		return "localized diagnostic"
	}); text != "localized diagnostic" {
		t.Fatalf("DiagnosticTooltipText() = %q", text)
	}

	painted := 0
	ui.NewTester(func(c *ui.Context) {
		renderer.ElementWithOptions(c, source, untie.Display, ElementOptions{
			Diagnostics: DiagnosticOptions{
				Mode: DiagnosticUnderline | DiagnosticTooltip,
				Paint: func(_ *ui.Painter, mark DiagnosticMark) {
					if mark.Diagnostic.Code != untie.CodeMissingGlyph {
						t.Errorf("diagnostic = %+v", mark.Diagnostic)
					}
					painted++
				},
			},
		})
	}, 320, 120)
	if painted == 0 {
		t.Fatal("diagnostic painter was not called")
	}
}

func TestFormulaClipboardData(t *testing.T) {
	data, ok := FormulaClipboardData(
		`x_i^2`,
		untie.Alt{Text: `x_i²`},
		untie.TeX,
	)
	if !ok {
		t.Fatal("FormulaClipboardData rejected TeX")
	}
	formats := data.Formats()
	if len(formats) != 2 ||
		!containsFormat(formats, transfer.Text) ||
		!containsFormat(formats, FormatTeX) {
		t.Fatalf("clipboard formats = %v", formats)
	}
	plain, err := data.Read(transfer.Text)
	if err != nil {
		t.Fatal(err)
	}
	sourceText, err := data.Read(FormatTeX)
	if err != nil {
		t.Fatal(err)
	}
	if string(plain) != `x_i²` || string(sourceText) != `x_i^2` {
		t.Fatalf("clipboard = plain %q source %q", plain, sourceText)
	}
	if _, ok := SourceClipboardData("x", untie.Dialect(99)); ok {
		t.Fatal("unknown dialect received a source format")
	}
}

func TestElementClipboardMenu(t *testing.T) {
	renderer := newTestRenderer(t, Options{Draw: DrawOptions{Padding: 4}})
	const source = `\frac{1}{2}`
	result, err := renderer.Render(
		context.Background(),
		source,
		untie.Display,
		untie.Style{Size: 24, GenerateAlt: true},
	)
	if err != nil {
		t.Fatal(err)
	}
	label := DescribeFormula(source, result.Alt).Label
	tester := ui.NewTester(func(c *ui.Context) {
		renderer.ElementWithOptions(c, source, untie.Display, ElementOptions{
			Clipboard: ClipboardOptions{Enabled: true},
		})
	}, 240, 120)
	rect, ok := tester.Find(label)
	if !ok {
		t.Fatalf("formula label %q not found in %q", label, tester.Texts())
	}
	tester.RightClickAt(rect.X+rect.W/2, rect.Y+rect.H/2)
	if err := tester.ChooseMenuItem("Copy TeX Source"); err != nil {
		t.Fatalf("choose copy source: %v; menu = %v", err, tester.Menu())
	}
	got, err := tester.ClipboardData().Read(FormatTeX)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != source {
		t.Fatalf("clipboard source = %q, want %q", got, source)
	}
}

func TestInlineMetricsModes(t *testing.T) {
	box := &untie.Box{
		Width:    20,
		Ascent:   10,
		Descent:  3,
		Overflow: 4,
	}
	width, height, depth := inlineObjectMetrics(box, InlineIncludeOverflow)
	if width != 28 || height != 14 || depth != 7 {
		t.Fatalf("include-overflow metrics = %v,%v,%v", width, height, depth)
	}
	width, height, depth = inlineObjectMetrics(box, InlineIgnoreOverflow)
	if width != 20 || height != 10 || depth != 3 {
		t.Fatalf("nominal metrics = %v,%v,%v", width, height, depth)
	}
	transform, clip := inlinePlacement(
		box,
		ui.Rect{X: 5, Y: 7, W: 20, H: 13},
		InlineIgnoreOverflow,
	)
	baseline := transform.Apply(untie.Point{X: 0, Y: 0})
	if baseline.X != 5 || baseline.Y != 17 {
		t.Fatalf("baseline origin = %+v", baseline)
	}
	if clip.X != 1 || clip.Y != 3 || clip.W != 28 || clip.H != 21 {
		t.Fatalf("overflow clip = %+v", clip)
	}
}

func TestInlineMathUsesConfiguredMetrics(t *testing.T) {
	const source = `\int_0^1 x^2\,dx`
	renderer := newTestRenderer(t, Options{
		Inline: InlineOptions{Metrics: InlineIgnoreOverflow},
	})
	result, err := renderer.Render(
		context.Background(),
		source,
		untie.Inline,
		untie.Style{Size: 16},
	)
	if err != nil {
		t.Fatal(err)
	}
	inline, ok := renderer.InlineMath(source, 16)
	if !ok {
		t.Fatal("InlineMath failed")
	}
	if inline.Width != result.Box.Width ||
		inline.Height != result.Box.Ascent ||
		inline.Depth != result.Box.Descent {
		t.Fatalf("inline metrics = %+v; box = width %v ascent %v descent %v",
			inline, result.Box.Width, result.Box.Ascent, result.Box.Descent)
	}
}

func TestElementAccessibilityLabelAndFallback(t *testing.T) {
	renderer := newTestRenderer(t, Options{})
	const source = `x^2`
	expr := untie.Parse(source, untie.TeX, untie.Inline, untie.ParseOptions{})
	label := DescribeFormula(source, expr.Alt()).Label
	tester := ui.NewTester(func(c *ui.Context) {
		renderer.Element(c, source, untie.Inline)
	}, 120, 80)
	if _, ok := tester.Find(label); !ok {
		t.Fatalf("semantic label %q not found in %q", label, tester.Texts())
	}

	var absent *Renderer
	fallbackSource := `\frac{1}{`
	fallbackTester := ui.NewTester(func(c *ui.Context) {
		absent.Element(c, fallbackSource, untie.Display)
	}, 160, 80)
	if _, ok := fallbackTester.Find(fallbackSource); !ok {
		t.Fatalf("safe fallback %q not found in %q",
			fallbackSource, fallbackTester.Texts())
	}
}

func TestNewValidatesEditorOptions(t *testing.T) {
	fonts := testFonts(t)
	for _, options := range []Options{
		{Accessibility: AccessibilityMode(255)},
		{Inline: InlineOptions{Metrics: InlineMetricsMode(255)}},
	} {
		if _, err := New(fonts, options); err == nil ||
			!strings.Contains(err.Error(), "unknown") {
			t.Fatalf("New(%+v) error = %v", options, err)
		}
	}
}

func containsFormat(formats []transfer.Format, want transfer.Format) bool {
	for _, format := range formats {
		if format == want {
			return true
		}
	}
	return false
}
