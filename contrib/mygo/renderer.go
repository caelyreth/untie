package untiemygo

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strings"

	"github.com/caelyreth/untie"
	"github.com/caelyreth/untie/mathml"
	"github.com/caelyreth/untie/render"
	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/markdown"
	"github.com/egoist/mygo/ui/markdown/parser"
)

const (
	// DefaultBaseSize is the fallback inline size in MyGo DIPs.
	DefaultBaseSize float32 = 16
	// DefaultDisplaySize is the display-math size in MyGo DIPs.
	DefaultDisplaySize float32 = 24
	// DefaultCacheEntries is used when Options.CacheEntries is zero.
	DefaultCacheEntries = 128
	// MaxCacheEntries bounds one adapter's retained layout count.
	MaxCacheEntries = 4096
)

var (
	// ErrInvalidOptions reports unusable renderer configuration.
	ErrInvalidOptions = errors.New("untiemygo: invalid options")
	// ErrInvalidBox reports a nil box passed to a drawing helper.
	ErrInvalidBox = errors.New("untiemygo: invalid box")
	// ErrInvalidPainter reports a nil painter passed to a drawing helper.
	ErrInvalidPainter = errors.New("untiemygo: invalid painter")
)

// DrawOptions controls placement and painting of a complete Box.Bounds
// rectangle in a viewport. Padding is applied on every side. Alignment affects
// horizontal placement; content is always vertically centered.
//
// ScaleFit uniformly scales a box down to fit the padded viewport, or up when
// AllowUpscale is true. ScaleNatural draws at scale 1 and clips only at the
// viewport boundary. Paint resolves semantic and explicit paints at draw time;
// nil uses the active MyGo theme.
type DrawOptions struct {
	Padding      float32
	Align        render.Align
	Scale        ScaleMode
	AllowUpscale bool
	Paint        PaintResolver
}

// Options configure a MyGo adapter.
//
// Dialect selects TeX (the zero value) or Typst. BaseSize is the fallback for
// non-positive InlineMath sizes. DisplaySize controls display math; zero uses
// 1.5 times BaseSize. CacheEntries zero uses DefaultCacheEntries, a negative
// value disables caching, and values above MaxCacheEntries are rejected.
// Limits apply to parsing, layout, and optional alternative text.
type Options struct {
	Dialect      untie.Dialect
	BaseSize     float32
	DisplaySize  float32
	CacheEntries int
	Limits       untie.Limits
	// ParseOptions supplies TeX macros and host-owned equation reference text.
	// Its resource limits are intersected with Limits.
	ParseOptions untie.ParseOptions
	Draw         DrawOptions
	// Accessibility controls semantic alternative-text generation for
	// standalone elements. The zero value requests it.
	Accessibility AccessibilityMode
	// LocalizeLabel optionally maps the structured Alt.Description to a
	// host-localized accessible label. It is called for standalone elements
	// after layout. An empty result falls back to Alt.Speech, then Alt.Text, then
	// source. The callback must not mutate or retain the description.
	LocalizeLabel func(description *untie.AltDescription) string
	// Inline controls Markdown inline-object baseline metrics.
	Inline InlineOptions
}

// Renderer is a concurrency-safe MyGo adapter over one Untie renderer.
//
// Cache entries hold immutable boxes. Callers may read boxes returned by
// Render, but must not modify them or their nested slices while shared.
type Renderer struct {
	renderer    *untie.Renderer
	dialect     untie.Dialect
	baseSize    float32
	displaySize float32
	draw        DrawOptions
	access      AccessibilityMode
	localize    func(description *untie.AltDescription) string
	inline      InlineOptions
	parse       untie.ParseOptions
	cache       *layoutCache
}

// New prepares a MyGo adapter. The font set must already be loaded.
func New(fonts *untie.FontSet, opts Options) (*Renderer, error) {
	if fonts == nil {
		return nil, fmt.Errorf("%w: nil font set", ErrInvalidOptions)
	}
	if opts.Dialect != untie.TeX && opts.Dialect != untie.Typst {
		return nil, fmt.Errorf("%w: unknown dialect %d", ErrInvalidOptions, opts.Dialect)
	}
	baseSize := opts.BaseSize
	if baseSize == 0 {
		baseSize = DefaultBaseSize
	}
	if !positiveFinite(baseSize) {
		return nil, fmt.Errorf("%w: base size %v is not positive and finite",
			ErrInvalidOptions, baseSize)
	}
	displaySize := opts.DisplaySize
	if displaySize == 0 {
		if opts.BaseSize == 0 {
			displaySize = DefaultDisplaySize
		} else {
			displaySize = baseSize * 1.5
		}
	}
	if !positiveFinite(displaySize) {
		return nil, fmt.Errorf("%w: display size %v is not positive and finite",
			ErrInvalidOptions, displaySize)
	}
	cacheEntries := opts.CacheEntries
	if cacheEntries == 0 {
		cacheEntries = DefaultCacheEntries
	}
	if cacheEntries > MaxCacheEntries {
		return nil, fmt.Errorf("%w: cache entries %d exceed %d",
			ErrInvalidOptions, cacheEntries, MaxCacheEntries)
	}
	if !opts.Accessibility.validate() {
		return nil, fmt.Errorf("%w: unknown accessibility mode %d",
			ErrInvalidOptions, opts.Accessibility)
	}
	if !opts.Inline.validate() {
		return nil, fmt.Errorf("%w: unknown inline metrics mode %d",
			ErrInvalidOptions, opts.Inline.Metrics)
	}
	if err := validateDrawOptions(opts.Draw); err != nil {
		return nil, err
	}
	parseOptions := cloneParseOptions(opts.ParseOptions)
	parseOptions.Limits = stricterLimits(opts.Limits, parseOptions.Limits)
	renderer, err := untie.NewRenderer(fonts, untie.RendererOptions{
		Limits:         opts.Limits,
		DefaultDialect: opts.Dialect,
		DefaultStyle:   untie.Style{Size: displaySize},
	})
	if err != nil {
		return nil, fmt.Errorf("%w: untie renderer: %v", ErrInvalidOptions, err)
	}
	return &Renderer{
		renderer:    renderer,
		dialect:     opts.Dialect,
		baseSize:    baseSize,
		displaySize: displaySize,
		draw:        opts.Draw,
		access:      opts.Accessibility,
		localize:    opts.LocalizeLabel,
		inline:      opts.Inline,
		parse:       parseOptions,
		cache:       newLayoutCache(cacheEntries),
	}, nil
}

// Render parses and lays out source, reusing a cached immutable box for an
// identical dialect, mode, and style.
//
// A zero Style.Size uses BaseSize for inline mode and DisplaySize for display
// mode. Returned boxes may be read concurrently but must not be mutated.
// Diagnostics are copied for every result.
func (r *Renderer) Render(
	ctx context.Context,
	source string,
	mode untie.Mode,
	style untie.Style,
) (*untie.RenderResult, error) {
	if r == nil || r.renderer == nil {
		return nil, ErrInvalidOptions
	}
	if ctx == nil {
		return nil, untie.ErrInvalidContext
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if mode != untie.Inline && mode != untie.Display {
		return nil, fmt.Errorf("%w: unknown mode %d", ErrInvalidOptions, mode)
	}
	if style.Size == 0 {
		if mode == untie.Display {
			style.Size = r.displaySize
		} else {
			style.Size = r.baseSize
		}
	}
	style.Features = append([]string(nil), style.Features...)
	key := makeLayoutKey(source, r.dialect, mode, style)
	if cached, ok := r.cache.get(key); ok {
		return &untie.RenderResult{
			Box:         cached.box,
			Diagnostics: cloneDiagnostics(cached.diagnostics),
			Alt:         cached.box.Alt.Clone(),
		}, nil
	}
	result, err := r.renderer.Render(ctx, source, untie.RenderRequest{
		Dialect: r.dialect,
		Mode:    mode,
		Style:   style,
		Parse:   r.parse,
	})
	if err != nil {
		return result, err
	}
	r.cache.put(key, cachedRender{box: result.Box, diagnostics: result.Diagnostics})
	return result, nil
}

// InlineMath implements markdown.Options.InlineMath.
//
// It returns false when rendering fails. By default the object dimensions map
// the complete Box.Bounds, including certified overflow, around the baseline.
// Options.Inline can instead report nominal metrics and preserve overflow only
// as drawable ink inside the surrounding text clip.
func (r *Renderer) InlineMath(source string, size float32) (markdown.InlineObject, bool) {
	if r == nil {
		return markdown.InlineObject{}, false
	}
	if size <= 0 {
		size = r.baseSize
	} else if !finite(size) {
		return markdown.InlineObject{}, false
	}
	result, err := r.Render(context.Background(), source, untie.Inline,
		untie.Style{Size: size})
	if err != nil || result == nil || result.Box == nil {
		return markdown.InlineObject{}, false
	}
	box := result.Box
	width, height, depth := inlineObjectMetrics(box, r.inline.Metrics)
	return markdown.InlineObject{
		Width:  width,
		Height: height,
		Depth:  depth,
		Draw: func(painter *ui.Painter, rect ui.Rect) {
			r.drawInline(painter, box, rect, r.inline.Metrics)
		},
	}, true
}

// Element renders source as a standalone MyGo element.
//
// The element has the formula's natural size plus the configured padding and
// scales down when its layout box is smaller. Inline mode is useful for
// standalone formula elements; use InlineMath or BindMarkdown when the formula
// must participate in a text baseline.
//
// Rendering reuses the adapter cache. The element carries source as its
// accessibility value and semantic alternative text as its label when enabled.
// Options.LocalizeLabel can replace the English label using the structured
// description. If parsing or layout fails, it shows the source as selectable
// danger-colored text instead of disappearing.
func (r *Renderer) Element(c *ui.Context, source string, mode untie.Mode) ui.Element {
	return r.ElementWithOptions(c, source, mode, ElementOptions{})
}

// ElementWithOptions is Element with optional hit callbacks, diagnostic
// decorations, and clipboard actions.
func (r *Renderer) ElementWithOptions(
	c *ui.Context,
	source string,
	mode untie.Mode,
	options ElementOptions,
) ui.Element {
	if c == nil || !c.Valid() {
		return ui.Element{}
	}
	if r == nil {
		return fallbackElement(
			c,
			source,
			DefaultBaseSize,
			untie.RoleError,
			"math renderer unavailable",
		)
	}
	size := r.baseSize
	if mode == untie.Display {
		size = r.displaySize
	}
	result, err := r.Render(context.Background(), source, mode, r.elementStyle(size))
	if err != nil || result == nil || result.Box == nil {
		return fallbackElement(
			c,
			source,
			size,
			untie.RoleError,
			renderFailureText(result, err),
		)
	}
	box := result.Box
	bounds := box.Bounds()
	padding := r.draw.Padding
	return r.formulaElement(
		c,
		source,
		mode,
		result,
		r.draw,
		max(1, bounds.Width+2*padding),
		max(1, bounds.Height+2*padding),
		options,
	)
}

// DisplayMath builds a full-width display-math element for MyGo Markdown.
//
// This is the compact preview path: it lays out at natural width and scales
// down when its host is too narrow. Hosts that know the available width should
// use DisplayMathWidth so Untie can wrap with Style.MaxWidth instead.
//
// A layout failure falls back to selectable source text in the danger color
// rather than hiding the input.
func (r *Renderer) DisplayMath(c *ui.Context, source string) ui.Element {
	return r.DisplayMathWithOptions(c, source, ElementOptions{})
}

// DisplayMathWithOptions is DisplayMath with editor element options.
func (r *Renderer) DisplayMathWithOptions(
	c *ui.Context,
	source string,
	options ElementOptions,
) ui.Element {
	return r.ElementWithOptions(c, source, untie.Display, options).FillWidth()
}

// DisplayMathWidth builds a width-aware display-math element.
//
// width is the element's outer width in MyGo DIPs. The configured draw padding
// is removed from that width and the remaining frame is passed to Untie as
// Style.MaxWidth, producing Box.Lines when the formula does not fit. The
// resulting box is drawn at natural scale rather than uniformly shrunk.
//
// Invalid input or a layout failure falls back to selectable source text in the
// danger color rather than hiding the input.
func (r *Renderer) DisplayMathWidth(
	c *ui.Context,
	source string,
	width float32,
) ui.Element {
	return r.DisplayMathWidthWithOptions(c, source, width, ElementOptions{})
}

// DisplayMathWidthWithOptions is DisplayMathWidth with editor element options.
func (r *Renderer) DisplayMathWidthWithOptions(
	c *ui.Context,
	source string,
	width float32,
	elementOptions ElementOptions,
) ui.Element {
	if c == nil || !c.Valid() {
		return ui.Element{}
	}
	if r == nil {
		return fallbackElement(
			c,
			source,
			DefaultBaseSize,
			untie.RoleError,
			"math renderer unavailable",
		)
	}
	padding := r.draw.Padding
	frameWidth := width - 2*padding
	if !positiveFinite(width) || !positiveFinite(frameWidth) {
		return fallbackElement(
			c,
			source,
			r.displaySize,
			untie.RoleError,
			"display width does not leave a positive content frame",
		)
	}
	style := r.elementStyle(r.displaySize)
	style.MaxWidth = frameWidth
	style.Align = r.draw.Align
	result, err := r.Render(context.Background(), source, untie.Display, style)
	if err != nil || result == nil || result.Box == nil {
		return fallbackElement(
			c,
			source,
			r.displaySize,
			untie.RoleError,
			renderFailureText(result, err),
		)
	}
	box := result.Box
	bounds := box.Bounds()
	draw := r.draw
	draw.Scale = ScaleNatural
	return r.formulaElement(
		c,
		source,
		untie.Display,
		result,
		draw,
		width,
		max(1, bounds.Height+2*padding),
		elementOptions,
	)
}

// BindMarkdown installs math rendering on options and enables single-dollar
// inline math. Existing non-math callbacks and parser extensions are retained.
func (r *Renderer) BindMarkdown(options *markdown.Options) {
	r.BindMarkdownWithOptions(options, ElementOptions{})
}

// BindMarkdownWithOptions is BindMarkdown with editor element options applied
// to display math. Inline math keeps the markdown.InlineObject contract and is
// configured through Options.Inline.
func (r *Renderer) BindMarkdownWithOptions(
	options *markdown.Options,
	elementOptions ElementOptions,
) {
	if r == nil || options == nil {
		return
	}
	options.Extensions |= parser.ExtMath | parser.ExtDollarMath
	options.Math = func(c *ui.Context, source string) {
		r.DisplayMathWithOptions(c, source, elementOptions)
	}
	options.InlineMath = r.InlineMath
}

func (r *Renderer) elementStyle(size float32) untie.Style {
	return untie.Style{
		Size:        size,
		GenerateAlt: r.access == AccessibilityAuto,
	}
}

// CacheStats reports the current cache size, hard capacity, and hit/miss
// counters.
func (r *Renderer) CacheStats() CacheStats {
	if r == nil {
		return CacheStats{}
	}
	return r.cache.stats()
}

func (r *Renderer) formulaElement(
	c *ui.Context,
	source string,
	mode untie.Mode,
	result *untie.RenderResult,
	draw DrawOptions,
	width float32,
	height float32,
	options ElementOptions,
) ui.Element {
	box := result.Box
	semantics := DescribeFormula(source, result.Alt)
	label := accessibleLabel(semantics, r.localize)
	element := ui.Box(c).
		Width(width).
		Height(height).
		Role(ui.RoleImage).
		Label(label).
		Value(semantics.Value).
		Draw(func(painter *ui.Painter, rect ui.Rect) {
			_ = DrawBox(painter, box, rect, draw)
		})
	if options.Diagnostics.Mode&DiagnosticUnderline != 0 {
		element = element.DrawOver(func(painter *ui.Painter, rect ui.Rect) {
			paintDiagnostics(
				painter,
				rect,
				box,
				result.Diagnostics,
				draw,
				options.Diagnostics,
			)
		})
	}
	if options.Diagnostics.Mode&DiagnosticTooltip != 0 {
		if tooltip := DiagnosticTooltipText(
			result.Diagnostics,
			options.Diagnostics.Format,
		); tooltip != "" {
			element = element.Tooltip(tooltip)
		}
	}
	element = wirePointerHits(
		element,
		source,
		mode,
		box,
		width,
		height,
		draw,
		options.OnHit,
	)
	if options.OnHit != nil {
		element = element.Cursor(ui.CursorPointer)
	}
	if options.Clipboard.Enabled {
		element = addClipboardMenu(
			element,
			c,
			source,
			result.Alt,
			r.dialect,
			func() ([]byte, error) {
				return r.MarshalMathML(context.Background(), source, mode)
			},
			options.Clipboard,
		)
	}
	return element
}

// MarshalMathML exports source as Presentation MathML using the adapter's
// dialect and ParseOptions.
func (r *Renderer) MarshalMathML(
	ctx context.Context,
	source string,
	mode untie.Mode,
) ([]byte, error) {
	if r == nil || r.renderer == nil {
		return nil, ErrInvalidOptions
	}
	if ctx == nil {
		return nil, untie.ErrInvalidContext
	}
	if mode != untie.Inline && mode != untie.Display {
		return nil, fmt.Errorf("%w: unknown mode %d", ErrInvalidOptions, mode)
	}
	expr, err := untie.ParseContext(ctx, source, r.dialect, mode, r.parse)
	if err != nil {
		return nil, err
	}
	formula, err := expr.RootSnapshot()
	if err != nil {
		return nil, err
	}
	return mathml.Marshal(ctx, formula, mode, mathml.Options{Limits: r.parse.Limits})
}

func cloneParseOptions(options untie.ParseOptions) untie.ParseOptions {
	options.Limits = options.Limits.WithDefaults()
	if len(options.Macros) > 0 {
		macros := make(map[string]untie.Macro, len(options.Macros))
		for name, macro := range options.Macros {
			macros[name] = macro
		}
		options.Macros = macros
	} else {
		options.Macros = nil
	}
	if len(options.References) > 0 {
		references := make(map[string]string, len(options.References))
		for name, value := range options.References {
			references[name] = value
		}
		options.References = references
	} else {
		options.References = nil
	}
	return options
}

func stricterLimits(left, right untie.Limits) untie.Limits {
	left = left.WithDefaults()
	right = right.WithDefaults()
	minInt := func(a, b int) int {
		if a < b {
			return a
		}
		return b
	}
	minDimension := func(a, b int64) int64 {
		if a < b {
			return a
		}
		return b
	}
	return untie.Limits{
		MaxSourceBytes:     minInt(left.MaxSourceBytes, right.MaxSourceBytes),
		MaxTokens:          minInt(left.MaxTokens, right.MaxTokens),
		MaxNesting:         minInt(left.MaxNesting, right.MaxNesting),
		MaxNodes:           minInt(left.MaxNodes, right.MaxNodes),
		MaxMacroExpansions: minInt(left.MaxMacroExpansions, right.MaxMacroExpansions),
		MaxExpandedTokens:  minInt(left.MaxExpandedTokens, right.MaxExpandedTokens),
		MaxCallDepth:       minInt(left.MaxCallDepth, right.MaxCallDepth),
		MaxTableCells:      minInt(left.MaxTableCells, right.MaxTableCells),
		MaxAssemblyRepeats: minInt(left.MaxAssemblyRepeats, right.MaxAssemblyRepeats),
		MaxTextBytes:       minInt(left.MaxTextBytes, right.MaxTextBytes),
		MaxOutputItems:     minInt(left.MaxOutputItems, right.MaxOutputItems),
		MaxPathSegments:    minInt(left.MaxPathSegments, right.MaxPathSegments),
		MaxDimension:       minDimension(left.MaxDimension, right.MaxDimension),
	}
}

func accessibleLabel(
	semantics FormulaSemantics,
	localize func(description *untie.AltDescription) string,
) string {
	if localize != nil && semantics.Description != nil {
		if label := strings.TrimSpace(localize(semantics.Description)); label != "" {
			return label
		}
	}
	return semantics.Label
}

func fallbackElement(
	c *ui.Context,
	source string,
	size float32,
	role untie.Role,
	detail string,
) ui.Element {
	semantics := DescribeFormula(source, untie.Alt{})
	element := ui.Text(c, source).
		FontSize(size).
		TextColor(PaintColor(c.Theme(), untie.Paint{Role: role})).
		Selectable().
		Label(semantics.Label).
		Value(semantics.Value)
	if detail != "" {
		element = element.Tooltip(detail)
	}
	return element
}

func renderFailureText(result *untie.RenderResult, err error) string {
	if err != nil {
		return err.Error()
	}
	if result != nil {
		return DiagnosticTooltipText(result.Diagnostics, nil)
	}
	return ""
}

func inlineExtents(bounds untie.Rect) (height, depth float32) {
	height = max(0, -bounds.Y)
	depth = max(0, bounds.Y+bounds.Height)
	return height, depth
}

func positiveFinite(value float32) bool {
	return finite(value) && value > 0
}

func finite(value float32) bool {
	number := float64(value)
	return !math.IsNaN(number) && !math.IsInf(number, 0)
}
