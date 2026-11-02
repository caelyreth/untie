package untie

import internallayout "github.com/caelyreth/untie/internal/layout"

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
