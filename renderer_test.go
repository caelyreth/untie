package untie

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"

	"github.com/caelyreth/untie/display"
	"github.com/caelyreth/untie/source"
)

func TestNewRendererValidationAndDefaults(t *testing.T) {
	t.Parallel()

	if renderer, err := NewRenderer(nil, RendererOptions{}); renderer != nil ||
		!errors.Is(err, ErrInvalidRenderer) || !errors.Is(err, ErrInvalidFonts) {
		t.Fatalf("NewRenderer(nil) = %v, %v", renderer, err)
	}
	fonts := loadTestFonts(t)
	if renderer, err := NewRenderer(fonts, RendererOptions{DefaultDialect: Dialect(255)}); renderer != nil ||
		!errors.Is(err, ErrInvalidRenderer) {
		t.Fatalf("NewRenderer(invalid dialect) = %v, %v", renderer, err)
	}
	if renderer, err := NewRenderer(fonts, RendererOptions{
		DefaultStyle: display.Style{Size: -1},
	}); renderer != nil || !errors.Is(err, ErrInvalidRenderer) || !errors.Is(err, ErrInvalidStyle) {
		t.Fatalf("NewRenderer(invalid style) = %v, %v", renderer, err)
	}
	if renderer, err := NewRenderer(fonts, RendererOptions{
		Recovery: RecoveryMode(255),
	}); renderer != nil || !errors.Is(err, ErrInvalidRenderer) ||
		!errors.Is(err, ErrInvalidRecovery) {
		t.Fatalf("NewRenderer(invalid recovery) = %v, %v", renderer, err)
	}

	renderer, err := NewRenderer(fonts, RendererOptions{CacheEntries: -1})
	if err != nil {
		t.Fatalf("NewRenderer() error = %v", err)
	}
	if renderer.cache != nil {
		t.Fatal("negative CacheEntries enabled the parse cache")
	}
	if renderer.defaultDialect != TeX {
		t.Fatalf("default dialect = %v, want TeX", renderer.defaultDialect)
	}
	if renderer.defaultStyle.Size != defaultRendererSize {
		t.Fatalf("default size = %v, want %v", renderer.defaultStyle.Size, defaultRendererSize)
	}
	var nilRenderer *Renderer
	if result, err := nilRenderer.Render(context.Background(), "x", RenderRequest{}); result != nil ||
		!errors.Is(err, ErrInvalidRenderer) {
		t.Fatalf("nil Renderer.Render() = %#v, %v", result, err)
	}
	//nolint:staticcheck // This assertion intentionally verifies nil-context handling.
	if result, err := renderer.Render(nil, "x", RenderRequest{}); result != nil ||
		!errors.Is(err, ErrInvalidContext) {
		t.Fatalf("Render(nil context) = %#v, %v", result, err)
	}
}

func TestRenderAppliesDialectModeSizeAndStyleDefaults(t *testing.T) {
	t.Parallel()

	features := []string{"dtls=1"}
	renderer, err := NewRenderer(loadTestFonts(t), RendererOptions{
		DefaultDialect: Typst,
		DefaultStyle: display.Style{
			Size:        24,
			MaxWidth:    320,
			Align:       display.AlignCenter,
			Features:    features,
			GenerateAlt: true,
		},
	})
	if err != nil {
		t.Fatalf("NewRenderer() error = %v", err)
	}
	features[0] = "changed"

	result, err := renderer.Render(context.Background(), `frac(1, 2)`, RenderRequest{})
	if err != nil {
		t.Fatalf("Render(defaults) error = %v", err)
	}
	if result.Box == nil {
		t.Fatal("Render(defaults) Box = nil")
	}
	if len(result.Diagnostics) != 0 {
		t.Fatalf("Render(defaults) diagnostics = %+v", result.Diagnostics)
	}
	if result.Alt.Text == "" || result.Alt.Speech == "" || result.Alt != result.Box.Alt {
		t.Fatalf("Render(defaults) Alt = %#v, Box.Alt = %#v", result.Alt, result.Box.Alt)
	}
	style, err := renderer.effectiveStyle(display.Style{})
	if err != nil {
		t.Fatalf("effectiveStyle() error = %v", err)
	}
	if style.Size != 24 || style.MaxWidth != 320 || style.Align != display.AlignCenter ||
		style.ScriptLevel != 0 || len(style.Features) != 1 || style.Features[0] != "dtls=1" ||
		!style.GenerateAlt {
		t.Fatalf("effective default style = %+v", style)
	}

	explicit, err := renderer.engine.Layout(ParseTypst(`frac(1, 2)`, Inline, ParseOptions{}), style)
	if err != nil {
		t.Fatalf("explicit Layout() error = %v", err)
	}
	gotJSON, err := result.Box.MarshalCanonical()
	if err != nil {
		t.Fatalf("Render Box.MarshalCanonical() error = %v", err)
	}
	wantJSON, err := explicit.MarshalCanonical()
	if err != nil {
		t.Fatalf("explicit MarshalCanonical() error = %v", err)
	}
	if string(gotJSON) != string(wantJSON) {
		t.Fatalf("default render differs from explicit style\n got: %s\nwant: %s", gotJSON, wantJSON)
	}
}

func TestRenderDefaultSizeIsSixteenWithZeroOptions(t *testing.T) {
	t.Parallel()

	renderer, err := NewRenderer(loadTestFonts(t), RendererOptions{})
	if err != nil {
		t.Fatalf("NewRenderer() error = %v", err)
	}
	result, err := renderer.Render(context.Background(), `x+1`, RenderRequest{})
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}
	explicit, err := renderer.engine.Layout(
		ParseTeX(`x+1`, Inline, ParseOptions{}),
		display.Style{Size: defaultRendererSize},
	)
	if err != nil {
		t.Fatalf("explicit Layout() error = %v", err)
	}
	gotJSON, err := result.Box.MarshalCanonical()
	if err != nil {
		t.Fatalf("Render Box.MarshalCanonical() error = %v", err)
	}
	wantJSON, err := explicit.MarshalCanonical()
	if err != nil {
		t.Fatalf("explicit MarshalCanonical() error = %v", err)
	}
	if string(gotJSON) != string(wantJSON) {
		t.Fatalf("default 16px render differs from explicit style\n got: %s\nwant: %s", gotJSON, wantJSON)
	}
}

func TestRenderPreservesRecoverableDiagnostics(t *testing.T) {
	t.Parallel()

	renderer, err := NewRenderer(loadTestFonts(t), RendererOptions{CacheEntries: 2})
	if err != nil {
		t.Fatalf("NewRenderer() error = %v", err)
	}
	request := RenderRequest{Style: display.Style{Size: 16}}
	first, err := renderer.Render(context.Background(), `\frac{1}`, request)
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}
	if first.Box == nil {
		t.Fatal("Render() Box = nil")
	}
	if len(first.Diagnostics) == 0 {
		t.Fatal("Render() lost recoverable diagnostics")
	}
	first.Diagnostics[0].Message = "mutated"
	second, err := renderer.Render(context.Background(), `\frac{1}`, request)
	if err != nil {
		t.Fatalf("Render(cached) error = %v", err)
	}
	if len(second.Diagnostics) == 0 {
		t.Fatal("Render(cached) lost recoverable diagnostics")
	}
	if second.Diagnostics[0].Message == "mutated" {
		t.Fatal("Render() results share diagnostic storage")
	}
	if err := second.Box.Validate(); err != nil {
		t.Fatalf("Render(cached) Box invalid: %v", err)
	}
}

func TestRenderIntersectsRendererAndRequestParseLimits(t *testing.T) {
	t.Parallel()

	renderer, err := NewRenderer(loadTestFonts(t), RendererOptions{
		Limits: Limits{MaxSourceBytes: 3},
	})
	if err != nil {
		t.Fatalf("NewRenderer() error = %v", err)
	}
	result, err := renderer.Render(
		context.Background(),
		"abcdef",
		RenderRequest{Style: display.Style{Size: 16}},
	)
	if result == nil {
		t.Fatalf("Render() result = nil, error = %v", err)
	}
	if len(result.Diagnostics) == 0 || result.Diagnostics[0].Code != source.CodeLimitExceeded {
		t.Fatalf("Render() diagnostics = %+v, want source limit", result.Diagnostics)
	}
}

func TestRenderRecoversMissingGlyphWithDiagnostic(t *testing.T) {
	t.Parallel()

	renderer, err := NewRenderer(loadTestFonts(t), RendererOptions{})
	if err != nil {
		t.Fatalf("NewRenderer() error = %v", err)
	}
	result, err := renderer.Render(
		context.Background(),
		`x+\text{中}+y`,
		RenderRequest{Style: display.Style{Size: 16}},
	)
	if err != nil {
		t.Fatalf("Render() error = %v, want visible recovery", err)
	}
	if result == nil {
		t.Fatal("Render(recovered) result = nil")
	}
	if result.Box == nil {
		t.Fatal("Render(recovered) returned no box")
	}
	if len(result.Box.Items) == 0 {
		t.Fatal("Render(recovered) box has no drawable items")
	}
	if !hasCode(result.Diagnostics, source.CodeMissingGlyph) {
		t.Fatalf("Render() diagnostics = %+v, want missing-glyph", result.Diagnostics)
	}
	if !hasCode(result.Box.Diagnostics, source.CodeMissingGlyph) {
		t.Fatalf("Box.Diagnostics = %+v, want missing-glyph", result.Box.Diagnostics)
	}
	for _, item := range result.Box.Items {
		if item.Paint.Role == display.RoleError {
			return
		}
	}
	t.Fatal("recovered box has no error-colored missing-glyph placeholder")
}

func TestRenderStrictMissingGlyphReturnsLayoutError(t *testing.T) {
	t.Parallel()

	renderer, err := NewRenderer(loadTestFonts(t), RendererOptions{
		Recovery: RecoveryStrict,
	})
	if err != nil {
		t.Fatalf("NewRenderer() error = %v", err)
	}
	result, err := renderer.Render(
		context.Background(),
		`中`,
		RenderRequest{Style: display.Style{Size: 16}},
	)
	if !errors.Is(err, ErrMissingGlyph) {
		t.Fatalf("Render() error = %v, want ErrMissingGlyph", err)
	}
	if result == nil || result.Box != nil {
		t.Fatalf("Render(strict) result = %+v, want diagnostics and no box", result)
	}
	if !hasCode(result.Diagnostics, source.CodeMissingGlyph) {
		t.Fatalf("Render(strict) diagnostics = %+v, want missing-glyph", result.Diagnostics)
	}
}

func TestRenderRecoversOnlyMissingTextCluster(t *testing.T) {
	t.Parallel()

	renderer, err := NewRenderer(loadTestFonts(t), RendererOptions{})
	if err != nil {
		t.Fatalf("NewRenderer() error = %v", err)
	}
	result, err := renderer.Render(
		context.Background(),
		`x+\text{A中B}+y`,
		RenderRequest{Style: display.Style{Size: 16}},
	)
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}
	if result.Box == nil {
		t.Fatal("Render() Box = nil")
	}
	if len(result.Box.Diagnostics) != 1 || result.Box.Diagnostics[0].Message != `missing glyph: "中"` {
		t.Fatalf("Box.Diagnostics = %+v, want one CJK cluster diagnostic", result.Box.Diagnostics)
	}
	errorRules, inkGlyphs := 0, 0
	for _, item := range result.Box.Items {
		switch {
		case item.Paint.Role == display.RoleError && item.Kind == display.ItemRule:
			errorRules++
		case item.Paint.Role == display.RoleInk && item.Kind == display.ItemGlyphPath:
			inkGlyphs++
		}
	}
	if errorRules != 4 {
		t.Fatalf("error rules = %d, want one four-rule placeholder", errorRules)
	}
	if inkGlyphs < 4 {
		t.Fatalf("ink glyphs = %d, want surrounding x, A, B, and y", inkGlyphs)
	}
}

func TestRenderAcceptsShortTextFeatureTags(t *testing.T) {
	t.Parallel()

	renderer, err := NewRenderer(loadTestFonts(t), RendererOptions{})
	if err != nil {
		t.Fatalf("NewRenderer() error = %v", err)
	}
	result, err := renderer.Render(
		context.Background(),
		`\text{abc}`,
		RenderRequest{Style: display.Style{
			Size:     16,
			Features: []string{"ss1"},
		}},
	)
	if err != nil {
		t.Fatalf("Render(ss1) error = %v", err)
	}
	if result.Box == nil || len(result.Box.Items) == 0 {
		t.Fatal("Render(ss1) returned no visible text")
	}
}

func TestRenderReportsEachRepeatedMissingTextCluster(t *testing.T) {
	t.Parallel()

	renderer, err := NewRenderer(loadTestFonts(t), RendererOptions{})
	if err != nil {
		t.Fatalf("NewRenderer() error = %v", err)
	}
	const sourceText = `x+\text{中中}+y`
	result, err := renderer.Render(
		context.Background(),
		sourceText,
		RenderRequest{Style: display.Style{Size: 16}},
	)
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}
	if len(result.Box.Diagnostics) != 2 {
		t.Fatalf("diagnostics = %#v, want two clusters", result.Box.Diagnostics)
	}
	first := strings.Index(sourceText, "中")
	second := strings.Index(sourceText[first+len("中"):], "中") + first + len("中")
	for i, start := range []int{first, second} {
		want := source.NewRange(start, start+len("中"))
		if got := result.Box.Diagnostics[i].Range; got != want {
			t.Fatalf("diagnostic %d range = %v, want %v", i, got, want)
		}
		if _, ok := findAtomHit(result.Box.Hits, want); !ok {
			t.Fatalf("missing atom hit for diagnostic %d range %v", i, want)
		}
	}
}

func TestRenderMissingGlyphRecoveryAcrossStructures(t *testing.T) {
	t.Parallel()

	renderer, err := NewRenderer(loadTestFonts(t), RendererOptions{})
	if err != nil {
		t.Fatalf("NewRenderer() error = %v", err)
	}
	sources := []string{
		`\frac{x}{\text{中}}`,
		`x^{\text{中}}`,
		`\sqrt{\text{中}}`,
		`\begin{matrix}\text{中}&x\\y&z\end{matrix}`,
		`\begin{aligned}x&=1\\y&=\text{中}\end{aligned}`,
	}
	for _, sourceText := range sources {
		sourceText := sourceText
		t.Run(sourceText, func(t *testing.T) {
			t.Parallel()
			result, err := renderer.Render(
				context.Background(),
				sourceText,
				RenderRequest{Style: display.Style{Size: 16}},
			)
			if err != nil {
				t.Fatalf("Render() error = %v", err)
			}
			if result.Box == nil || len(result.Box.Items) == 0 {
				t.Fatal("recovered formula has no drawable items")
			}
			if !hasCode(result.Box.Diagnostics, source.CodeMissingGlyph) {
				t.Fatalf("diagnostics = %+v, want missing-glyph", result.Box.Diagnostics)
			}
		})
	}
}

func TestContextFailureProducesDiagnostic(t *testing.T) {
	t.Parallel()

	for _, err := range []error{context.Canceled, context.DeadlineExceeded} {
		diagnostic, ok := diagnosticForContextError(err)
		if !ok || diagnostic.Code != source.CodeCanceled {
			t.Fatalf("diagnosticForContextError(%v) = %+v, %v", err, diagnostic, ok)
		}
	}
}

func hasCode(diagnostics []source.Diagnostic, code source.Code) bool {
	for _, diagnostic := range diagnostics {
		if diagnostic.Code == code {
			return true
		}
	}
	return false
}

func TestRenderAlternativeTextIsOptional(t *testing.T) {
	t.Parallel()

	renderer, err := NewRenderer(loadTestFonts(t), RendererOptions{})
	if err != nil {
		t.Fatalf("NewRenderer() error = %v", err)
	}
	plain, err := renderer.Render(
		context.Background(),
		`x_i^2`,
		RenderRequest{Style: display.Style{Size: 16}},
	)
	if err != nil {
		t.Fatalf("Render(plain) error = %v", err)
	}
	if plain.Alt != (display.Alt{}) || plain.Box.Alt != (display.Alt{}) {
		t.Fatalf("Render(plain) Alt = %#v, Box.Alt = %#v", plain.Alt, plain.Box.Alt)
	}
	accessible, err := renderer.Render(
		context.Background(),
		`x_i^2`,
		RenderRequest{Style: display.Style{Size: 16, GenerateAlt: true}},
	)
	if err != nil {
		t.Fatalf("Render(accessible) error = %v", err)
	}
	if accessible.Alt.Text == "" || accessible.Alt.Speech == "" {
		t.Fatalf("Render(accessible) Alt = %#v", accessible.Alt)
	}
	if accessible.Alt != accessible.Box.Alt {
		t.Fatalf("Render(accessible) Alt = %#v, Box.Alt = %#v", accessible.Alt, accessible.Box.Alt)
	}
}

func TestRenderContextCancellation(t *testing.T) {
	t.Parallel()

	renderer, err := NewRenderer(loadTestFonts(t), RendererOptions{})
	if err != nil {
		t.Fatalf("NewRenderer() error = %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if result, err := renderer.Render(ctx, "x", RenderRequest{}); result != nil ||
		!errors.Is(err, context.Canceled) {
		t.Fatalf("Render(canceled) = %#v, %v", result, err)
	}
}

func TestRenderLayoutCancellationProducesDiagnostic(t *testing.T) {
	t.Parallel()

	renderer, err := NewRenderer(loadTestFonts(t), RendererOptions{})
	if err != nil {
		t.Fatalf("NewRenderer() error = %v", err)
	}
	parseOptions := renderer.effectiveParseOptions(ParseOptions{})
	expr, err := renderer.parse(context.Background(), "x", TeX, Inline, parseOptions)
	if err != nil {
		t.Fatalf("parse error = %v", err)
	}
	style, err := renderer.effectiveStyle(RenderRequest{}.Style)
	if err != nil {
		t.Fatalf("effectiveStyle error = %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	result, err := renderer.renderParsed(ctx, "x", expr, style)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("renderParsed error = %v, want context.Canceled", err)
	}
	if result == nil || !hasCode(result.Diagnostics, source.CodeCanceled) {
		t.Fatalf("renderParsed result = %#v, want cancellation diagnostic", result)
	}
}

func TestRendererCacheHitEvictionAndEffectiveKey(t *testing.T) {
	t.Parallel()

	renderer, err := NewRenderer(loadTestFonts(t), RendererOptions{
		CacheEntries: 1,
		Limits:       Limits{MaxNodes: 50},
	})
	if err != nil {
		t.Fatalf("NewRenderer() error = %v", err)
	}
	ctx := context.Background()
	opts := renderer.effectiveParseOptions(ParseOptions{})
	first, err := renderer.parse(ctx, "x", TeX, Inline, opts)
	if err != nil {
		t.Fatalf("parse(first) error = %v", err)
	}
	hit, err := renderer.parse(ctx, "x", TeX, Inline, opts)
	if err != nil {
		t.Fatalf("parse(hit) error = %v", err)
	}
	if first != hit {
		t.Fatal("cache hit returned a different Expr")
	}
	if got := renderer.cache.length(); got != 1 {
		t.Fatalf("cache length = %d, want 1", got)
	}

	_, err = renderer.parse(ctx, "y", TeX, Inline, opts)
	if err != nil {
		t.Fatalf("parse(second key) error = %v", err)
	}
	evicted, err := renderer.parse(ctx, "x", TeX, Inline, opts)
	if err != nil {
		t.Fatalf("parse(evicted) error = %v", err)
	}
	if evicted == first {
		t.Fatal("LRU eviction retained the evicted Expr")
	}
	if got := renderer.cache.length(); got != 1 {
		t.Fatalf("cache length after eviction = %d, want 1", got)
	}

	changedLimits := renderer.effectiveParseOptions(ParseOptions{Limits: Limits{MaxSourceBytes: 99}})
	if makeRenderExprCacheKey("x", TeX, Inline, opts) ==
		makeRenderExprCacheKey("x", TeX, Inline, changedLimits) {
		t.Fatal("effective parse limits do not affect the cache key")
	}
	if makeRenderExprCacheKey("x", Typst, Inline, ParseOptions{
		Macros: map[string]Macro{"a": {Body: "A"}},
	}) != makeRenderExprCacheKey("x", Typst, Inline, ParseOptions{
		Macros: map[string]Macro{"a": {Body: "B"}},
	}) {
		t.Fatal("Typst cache key includes ignored TeX macros")
	}
}

func TestRenderReturnsIndependentBoxes(t *testing.T) {
	t.Parallel()

	renderer, err := NewRenderer(loadTestFonts(t), RendererOptions{CacheEntries: 4})
	if err != nil {
		t.Fatalf("NewRenderer() error = %v", err)
	}
	request := RenderRequest{Style: display.Style{Size: 18}}
	first, err := renderer.Render(context.Background(), `x+1`, request)
	if err != nil {
		t.Fatalf("Render(first) error = %v", err)
	}
	second, err := renderer.Render(context.Background(), `x+1`, request)
	if err != nil {
		t.Fatalf("Render(second) error = %v", err)
	}
	if first.Box == second.Box {
		t.Fatal("Render reused a mutable Box")
	}
	if len(first.Box.Items) == 0 || len(second.Box.Items) == 0 {
		t.Fatal("Render produced no items")
	}
	before, err := second.Box.MarshalCanonical()
	if err != nil {
		t.Fatalf("second Box.MarshalCanonical() error = %v", err)
	}
	first.Box.Items[0].Paint.Role = display.RoleError
	first.Box.Items[0].Bound.X += 10
	after, err := second.Box.MarshalCanonical()
	if err != nil {
		t.Fatalf("second Box.MarshalCanonical() after mutation error = %v", err)
	}
	if string(before) != string(after) {
		t.Fatal("mutating the first Box changed the second Box")
	}
}

func TestRenderCacheConcurrentUse(t *testing.T) {
	t.Parallel()

	renderer, err := NewRenderer(loadTestFonts(t), RendererOptions{CacheEntries: 4})
	if err != nil {
		t.Fatalf("NewRenderer() error = %v", err)
	}
	const workers = 32
	results := make([][]byte, workers)
	errs := make([]error, workers)
	var wg sync.WaitGroup
	for i := range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			result, err := renderer.Render(
				context.Background(),
				`\frac{x}{2}+1`,
				RenderRequest{Style: display.Style{Size: 16}},
			)
			if err != nil {
				errs[i] = err
				return
			}
			if err := result.Box.Validate(); err != nil {
				errs[i] = err
				return
			}
			results[i], errs[i] = result.Box.MarshalCanonical()
		}()
	}
	wg.Wait()
	for i := range workers {
		if errs[i] != nil {
			t.Fatalf("worker %d error = %v", i, errs[i])
		}
		if string(results[i]) != string(results[0]) {
			t.Fatalf("worker %d output differs", i)
		}
	}
	if got := renderer.cache.length(); got != 1 {
		t.Fatalf("cache length = %d, want 1", got)
	}
}

func TestRenderMacroCacheKeyIsStableAndComplete(t *testing.T) {
	t.Parallel()

	a := makeRenderExprCacheKey("x", TeX, Inline, ParseOptions{
		Macros: map[string]Macro{
			"a": {Body: "A", Args: 0},
			"b": {Body: "B", Args: 1},
		},
	})
	b := makeRenderExprCacheKey("x", TeX, Inline, ParseOptions{
		Macros: map[string]Macro{
			"b": {Body: "B", Args: 1},
			"a": {Body: "A", Args: 0},
		},
	})
	if a != b {
		t.Fatal("macro cache key depends on map insertion order")
	}
	changed := makeRenderExprCacheKey("x", TeX, Inline, ParseOptions{
		Macros: map[string]Macro{
			"a": {Body: "A", Args: 1},
			"b": {Body: "B", Args: 1},
		},
	})
	if a == changed {
		t.Fatal("macro cache key ignored argument count")
	}
	base := makeRenderExprCacheKey("x", TeX, Inline, ParseOptions{})
	for name, candidate := range map[string]renderExprCacheKey{
		"source":  makeRenderExprCacheKey("y", TeX, Inline, ParseOptions{}),
		"dialect": makeRenderExprCacheKey("x", Typst, Inline, ParseOptions{}),
		"mode":    makeRenderExprCacheKey("x", TeX, Display, ParseOptions{}),
		"limits": makeRenderExprCacheKey("x", TeX, Inline, ParseOptions{
			Limits: Limits{MaxNodes: 1},
		}),
	} {
		if base == candidate {
			t.Fatalf("cache key ignored %s", name)
		}
	}
	if got := renderMacroKey(TeX, nil); got != "" {
		t.Fatalf("renderMacroKey(empty) = %q", got)
	}
	if got := renderMacroKey(Typst, map[string]Macro{"a": {Body: fmt.Sprint(1)}}); got != "" {
		t.Fatalf("renderMacroKey(Typst) = %q", got)
	}
}
