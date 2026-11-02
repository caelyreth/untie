package untie

import (
	"context"
	"errors"
	"fmt"

	"github.com/caelyreth/untie/internal/engine"
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
//
// Style.Align is currently inactive: it is range-checked for forward
// compatibility, but the renderer always emits start-origin Box geometry.
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
// reuse or modify its slice after LayoutContext returns.
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
	box, err := e.inner.Layout(ctx, root.Root, expr.mode, style)
	if err != nil {
		return nil, err
	}
	if box == nil {
		return nil, fmt.Errorf("untie: layout returned a nil box")
	}
	if err := box.Validate(); err != nil {
		return nil, fmt.Errorf("untie: invalid layout output: %w", err)
	}
	return box, nil
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
