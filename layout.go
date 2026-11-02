package untie

import (
	"context"
	"errors"
	"fmt"

	internalalt "github.com/caelyreth/untie/internal/alt"
	"github.com/caelyreth/untie/internal/engine"
	"github.com/caelyreth/untie/source"
)

var (
	// ErrInvalidContext reports a nil context.
	ErrInvalidContext = errors.New("untie: nil context")
	// ErrInvalidFonts reports a missing or unusable prepared font set.
	ErrInvalidFonts = errors.New("untie: invalid font set")
	// ErrInvalidStyle reports an invalid host layout style.
	ErrInvalidStyle = errors.New("untie: invalid layout style")
	// ErrInvalidEngine reports a nil or unusable prepared layout engine.
	ErrInvalidEngine = errors.New("untie: invalid layout engine")
)

// Engine is a prepared, concurrency-safe layout environment owned by the
// caller. Reuse it for repeated layouts with the same FontSet and Limits.
//
// An Engine retains its FontSet and is GC-managed; neither value needs a Close
// call. Use LoadFonts and NewEngine once during application setup, then retain
// the Engine for the lifetime of the font configuration.
type Engine struct {
	fonts  *FontSet
	inner  *engine.Engine
	limits Limits
}

// NewEngine prepares an Engine for repeated layouts.
//
// Prefer this API when the application owns the font/cache lifecycle. The
// package-level Layout and LayoutContext convenience functions use a separate
// bounded process-wide cache.
func NewEngine(fonts *FontSet, limits Limits) (prepared *Engine, err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			prepared = nil
			err = fmt.Errorf("%w: engine construction panic: %v", ErrInvalidEngine, recovered)
		}
	}()
	if fonts == nil {
		return nil, ErrInvalidFonts
	}
	effective := limits.WithDefaults()
	inner, err := engine.New(fonts, effective)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrInvalidFonts, err)
	}
	return &Engine{fonts: fonts, inner: inner, limits: effective}, nil
}

// Limits returns the effective resource limits used by the Engine.
func (e *Engine) Limits() Limits {
	if e == nil {
		return Limits{}
	}
	return e.limits
}

// FontKeySHA256 returns the collision-resistant identity of the prepared font
// set used by the Engine.
func (e *Engine) FontKeySHA256() [32]byte {
	if e == nil || e.fonts == nil {
		return [32]byte{}
	}
	return e.fonts.KeySHA256()
}

// LayoutContext lays out an expression with cancellation and resource limits.
// Style.Features is snapshotted for the duration of the call; the caller may
// reuse or modify its slice after LayoutContext returns. Box.Alt is generated
// under the per-field stricter intersection of Engine and expression limits,
// so it may be a bounded fallback that differs from Expr.Alt.
func (e *Engine) LayoutContext(ctx context.Context, expr *Expr, style Style) (*Box, error) {
	if e == nil || e.inner == nil {
		return nil, ErrInvalidEngine
	}
	if ctx == nil {
		return nil, ErrInvalidContext
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if expr == nil {
		return nil, ErrInvalidExpr
	}
	style, err := snapshotLayoutStyle(style)
	if err != nil {
		return nil, err
	}
	root := expr.rootForLayout()
	if root == nil || root.Root == nil {
		return nil, ErrInvalidExpr
	}
	return e.layoutRootContext(ctx, root, expr.mode, style, intersectLimits(e.limits, expr.limits), true)
}

// LayoutFormulaContext lays out canonical IR with cancellation and the
// Engine's configured resource limits. Unlike Expr, formula must be immutable
// for the duration of the call and may be shared after it returns.
//
// This is the advanced boundary for hosts that cache canonical IR or produce
// it through another validated frontend. Parsed expressions should use
// LayoutContext so parser-validated structure can be reused.
func (e *Engine) LayoutFormulaContext(ctx context.Context, formula *Formula, mode Mode, style Style) (*Box, error) {
	if e == nil || e.inner == nil {
		return nil, ErrInvalidEngine
	}
	if ctx == nil {
		return nil, ErrInvalidContext
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if formula == nil || formula.Root == nil {
		return nil, publicLayoutError(fmt.Errorf("%w: nil formula", ErrInvalidInput))
	}
	style, err := snapshotLayoutStyle(style)
	if err != nil {
		return nil, err
	}
	return e.layoutRootContext(ctx, formula, mode, style, e.limits, false)
}

func (e *Engine) layoutRootContext(
	ctx context.Context,
	root *Formula,
	mode Mode,
	style Style,
	altLimits Limits,
	parsed bool,
) (*Box, error) {
	var box *Box
	var err error
	if parsed {
		box, err = e.inner.LayoutParsedFormula(ctx, root, mode, style)
	} else {
		box, err = e.inner.LayoutFormula(ctx, root, mode, style)
	}
	if err != nil {
		return nil, publicLayoutError(err)
	}
	if box == nil {
		return nil, publicLayoutError(fmt.Errorf("%w: layout returned a nil box", ErrInvalidInput))
	}
	if !style.GenerateAlt {
		return box, nil
	}
	alt, altErr := internalalt.GenerateFormulaContext(ctx, root, mode, altLimits)
	if altErr != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return nil, ctxErr
		}
		// Accessibility failure never invalidates otherwise correct geometry.
		// GenerateContext returns a bounded visible fallback for this case.
	}
	if err := alt.Validate(); err != nil {
		return nil, publicLayoutError(fmt.Errorf("%w: invalid alternative text: %w", ErrInvalidInput, err))
	}
	box.Alt = alt
	return box, nil
}

func publicLayoutError(err error) error {
	if err == nil ||
		errors.Is(err, context.Canceled) ||
		errors.Is(err, context.DeadlineExceeded) {
		return err
	}
	var existing *LayoutError
	if errors.As(err, &existing) {
		return err
	}
	kind, ok := publicLayoutErrorKind(err)
	if !ok {
		return err
	}
	r := source.Detached()
	var located interface {
		SourceRange() source.Range
	}
	if errors.As(err, &located) {
		candidate := located.SourceRange()
		if candidate.IsValid() && !candidate.IsDetached() {
			r = candidate
		}
	}
	cause := err
	if located, ok := err.(interface {
		SourceRange() source.Range
		Unwrap() error
	}); ok {
		cause = located.Unwrap()
	}
	return &LayoutError{Kind: kind, Range: r, Err: cause}
}

func publicLayoutErrorKind(err error) (LayoutErrorKind, bool) {
	switch {
	case errors.Is(err, ErrMissingGlyph):
		return LayoutErrorMissingGlyph, true
	case errors.Is(err, ErrInsufficientStretch):
		return LayoutErrorInsufficientStretch, true
	case errors.Is(err, ErrUnsupported):
		return LayoutErrorUnsupported, true
	case errors.Is(err, ErrLimit):
		return LayoutErrorLimit, true
	case errors.Is(err, ErrInvalidInput):
		return LayoutErrorInvalidInput, true
	default:
		return "", false
	}
}

// intersectLimits returns the stricter positive value for every resource
// field. Engine limits govern public layout while expression limits govern the
// parsed tree; derived outputs must honor both contracts.
func intersectLimits(left, right source.Limits) source.Limits {
	left = left.WithDefaults()
	right = right.WithDefaults()
	minPositive := func(a, b int) int {
		if a <= 0 {
			return b
		}
		if b <= 0 || a < b {
			return a
		}
		return b
	}
	minPositiveDimension := func(a, b int64) int64 {
		if a <= 0 {
			return b
		}
		if b <= 0 || a < b {
			return a
		}
		return b
	}
	return source.Limits{
		MaxSourceBytes:     minPositive(left.MaxSourceBytes, right.MaxSourceBytes),
		MaxTokens:          minPositive(left.MaxTokens, right.MaxTokens),
		MaxNesting:         minPositive(left.MaxNesting, right.MaxNesting),
		MaxNodes:           minPositive(left.MaxNodes, right.MaxNodes),
		MaxMacroExpansions: minPositive(left.MaxMacroExpansions, right.MaxMacroExpansions),
		MaxExpandedTokens:  minPositive(left.MaxExpandedTokens, right.MaxExpandedTokens),
		MaxCallDepth:       minPositive(left.MaxCallDepth, right.MaxCallDepth),
		MaxTableCells:      minPositive(left.MaxTableCells, right.MaxTableCells),
		MaxAssemblyRepeats: minPositive(left.MaxAssemblyRepeats, right.MaxAssemblyRepeats),
		MaxTextBytes:       minPositive(left.MaxTextBytes, right.MaxTextBytes),
		MaxOutputItems:     minPositive(left.MaxOutputItems, right.MaxOutputItems),
		MaxPathSegments:    minPositive(left.MaxPathSegments, right.MaxPathSegments),
		MaxDimension:       minPositiveDimension(left.MaxDimension, right.MaxDimension),
	}
}

// Layout lays out an expression with a background context.
func (e *Engine) Layout(expr *Expr, style Style) (*Box, error) {
	return e.LayoutContext(context.Background(), expr, style)
}

// LayoutContext is a convenience wrapper that reuses a bounded eight-entry
// process-wide cache of prepared Engines. The cache is keyed by library
// version, the full FontSet revision, and effective resource limits. Layout
// results are not cached; callers receive an independent Box for every call.
//
// Use NewEngine when the host needs explicit ownership of prepared resources.
func LayoutContext(ctx context.Context, expr *Expr, fonts *FontSet, style Style) (*Box, error) {
	if ctx == nil {
		return nil, ErrInvalidContext
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if expr == nil {
		return nil, ErrInvalidExpr
	}
	if fonts == nil {
		return nil, ErrInvalidFonts
	}
	style, err := snapshotLayoutStyle(style)
	if err != nil {
		return nil, err
	}
	layoutEngine, err := defaultLayoutEngineCache.get(ctx, fonts, expr.limits)
	if err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return nil, ctxErr
		}
		return nil, err
	}
	return layoutEngine.LayoutContext(ctx, expr, style)
}

// Layout lays out an expression with a background context and the bounded
// process-wide Engine cache. It reports preparation, cancellation, and layout
// errors rather than returning a misleading empty result.
func Layout(expr *Expr, fonts *FontSet, style Style) (*Box, error) {
	return LayoutContext(context.Background(), expr, fonts, style)
}

// ClearLayoutEngineCache drops every prepared Engine retained by the
// package-level layout convenience functions. Explicitly owned Engines from
// NewEngine are unaffected. An in-flight convenience build may still finish,
// but it will not be inserted after the clear.
func ClearLayoutEngineCache() {
	defaultLayoutEngineCache.clear()
}

func snapshotLayoutStyle(style Style) (Style, error) {
	if err := engine.ValidateStyle(style); err != nil {
		return Style{}, fmt.Errorf("%w: %v", ErrInvalidStyle, err)
	}
	style.Features = append([]string(nil), style.Features...)
	return style, nil
}
