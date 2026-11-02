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
	adapter := newAdapter(fonts)
	renderer, err := layout.New(layout.Fonts{
		Glyphs:  adapter,
		Metrics: adapter,
		Stretch: adapter,
		Text:    adapter,
	}, layout.Options{Limits: limits})
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidInput, err)
	}
	return &Engine{fonts: fonts, renderer: renderer}, nil
}

// Layout lays root out with mode and style.
func (e *Engine) Layout(ctx context.Context, root ir.Node, mode ir.Mode, style display.Style) (*display.Box, error) {
	if e == nil || e.renderer == nil {
		return nil, fmt.Errorf("%w: nil engine", ErrInvalidInput)
	}
	return e.renderer.LayoutContext(ctx, root, mode, style)
}
