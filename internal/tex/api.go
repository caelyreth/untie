// Package tex implements the TeX math frontend.
package tex

import (
	"fmt"

	"github.com/caelyreth/untie/ir"
	"github.com/caelyreth/untie/source"
)

// Macro is a host-provided TeX macro.
//
// Args is the number of undelimited arguments, from zero through nine.
type Macro struct {
	Body string
	Args int
}

// Options configure TeX parsing. Map keys in Macros may be supplied with or
// without the leading backslash.
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
// Malformed input produces diagnostics and recoverable IR. Parse only panics
// if an internal invariant is broken; the public boundary converts such a
// panic into a diagnostic and a drawable fallback.
func Parse(src string, mode ir.Mode, opts Options) (result Result) {
	limits := opts.Limits.WithDefaults()
	rootSpan := source.NewRange(0, len(src))

	defer func() {
		if recovered := recover(); recovered != nil {
			fallbackText := src
			if fallbackText == "" {
				fallbackText = "TeX frontend internal error"
			}
			result = fallbackResult(
				rootSpan,
				fallbackText,
				source.CodeInternal,
				fmt.Sprintf("TeX frontend invariant failure: %v", recovered),
				limits,
			)
		}
	}()

	if len(src) > limits.MaxSourceBytes {
		return fallbackResult(rootSpan, src, source.CodeLimitExceeded, "source exceeds configured byte limit", limits)
	}

	p := newParser(src, mode, opts.Macros, limits)
	root := p.parse()
	return Result{
		Root:        root,
		Diagnostics: source.NormalizeDiagnostics(p.diagnostics),
	}
}

func fallbackResult(
	span source.Range,
	text string,
	code source.Code,
	message string,
	limits source.Limits,
) Result {
	limits = limits.WithDefaults()
	diagnostic := source.Diagnostic{
		Range:    span,
		Severity: source.SeverityError,
		Code:     code,
		Message:  message,
	}
	if limits.MaxNodes < 2 {
		return Result{
			Root:        ir.NewPlaceholder(span, "TeX frontend error"),
			Diagnostics: []source.Diagnostic{diagnostic},
		}
	}
	return Result{
		Root:        ir.NewRow(span, ir.NewError(span, text, code)),
		Diagnostics: []source.Diagnostic{diagnostic},
	}
}
