// Package engine connects parsed IR, parsed fonts, and the layout engine.
package engine

import (
	"context"
	"errors"
	"fmt"

	"github.com/caelyreth/untie/display"
	"github.com/caelyreth/untie/font"
	"github.com/caelyreth/untie/internal/layout"
	"github.com/caelyreth/untie/ir"
	"github.com/caelyreth/untie/source"
)

var ErrInvalidInput = errors.New("engine: invalid input")

// Engine is an immutable arrangement of one parsed font set and its layout
// renderer. It is safe for concurrent Layout calls when the FontSet is safe,
// which the font package guarantees.
type Engine struct {
	fonts    *font.FontSet
	renderer *layout.Renderer
}

// New constructs an end-to-end layout engine.
func New(fonts *font.FontSet, limits source.Limits) (*Engine, error) {
	if fonts == nil {
		return nil, fmt.Errorf("%w: nil font set", ErrInvalidInput)
	}
	if err := validateFontSet(fonts); err != nil {
		return nil, err
	}
	effective := limits.WithDefaults()
	adapter, err := newAdapter(fonts, effective)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrInvalidInput, err)
	}
	renderer, err := layout.New(layout.Fonts{
		Glyphs:    adapter,
		Metrics:   adapter,
		Stretch:   adapter,
		Text:      adapter,
		Operators: adapter,
	}, layout.Options{Limits: effective})
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrInvalidInput, err)
	}
	return &Engine{fonts: fonts, renderer: renderer}, nil
}

func validateFontSet(fonts *font.FontSet) error {
	if fonts == nil || fonts.FaceCount() == 0 {
		return fmt.Errorf("%w: empty font set", ErrInvalidInput)
	}
	info, ok := fonts.Face(font.MathFace)
	if !ok || info.UnitsPerEm == 0 || info.GlyphCount == 0 || !info.HasMath {
		return fmt.Errorf("%w: invalid primary math face", ErrInvalidInput)
	}
	if _, ok := fonts.VerticalMetrics(font.MathFace); !ok {
		return fmt.Errorf("%w: math face has no vertical metrics", ErrInvalidInput)
	}
	if _, ok := fonts.MathConstant(font.MathFace, font.AxisHeight); !ok {
		return fmt.Errorf("%w: math face has no axis height", ErrInvalidInput)
	}
	if info, ok := fonts.Face(font.TextFace); ok {
		if info.UnitsPerEm == 0 || info.GlyphCount == 0 {
			return fmt.Errorf("%w: invalid text face", ErrInvalidInput)
		}
	}
	for i := 0; ; i++ {
		ref, ok := font.FallbackFace(i)
		if !ok {
			break
		}
		info, exists := fonts.Face(ref)
		if !exists {
			break
		}
		if info.UnitsPerEm == 0 || info.GlyphCount == 0 {
			return fmt.Errorf("%w: invalid fallback face %d", ErrInvalidInput, i)
		}
	}
	return nil
}

// Layout lays root out with mode and style.
func (e *Engine) Layout(ctx context.Context, root ir.Node, mode ir.Mode, style display.Style) (*display.Box, error) {
	return e.LayoutFormula(ctx, &ir.Formula{Root: root}, mode, style)
}

// LayoutWithRecovery lays out root with explicit missing-glyph recovery.
func (e *Engine) LayoutWithRecovery(
	ctx context.Context,
	root ir.Node,
	mode ir.Mode,
	style display.Style,
	recoverMissingGlyph bool,
) (*display.Box, error) {
	return e.LayoutFormulaWithRecovery(ctx, &ir.Formula{Root: root}, mode, style, recoverMissingGlyph)
}

// LayoutFormula lays out a complete formula, including formula-scoped metadata.
func (e *Engine) LayoutFormula(ctx context.Context, formula *ir.Formula, mode ir.Mode, style display.Style) (*display.Box, error) {
	return e.LayoutFormulaWithRecovery(ctx, formula, mode, style, true)
}

// LayoutFormulaWithRecovery lays out a formula with explicit missing-glyph
// recovery.
func (e *Engine) LayoutFormulaWithRecovery(
	ctx context.Context,
	formula *ir.Formula,
	mode ir.Mode,
	style display.Style,
	recoverMissingGlyph bool,
) (*display.Box, error) {
	return e.layoutFormula(ctx, formula, ir.Stats{}, mode, style, false, recoverMissingGlyph)
}

// LayoutParsedFormula lays out a formula that was structurally validated and
// frozen by the parser boundary. It skips repeated structural validation while
// retaining engine-configured limits and output invariants.
func (e *Engine) LayoutParsedFormula(ctx context.Context, formula *ir.Formula, stats ir.Stats, mode ir.Mode, style display.Style) (*display.Box, error) {
	return e.LayoutParsedFormulaWithRecovery(ctx, formula, stats, mode, style, true)
}

// LayoutParsedFormulaWithRecovery lays out parser-validated formula IR with
// explicit missing-glyph recovery.
func (e *Engine) LayoutParsedFormulaWithRecovery(
	ctx context.Context,
	formula *ir.Formula,
	stats ir.Stats,
	mode ir.Mode,
	style display.Style,
	recoverMissingGlyph bool,
) (*display.Box, error) {
	return e.layoutFormula(ctx, formula, stats, mode, style, true, recoverMissingGlyph)
}

func (e *Engine) layoutFormula(
	ctx context.Context,
	formula *ir.Formula,
	stats ir.Stats,
	mode ir.Mode,
	style display.Style,
	parsed bool,
	recoverMissingGlyph bool,
) (*display.Box, error) {
	if e == nil || e.renderer == nil {
		return nil, fmt.Errorf("%w: nil engine", ErrInvalidInput)
	}
	if ctx == nil {
		return nil, fmt.Errorf("%w: nil context", ErrInvalidInput)
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if err := ValidateStyle(style); err != nil {
		return nil, err
	}
	if formula == nil || formula.Root == nil {
		return nil, fmt.Errorf("%w: nil formula", ErrInvalidInput)
	}
	style.Features = append([]string(nil), style.Features...)
	layoutOptions := layout.LayoutOptions{RecoverMissingGlyph: recoverMissingGlyph}
	if parsed {
		return e.renderer.LayoutParsedFormulaContextOptions(ctx, formula, stats, mode, style, layoutOptions)
	}
	return e.renderer.LayoutFormulaContextOptions(ctx, formula, mode, style, layoutOptions)
}
