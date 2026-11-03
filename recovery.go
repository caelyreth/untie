package untie

import "fmt"

// RecoveryMode controls whether layout may replace recoverable font failures
// with visible output.
//
// The zero value is RecoveryVisible so editor-facing rendering remains useful
// while a formula is incomplete. RecoveryStrict is intended for conformance
// tests, validation tooling, and hosts that require a complete font match.
type RecoveryMode uint8

const (
	// RecoveryVisible renders recoverable missing glyphs as error-colored
	// placeholder boxes and reports them through diagnostics.
	RecoveryVisible RecoveryMode = iota
	// RecoveryStrict reports recoverable font failures as layout errors.
	RecoveryStrict
)

func (m RecoveryMode) validate() error {
	if m > RecoveryStrict {
		return fmt.Errorf("%w: unknown recovery mode %d", ErrInvalidRenderer, m)
	}
	return nil
}

// LayoutOptions controls one low-level Engine layout.
type LayoutOptions struct {
	Recovery RecoveryMode
}
