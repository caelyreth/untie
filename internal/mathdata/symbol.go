package mathdata

// TeXFont identifies the KaTeX symbol source font class.
type TeXFont uint8

const (
	TeXFontMain TeXFont = iota
	TeXFontAMS
)

// String returns the KaTeX source name of f.
func (f TeXFont) String() string {
	switch f {
	case TeXFontMain:
		return "main"
	case TeXFontAMS:
		return "ams"
	default:
		return "invalid"
	}
}

// TeXGroup is KaTeX's parser-level symbol group.
//
// It is intentionally finer than Class: mathord and textord both usually use
// ClassNormal or ClassAlphabetic, but frontends need the original distinction
// when choosing an alphabet.
type TeXGroup uint8

const (
	TeXGroupTextOrd TeXGroup = iota
	TeXGroupMathOrd
	TeXGroupOperator
	TeXGroupBinary
	TeXGroupRelation
	TeXGroupOpening
	TeXGroupClosing
	TeXGroupPunctuation
	TeXGroupInner
	TeXGroupAccent
	TeXGroupSpacing
)

// String returns the KaTeX group name.
func (g TeXGroup) String() string {
	switch g {
	case TeXGroupTextOrd:
		return "textord"
	case TeXGroupMathOrd:
		return "mathord"
	case TeXGroupOperator:
		return "op-token"
	case TeXGroupBinary:
		return "bin"
	case TeXGroupRelation:
		return "rel"
	case TeXGroupOpening:
		return "open"
	case TeXGroupClosing:
		return "close"
	case TeXGroupPunctuation:
		return "punct"
	case TeXGroupInner:
		return "inner"
	case TeXGroupAccent:
		return "accent-token"
	case TeXGroupSpacing:
		return "spacing"
	default:
		return "invalid"
	}
}

// TeXSymbolKind distinguishes public syntax from KaTeX implementation details.
type TeXSymbolKind uint8

const (
	// TeXSymbolPublic is a command that may appear in user TeX.
	TeXSymbolPublic TeXSymbolKind = iota
	// TeXSymbolInternal is a KaTeX implementation command, conventionally
	// prefixed with \@. It must not be offered as user-facing syntax.
	TeXSymbolInternal
)

// String returns a stable name for the symbol kind.
func (k TeXSymbolKind) String() string {
	switch k {
	case TeXSymbolPublic:
		return "public"
	case TeXSymbolInternal:
		return "internal"
	default:
		return "invalid"
	}
}

// TeXSymbol is metadata for a no-argument TeX symbol command.
type TeXSymbol struct {
	Command string
	// Text is portable Unicode text. It is never a KaTeX private-use code point.
	// It is empty for commands with no standalone portable scalar, such as
	// internal composition primitives and zero-width spacing controls.
	Text          string
	Class         Class
	Group         TeXGroup
	Font          TeXFont
	AcceptUnicode bool
	Kind          TeXSymbolKind
	// SourceGlyph is the original KaTeX private-use code point for traceability.
	// It is source metadata, not renderable text; zero means no private-use code.
	SourceGlyph rune
}

// TypstSymbol is one addressable variant of a Typst sym-module symbol.
type TypstSymbol struct {
	Path        string
	Name        string
	Modifier    string
	Value       string
	Deprecation string
}

type negatedRelation struct {
	base    string
	negated string
}

// LookupTeXSymbol returns metadata for a TeX command such as \alpha.
func LookupTeXSymbol(command string) (TeXSymbol, bool) {
	return lookupTeXSymbol(command)
}

// LookupTypstSymbol returns a Typst symbol by its full path, such as arrow.r.
func LookupTypstSymbol(path string) (TypstSymbol, bool) {
	return lookupTypstSymbol(path)
}

// LookupTypstSymbolVariant returns name when modifier is empty, or the
// dot-separated path name.modifier otherwise.
func LookupTypstSymbolVariant(name, modifier string) (TypstSymbol, bool) {
	if modifier == "" {
		return lookupTypstSymbol(name)
	}
	return lookupTypstSymbol(name + "." + modifier)
}

// LookupNegatedRelation returns the precomposed Unicode relation whose visual
// semantics are the base relation plus a negation stroke.
func LookupNegatedRelation(base string) (string, bool) {
	return lookupNegatedRelation(base)
}
