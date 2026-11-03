package untiemygo

import "github.com/caelyreth/untie"

const emptyFormulaLabel = "Math formula"

// AccessibilityMode controls whether standalone formula elements request
// semantic alternative text during layout.
//
// Source text is always used as the element's accessibility value. Automatic
// mode also requests Alt.Speech and Alt.Text from Untie and uses the spoken
// form as the element label. Disabled mode skips that extra traversal and
// falls back to the source as the label.
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

// FormulaSemantics is the accessible text derived from a formula source and
// its optional generated alternative text.
type FormulaSemantics struct {
	// Label names the formula for assistive technology. It prefers spoken
	// alternative text, then plain text, then source.
	Label string
	// Value is the exact source in the selected dialect.
	Value string
	// PlainText is a compact linear form suitable for copy and search.
	PlainText string
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
		Label:     label,
		Value:     source,
		PlainText: plain,
	}
}
