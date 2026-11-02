package mathdata

import "sort"

// Class is a Unicode math character class as used by OpenType MATH layout.
//
// Numeric values are stable and intentionally match the order of Typst's
// unicode_math_class::MathClass enum.
type Class uint8

const (
	ClassNormal Class = iota
	ClassAlphabetic
	ClassBinary
	ClassClosing
	ClassDiacritic
	ClassFence
	ClassGlyphPart
	ClassLarge
	ClassOpening
	ClassPunctuation
	ClassRelation
	ClassSpace
	ClassUnary
	ClassVary
	ClassSpecial
)

// String returns the Typst name of c.
func (c Class) String() string {
	switch c {
	case ClassNormal:
		return "normal"
	case ClassAlphabetic:
		return "alphabetic"
	case ClassBinary:
		return "binary"
	case ClassClosing:
		return "closing"
	case ClassDiacritic:
		return "diacritic"
	case ClassFence:
		return "fence"
	case ClassGlyphPart:
		return "glyph-part"
	case ClassLarge:
		return "large"
	case ClassOpening:
		return "opening"
	case ClassPunctuation:
		return "punctuation"
	case ClassRelation:
		return "relation"
	case ClassSpace:
		return "space"
	case ClassUnary:
		return "unary"
	case ClassVary:
		return "vary"
	case ClassSpecial:
		return "special"
	default:
		return "invalid"
	}
}

// UnicodeClass returns the Unicode MathClass.txt class for r.
//
// The lookup table is generated from unicode-math-class 0.1.0 as transcribed
// by the pinned Typlet checkout. Characters absent from MathClass.txt return
// false.
func UnicodeClass(r rune) (Class, bool) {
	return lookupUnicodeClass(r)
}

// DefaultClass returns the math class Typst uses by default for r.
//
// It is UnicodeClass with Typst 0.15.1's documented overrides applied. Frontend
// code that needs raw Unicode data should call UnicodeClass instead.
func DefaultClass(r rune) (Class, bool) {
	i := sort.Search(len(defaultClassOverrides), func(i int) bool {
		return defaultClassOverrides[i].rune >= r
	})
	if i < len(defaultClassOverrides) && defaultClassOverrides[i].rune == r {
		return defaultClassOverrides[i].class, true
	}
	return UnicodeClass(r)
}
