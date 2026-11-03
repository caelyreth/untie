package engine

import (
	"fmt"
	"math"

	"github.com/caelyreth/untie/display"
)

const MaxStyleFeatures = 64

// ValidateStyle checks layout request values before any engine or provider is
// entered. Feature syntax is shared with shaping so validation cannot drift.
func ValidateStyle(style display.Style) error {
	if style.Size <= 0 || math.IsNaN(float64(style.Size)) || math.IsInf(float64(style.Size), 0) {
		return fmt.Errorf("%w: style size must be finite and positive", ErrInvalidInput)
	}
	if style.MaxWidth < 0 || math.IsNaN(float64(style.MaxWidth)) || math.IsInf(float64(style.MaxWidth), 0) {
		return fmt.Errorf("%w: max width must be finite and non-negative", ErrInvalidInput)
	}
	if style.Align > display.AlignEnd {
		return fmt.Errorf("%w: unknown alignment %d", ErrInvalidInput, style.Align)
	}
	if style.ScriptLevel < 0 {
		return fmt.Errorf("%w: negative script level", ErrInvalidInput)
	}
	if len(style.Features) > MaxStyleFeatures {
		return fmt.Errorf("%w: too many style features: %d", ErrInvalidInput, len(style.Features))
	}
	if err := style.ColorPolicy.Validate(); err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidInput, err)
	}
	if _, err := textFeatures(style.Features); err != nil {
		return err
	}
	return nil
}
