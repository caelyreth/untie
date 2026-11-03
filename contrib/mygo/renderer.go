package untiemygo

import (
	"context"
	"errors"
	"fmt"
	"math"

	"github.com/caelyreth/untie"
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
	Draw         DrawOptions
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
	if err := validateDrawOptions(opts.Draw); err != nil {
		return nil, err
	}
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
	})
	if err != nil {
		return result, err
	}
	r.cache.put(key, cachedRender{box: result.Box, diagnostics: result.Diagnostics})
	return result, nil
}

// InlineMath implements markdown.Options.InlineMath.
//
// It returns false when rendering fails. The object dimensions map the complete
// Box.Bounds, including certified overflow, around the baseline. The adapter
// retains the immutable box and the Draw closure only reads it.
func (r *Renderer) InlineMath(source string, size float32) (markdown.InlineObject, bool) {
	if r == nil {
		return markdown.InlineObject{}, false
	}
	if size <= 0 {
		size = r.baseSize
	}
	result, err := r.Render(context.Background(), source, untie.Inline,
		untie.Style{Size: size})
	if err != nil || result == nil || result.Box == nil {
		return markdown.InlineObject{}, false
	}
	box := result.Box
	bounds := box.Bounds()
	height, depth := inlineExtents(bounds)
	return markdown.InlineObject{
		Width:  bounds.Width,
		Height: height,
		Depth:  depth,
		Draw: func(painter *ui.Painter, rect ui.Rect) {
			r.drawInline(painter, box, rect)
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
// Rendering reuses the adapter cache. If parsing or layout fails, the element
// shows the source as selectable danger-colored text instead of disappearing.
func (r *Renderer) Element(c *ui.Context, source string, mode untie.Mode) ui.Element {
	if c == nil || !c.Valid() {
		return ui.Element{}
	}
	if r == nil {
		return fallbackElement(c, source, DefaultBaseSize, untie.RoleError)
	}
	size := r.baseSize
	if mode == untie.Display {
		size = r.displaySize
	}
	result, err := r.Render(context.Background(), source, mode, untie.Style{})
	if err != nil || result == nil || result.Box == nil {
		return fallbackElement(c, source, size, untie.RoleError)
	}
	box := result.Box
	bounds := box.Bounds()
	padding := r.draw.Padding
	return ui.Box(c).
		Width(max(1, bounds.Width+2*padding)).
		Height(max(1, bounds.Height+2*padding)).
		Label(source).
		Draw(func(painter *ui.Painter, rect ui.Rect) {
			_ = r.Draw(painter, box, rect)
		})
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
	return r.Element(c, source, untie.Display).FillWidth()
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
	if c == nil || !c.Valid() {
		return ui.Element{}
	}
	if r == nil {
		return fallbackElement(c, source, DefaultBaseSize, untie.RoleError)
	}
	padding := r.draw.Padding
	frameWidth := width - 2*padding
	if !positiveFinite(width) || !positiveFinite(frameWidth) {
		return fallbackElement(c, source, r.displaySize, untie.RoleError)
	}
	result, err := r.Render(context.Background(), source, untie.Display, untie.Style{
		Size:     r.displaySize,
		MaxWidth: frameWidth,
		Align:    untie.Align(r.draw.Align),
	})
	if err != nil || result == nil || result.Box == nil {
		return fallbackElement(c, source, r.displaySize, untie.RoleError)
	}
	box := result.Box
	bounds := box.Bounds()
	options := r.draw
	options.Scale = ScaleNatural
	return ui.Box(c).
		Width(width).
		Height(max(1, bounds.Height+2*padding)).
		Label(source).
		Draw(func(painter *ui.Painter, rect ui.Rect) {
			_ = DrawBox(painter, box, rect, options)
		})
}

// BindMarkdown installs math rendering on options and enables single-dollar
// inline math. Existing non-math callbacks and parser extensions are retained.
func (r *Renderer) BindMarkdown(options *markdown.Options) {
	if r == nil || options == nil {
		return
	}
	options.Extensions |= parser.ExtMath | parser.ExtDollarMath
	options.Math = func(c *ui.Context, source string) {
		r.DisplayMath(c, source)
	}
	options.InlineMath = r.InlineMath
}

// CacheStats reports the current cache size, hard capacity, and hit/miss
// counters.
func (r *Renderer) CacheStats() CacheStats {
	if r == nil {
		return CacheStats{}
	}
	return r.cache.stats()
}

func fallbackElement(c *ui.Context, source string, size float32, role untie.Role) ui.Element {
	return ui.Text(c, source).
		FontSize(size).
		TextColor(PaintColor(c.Theme(), untie.Paint{Role: role}))
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
