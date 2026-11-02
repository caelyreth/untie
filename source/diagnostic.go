package source

import (
	"cmp"
	"slices"
)

// Severity is the importance of a diagnostic.
type Severity uint8

const (
	SeverityError Severity = iota
	SeverityWarning
	SeverityInfo
)

// Code is a stable diagnostic identifier. Codes may be added over time, but
// existing semantic meanings should remain stable.
type Code string

const (
	CodeSyntax             Code = "syntax"
	CodeUnknownCommand     Code = "unknown-command"
	CodeUnknownSymbol      Code = "unknown-symbol"
	CodeMissingArgument    Code = "missing-argument"
	CodeUnclosedGroup      Code = "unclosed-group"
	CodeUnexpectedToken    Code = "unexpected-token"
	CodeUnsupported        Code = "unsupported"
	CodeInvalidValue       Code = "invalid-value"
	CodeInvalidEnvironment Code = "invalid-environment"
	CodeUnknownVariable    Code = "unknown-variable"
	CodeMissingGlyph       Code = "missing-glyph"
	CodeTextUnsupported    Code = "text-unsupported"
	CodeLimitExceeded      Code = "limit-exceeded"
	CodeCanceled           Code = "canceled"
	CodeInternal           Code = "internal"
	CodeDeprecated         Code = "deprecated"
)

// Diagnostic describes a recoverable source problem.
type Diagnostic struct {
	Range    Range
	Severity Severity
	Code     Code
	Message  string
	Args     []string
	Hints    []string
}

// Error returns the English diagnostic message.
func (d Diagnostic) Error() string {
	if d.Code == "" {
		return d.Message
	}
	return string(d.Code) + ": " + d.Message
}

// Clone returns an independent copy.
func (d Diagnostic) Clone() Diagnostic {
	d.Args = slices.Clone(d.Args)
	d.Hints = slices.Clone(d.Hints)
	return d
}

// NormalizeDiagnostics filters invalid ranges, copies the slice, and returns a
// deterministic ordering by source position, severity, code, and message.
func NormalizeDiagnostics(in []Diagnostic) []Diagnostic {
	if len(in) == 0 {
		return nil
	}
	out := make([]Diagnostic, 0, len(in))
	for _, d := range in {
		if !d.Range.IsValid() {
			continue
		}
		out = append(out, d.Clone())
	}
	slices.SortStableFunc(out, func(a, b Diagnostic) int {
		if c := cmp.Compare(a.Range.Start, b.Range.Start); c != 0 {
			return c
		}
		if c := cmp.Compare(a.Range.End, b.Range.End); c != 0 {
			return c
		}
		if c := cmp.Compare(a.Severity, b.Severity); c != 0 {
			return c
		}
		if c := cmp.Compare(a.Code, b.Code); c != 0 {
			return c
		}
		return cmp.Compare(a.Message, b.Message)
	})
	return out
}
