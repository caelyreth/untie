package untie

import (
	"context"
	"errors"
	"fmt"

	"github.com/caelyreth/untie/display"
	"github.com/caelyreth/untie/source"
)

const defaultRendererSize float32 = 16

var (
	// ErrInvalidRenderer reports a nil renderer, invalid default dialect, or
	// unusable renderer construction input.
	ErrInvalidRenderer = errors.New("untie: invalid renderer")
)

// RendererOptions configure a reusable high-level renderer.
//
// Limits are shared by parsing, layout, and alternative-text generation.
// DefaultDialect selects the dialect when RenderRequest.Dialect is TeX's zero
// value; a zero DefaultDialect means TeX. DefaultStyle is used when
// RenderRequest.Style is the zero value, with a zero Size defaulting to 16.
// CacheEntries bounds the parsed-expression cache; zero or a negative value
// disables it. Returned display boxes are never cached.
type RendererOptions struct {
	Limits         source.Limits
	DefaultDialect Dialect
	DefaultStyle   display.Style
	CacheEntries   int
	Recovery       RecoveryMode
}

// RenderRequest selects one source-to-display-list operation.
//
// A zero Dialect uses RendererOptions.DefaultDialect. A zero Mode means Inline.
// A fully zero Style uses RendererOptions.DefaultStyle; otherwise a zero Size or
// nil Features field inherits the configured default. Parse options are
// snapshotted before parsing, and their limits are intersected with
// RendererOptions.Limits.
type RenderRequest struct {
	Dialect Dialect
	Mode    Mode
	Style   display.Style
	Parse   ParseOptions
}

// RenderResult is one render attempt.
//
// Box is non-nil when layout succeeds or recoverable problems were rendered
// visibly. Diagnostics contain an independent snapshot of parse and layout
// problems and are returned with fatal layout errors when parsing completed.
// Alt is populated only when the effective style requests alternative-text
// generation.
type RenderResult struct {
	Box         *display.Box
	Diagnostics []source.Diagnostic
	Alt         display.Alt
}

// Renderer is a caller-owned, concurrency-safe high-level parse-and-layout
// facade. It retains one prepared Engine for the lifetime of the renderer and
// may be shared by concurrent rendering workers.
//
// Parsed expressions may be reused through a bounded LRU cache. Every
// successful Render call performs layout and returns a new Box.
type Renderer struct {
	engine         *Engine
	limits         source.Limits
	defaultDialect Dialect
	defaultStyle   display.Style
	cache          *renderExprCache
	recovery       RecoveryMode
}

// NewRenderer prepares a reusable renderer and its Engine.
//
// Engine preparation happens once here. The renderer inherits the engine's
// concurrency and resource-limit guarantees.
func NewRenderer(fonts *FontSet, opts RendererOptions) (*Renderer, error) {
	if fonts == nil {
		return nil, fmt.Errorf("%w: %w", ErrInvalidRenderer, ErrInvalidFonts)
	}
	limits := opts.Limits.WithDefaults()
	dialect := opts.DefaultDialect
	if dialect == 0 {
		dialect = TeX
	}
	if dialect != TeX && dialect != Typst {
		return nil, fmt.Errorf("%w: unknown default dialect %d", ErrInvalidRenderer, dialect)
	}
	if err := opts.Recovery.validate(); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrInvalidRenderer, err)
	}
	style := opts.DefaultStyle
	if style.Size == 0 {
		style.Size = defaultRendererSize
	}
	style, err := snapshotLayoutStyle(style)
	if err != nil {
		return nil, fmt.Errorf("%w: default style: %w", ErrInvalidRenderer, err)
	}
	engine, err := NewEngine(fonts, limits)
	if err != nil {
		return nil, fmt.Errorf("%w: engine: %w", ErrInvalidRenderer, err)
	}
	renderer := &Renderer{
		engine:         engine,
		limits:         limits,
		defaultDialect: dialect,
		defaultStyle:   style,
		recovery:       opts.Recovery,
	}
	if opts.CacheEntries > 0 {
		renderer.cache = newRenderExprCache(opts.CacheEntries)
	}
	return renderer, nil
}

// Render parses, lays out, and optionally generates alternative text for one
// formula.
//
// Recoverable source problems do not fail the call. When layout fails after
// parsing, Render returns a non-nil result containing diagnostics alongside the
// error. Context cancellation returns nil when observed before parsing and a
// diagnostic-bearing result when observed during layout.
func (r *Renderer) Render(
	ctx context.Context,
	src string,
	request RenderRequest,
) (*RenderResult, error) {
	if r == nil || r.engine == nil {
		return nil, ErrInvalidRenderer
	}
	if ctx == nil {
		return nil, ErrInvalidContext
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	dialect := request.Dialect
	if dialect == 0 {
		dialect = r.defaultDialect
	}
	mode := request.Mode
	if mode == 0 {
		mode = Inline
	}
	parseOptions := r.effectiveParseOptions(request.Parse)
	style, err := r.effectiveStyle(request.Style)
	if err != nil {
		return nil, err
	}
	expr, err := r.parse(ctx, src, dialect, mode, parseOptions)
	if err != nil {
		return nil, err
	}
	return r.renderParsed(ctx, src, expr, style)
}

func (r *Renderer) renderParsed(
	ctx context.Context,
	src string,
	expr *Expr,
	style display.Style,
) (*RenderResult, error) {
	result := &RenderResult{Diagnostics: expr.Diagnostics()}
	box, err := r.engine.LayoutContextWithOptions(ctx, expr, style, LayoutOptions{
		Recovery: r.recovery,
	})
	if err != nil {
		if diagnostic, ok := diagnosticForLayoutError(err); ok {
			result.Diagnostics = source.NormalizeDiagnosticsFor(
				append(result.Diagnostics, diagnostic),
				len(src),
			)
		} else if diagnostic, ok := diagnosticForContextError(err); ok {
			result.Diagnostics = source.NormalizeDiagnosticsFor(
				append(result.Diagnostics, diagnostic),
				len(src),
			)
		}
		return result, err
	}
	result.Box = box
	result.Alt = box.Alt.Clone()
	result.Diagnostics = source.NormalizeDiagnosticsFor(
		append(result.Diagnostics, box.Diagnostics...),
		len(src),
	)
	return result, nil
}

func diagnosticForContextError(err error) (source.Diagnostic, bool) {
	if !errors.Is(err, context.Canceled) && !errors.Is(err, context.DeadlineExceeded) {
		return source.Diagnostic{}, false
	}
	return source.Diagnostic{
		Range:    source.NewRange(0, 0),
		Severity: source.SeverityError,
		Code:     source.CodeCanceled,
		Message:  err.Error(),
	}, true
}

func diagnosticForLayoutError(err error) (source.Diagnostic, bool) {
	var layoutErr *LayoutError
	if !errors.As(err, &layoutErr) {
		return source.Diagnostic{}, false
	}
	code := source.CodeInternal
	switch layoutErr.Kind {
	case LayoutErrorMissingGlyph:
		code = source.CodeMissingGlyph
	case LayoutErrorUnsupported, LayoutErrorInsufficientStretch:
		code = source.CodeUnsupported
	case LayoutErrorLimit:
		code = source.CodeLimitExceeded
	case LayoutErrorInvalidInput:
		code = source.CodeInvalidValue
	}
	return source.Diagnostic{
		Range:    layoutErr.Range,
		Severity: source.SeverityError,
		Code:     code,
		Message:  layoutErr.Error(),
	}, true
}

func (r *Renderer) parse(
	ctx context.Context,
	src string,
	dialect Dialect,
	mode Mode,
	opts ParseOptions,
) (*Expr, error) {
	var key renderExprCacheKey
	if r.cache != nil {
		key = makeRenderExprCacheKey(src, dialect, mode, opts)
		if expr, ok := r.cache.get(key); ok {
			return expr, nil
		}
	}
	expr, err := ParseContext(ctx, src, dialect, mode, opts)
	if err != nil {
		return nil, err
	}
	if r.cache != nil {
		r.cache.put(key, expr)
	}
	return expr, nil
}

func (r *Renderer) effectiveParseOptions(opts ParseOptions) ParseOptions {
	opts = snapshotParseOptions(opts)
	opts.Limits = intersectLimits(r.limits, opts.Limits)
	return opts
}

func (r *Renderer) effectiveStyle(style display.Style) (display.Style, error) {
	if isZeroRenderStyle(style) {
		style = r.defaultStyle
	} else if style.Size == 0 {
		style.Size = r.defaultStyle.Size
	}
	if style.Features == nil {
		style.Features = append([]string(nil), r.defaultStyle.Features...)
	}
	return snapshotLayoutStyle(style)
}

func isZeroRenderStyle(style display.Style) bool {
	return style.Size == 0 &&
		style.MaxWidth == 0 &&
		style.Align == display.AlignStart &&
		style.ScriptLevel == 0 &&
		style.Features == nil &&
		!style.GlyphRefs &&
		!style.GenerateAlt &&
		style.ColorPolicy == (display.ColorPolicy{})
}
