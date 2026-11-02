// Package typst implements the Typst math frontend.
package typst

import (
	"github.com/caelyreth/untie/ir"
	"github.com/caelyreth/untie/source"
)

// Options configure Typst parsing and its restricted evaluator.
type Options struct {
	Preamble string
	Limits   source.Limits
}

// Result is a recoverable parse result.
type Result struct {
	Root        ir.Node
	Diagnostics []source.Diagnostic
}

// Parse lowers Typst math source into the neutral IR.
//
// This skeleton implementation reports that the frontend is not installed yet.
// The Typst work package owns the replacement implementation.
func Parse(src string, mode ir.Mode, opts Options) Result {
	span := source.NewRange(0, len(src))
	if src == "" {
		return Result{Root: &ir.Row{Base: ir.NewBase(ir.KindRow, span)}}
	}
	return Result{
		Root: &ir.Row{
			Base:  ir.NewBase(ir.KindRow, span),
			Items: []ir.Node{ir.NewError(span, src, source.CodeUnsupported)},
		},
		Diagnostics: []source.Diagnostic{{
			Range:    span,
			Severity: source.SeverityError,
			Code:     source.CodeUnsupported,
			Message:  "Typst frontend is not installed",
		}},
	}
}
