package layout

import (
	"fmt"

	"github.com/caelyreth/untie/geometry"
	"github.com/caelyreth/untie/ir"
)

// MathSize is the typographic size component of a math style.
type MathSize uint8

const (
	// Display is the size used for standalone equations.
	Display MathSize = iota
	// Text is the size used inside running text.
	Text
	// Script is the size used for first-level scripts.
	Script
	// ScriptScript is the smallest supported size.
	ScriptScript
)

// Style is one of the eight OpenType math styles: four sizes, each normal or
// cramped.
type Style struct {
	Size    MathSize
	Cramped bool
	// SizeScale is an absolute font-size multiplier relative to the root em.
	// Zero means the default multiplier (one). It is multiplied by the
	// display/text/script/script-script style scale at layout time.
	SizeScale geometry.Em
}

// Superscript returns the style for a superscript. Display and text move to
// script; both script sizes move to script-script. Cramping is inherited.
func (s Style) Superscript() Style {
	if s.Size == Display || s.Size == Text {
		s.Size = Script
	} else {
		s.Size = ScriptScript
	}
	return s
}

// Subscript returns the style for a subscript.
func (s Style) Subscript() Style {
	return s.Superscript().WithCramped(true)
}

// Numerator returns the style for a vertical fraction numerator.
func (s Style) Numerator() Style {
	switch s.Size {
	case Display:
		s.Size = Text
	case Text:
		s.Size = Script
	default:
		s.Size = ScriptScript
	}
	return s
}

// Denominator returns the style for a vertical fraction denominator.
func (s Style) Denominator() Style {
	return s.Numerator().WithCramped(true)
}

// WithSize replaces the size while preserving cramping.
func (s Style) WithSize(size MathSize) Style {
	s.Size = size
	return s
}

// WithCramped replaces the cramped flag.
func (s Style) WithCramped(cramped bool) Style {
	s.Cramped = cramped
	return s
}

// Validate checks that the style is one of the eight supported states.
func (s Style) Validate() error {
	if s.Size > ScriptScript {
		return fmt.Errorf("%w: invalid math size %d", ErrInvalidInput, s.Size)
	}
	return nil
}

// String returns a stable diagnostic name.
func (s Style) String() string {
	names := [...]string{"display", "text", "script", "script-script"}
	if s.Size > ScriptScript {
		return "invalid"
	}
	if s.Cramped {
		return names[s.Size] + "/cramped"
	}
	return names[s.Size] + "/normal"
}

func styleFromIR(size ir.MathSize) (MathSize, bool) {
	switch size {
	case ir.SizeInherit:
		return 0, false
	case ir.SizeDisplay:
		return Display, true
	case ir.SizeText:
		return Text, true
	case ir.SizeScript:
		return Script, true
	case ir.SizeScriptScript:
		return ScriptScript, true
	default:
		return 0, false
	}
}
