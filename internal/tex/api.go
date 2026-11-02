// Package tex implements the TeX math frontend.
package tex

import (
	"github.com/caelyreth/untie/ir"
	"github.com/caelyreth/untie/source"
)

// Macro is a host-provided TeX macro.
type Macro struct {
	Body string
	Args int
}

// Options configure TeX parsing.
type Options struct {
	Macros map[string]Macro
	Limits source.Limits
}

// Result is a recoverable parse result.
type Result struct {
	Root        ir.Node
	Diagnostics []source.Diagnostic
}

// Parse lowers TeX source into the neutral IR.
//
// This skeleton implementation deliberately reports that the frontend is not
// installed yet. The TeX work package owns the replacement implementation.
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
			Message:  "TeX frontend is not installed",
		}},
	}
}
