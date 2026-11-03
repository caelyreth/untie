package untiemygo

import "github.com/caelyreth/untie"

const emptyFormulaLabel = "Math formula"

// AccessibilityMode controls whether standalone formula elements request
// semantic alternative text during layout.
//
// Source text is always used as the element's accessibility value. Automatic
// mode also requests Alt.Speech, Alt.Text, and Alt.Description from Untie.
// Disabled mode skips that extra traversal and falls back to the source as the
// label.
type AccessibilityMode uint8

const (
	// AccessibilityAuto requests semantic alternative text for standalone
	// elements. It is the zero value.
	AccessibilityAuto AccessibilityMode = iota
	// AccessibilityDisabled skips alternative-text generation.
	AccessibilityDisabled
)

func (m AccessibilityMode) validate() bool {
	return m == AccessibilityAuto || m == AccessibilityDisabled
}

// FormulaSemantics is the accessible text and structured semantic description
// derived from a formula source and its optional generated alternative text.
type FormulaSemantics struct {
	// Label names the formula for assistive technology. It prefers spoken
	// alternative text, then plain text, then source.
	Label string
	// Value is the exact source in the selected dialect.
	Value string
	// PlainText is a compact linear form suitable for copy and search.
	PlainText string
	// Description is the source-independent semantic tree generated with Alt.
	// Hosts can map it to their locale without parsing Label or PlainText.
	Description *untie.AltDescription
}

// DescribeFormula returns deterministic accessible text without inspecting
// display geometry. Empty formulas receive a stable non-empty label so they
// remain discoverable by assistive technology.
func DescribeFormula(source string, alt untie.Alt) FormulaSemantics {
	plain := alt.Text
	if plain == "" {
		plain = source
	}
	label := alt.Speech
	if label == "" {
		label = plain
	}
	if label == "" {
		label = emptyFormulaLabel
	}
	return FormulaSemantics{
		Label:       label,
		Value:       source,
		PlainText:   plain,
		Description: alt.Description.Clone(),
	}
}
