// Package typst implements the Typst math frontend.
package typst

import (
	"fmt"

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

// Parse lowers Typst math source into the neutral IR. Malformed source is
// recoverable: the returned tree always contains a drawable fallback for the
// parts that could not be understood.
func Parse(src string, mode ir.Mode, opts Options) (result Result) {
	limits := opts.Limits.WithDefaults()
	result = Result{Root: ir.NewRow(source.NewRange(0, len(src)))}
	defer func() {
		if recovered := recover(); recovered != nil {
			span := source.NewRange(0, len(src))
			text := src
			if text == "" {
				text = "internal Typst frontend failure"
			}
			result = Result{
				Root: ir.NewRow(span, ir.NewError(span, text, source.CodeInternal)),
				Diagnostics: []source.Diagnostic{{
					Range:    span,
					Severity: source.SeverityError,
					Code:     source.CodeInternal,
					Message:  fmt.Sprintf("internal Typst frontend failure: %v", recovered),
				}},
			}
		}
	}()

	root, diagnostics := parseSource(src, mode, limits)
	lowered := lower(root, src, limits)
	result = lowered
	result.Diagnostics = source.NormalizeDiagnostics(append(diagnostics, result.Diagnostics...))
	if opts.Preamble != "" {
		result.Diagnostics = source.NormalizeDiagnostics(append(result.Diagnostics, source.Diagnostic{
			Range:    source.NewRange(0, min(len(src), 1)),
			Severity: source.SeverityWarning,
			Code:     source.CodeUnsupported,
			Message:  "Typst preamble definitions are not supported by the P1 math frontend",
		}))
	}
	if result.Root == nil {
		result.Root = ir.NewRow(source.NewRange(0, len(src)))
	}
	return result
}
