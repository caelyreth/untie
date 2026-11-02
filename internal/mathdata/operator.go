package mathdata

// Operator describes a named, no-argument math operator.
//
// Name includes the leading backslash for TeX operators and has no prefix for
// Typst. Text is the upright operator text for named text operators; it is
// empty for TeX symbol operators whose glyph is resolved through the font.
type Operator struct {
	Name   string
	Text   string
	Limits bool
	Symbol bool
}

// LookupTeXOperator returns metadata for a TeX operator such as \sin or \sum.
func LookupTeXOperator(name string) (Operator, bool) {
	return lookupTeXOperator(name)
}

// LookupTypstOperator returns metadata for a Typst operator such as sin or lim.
func LookupTypstOperator(name string) (Operator, bool) {
	return lookupTypstOperator(name)
}
