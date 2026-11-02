package untie

import (
	"fmt"

	internallayout "github.com/caelyreth/untie/internal/layout"
	"github.com/caelyreth/untie/source"
)

// LayoutErrorKind is the stable machine-readable classification of a layout
// failure. Kinds may be added over time, but existing meanings remain stable.
type LayoutErrorKind string

const (
	// LayoutErrorInvalidInput reports malformed canonical layout input or an
	// invalid output invariant.
	LayoutErrorInvalidInput LayoutErrorKind = "invalid-input"
	// LayoutErrorMissingGlyph reports unavailable glyph data.
	LayoutErrorMissingGlyph LayoutErrorKind = "missing-glyph"
	// LayoutErrorUnsupported reports a recognized but unsupported construct.
	LayoutErrorUnsupported LayoutErrorKind = "unsupported"
	// LayoutErrorInsufficientStretch reports a delimiter or accent that the
	// configured font cannot stretch far enough.
	LayoutErrorInsufficientStretch LayoutErrorKind = "insufficient-stretch"
	// LayoutErrorLimit reports an exceeded resource limit.
	LayoutErrorLimit LayoutErrorKind = "limit"
)

// LayoutError is a structured public layout failure.
//
// Kind is stable for programmatic classification. Range identifies the
// originating canonical IR node when one can be attributed; otherwise it is
// source.Detached(). Err preserves the underlying cause and can be inspected
// with errors.Is or errors.As.
type LayoutError struct {
	Kind  LayoutErrorKind
	Range source.Range
	Err   error
}

// Error returns the classification, source location, and underlying cause.
func (e *LayoutError) Error() string {
	if e == nil {
		return "untie: nil layout error"
	}
	location := "detached source"
	if e.Range.IsValid() && !e.Range.IsDetached() {
		location = fmt.Sprintf("bytes [%d,%d)", e.Range.Start, e.Range.End)
	}
	if e.Err == nil {
		return fmt.Sprintf("untie: layout %s at %s", e.Kind, location)
	}
	return fmt.Sprintf("untie: layout %s at %s: %v", e.Kind, location, e.Err)
}

// Unwrap returns the underlying layout cause.
func (e *LayoutError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

// Code returns the stable machine-readable classification.
func (e *LayoutError) Code() LayoutErrorKind {
	if e == nil {
		return ""
	}
	return e.Kind
}

var (
	// ErrInvalidInput reports malformed input at the public layout boundary.
	ErrInvalidInput = internallayout.ErrInvalidInput
	// ErrMissingGlyph reports that the configured font set cannot supply a
	// glyph required by the expression.
	ErrMissingGlyph = internallayout.ErrMissingGlyph
	// ErrUnsupported reports a recognized construct that this release cannot
	// render.
	ErrUnsupported = internallayout.ErrUnsupported
	// ErrInsufficientStretch reports that the font cannot stretch a delimiter
	// or accent far enough for the requested construction.
	ErrInsufficientStretch = internallayout.ErrInsufficientStretch
	// ErrLimit reports that a configured parse, layout, or output limit was
	// exceeded.
	ErrLimit = internallayout.ErrLimit
)
