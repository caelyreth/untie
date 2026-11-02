package mathdata

import (
	"sort"
	"strings"
	"unicode/utf8"
)

// AlphaVariant is a Unicode mathematical alphanumeric style.
//
// Calligraphic and Script are distinct because fonts use variation selectors
// to distinguish chancery from roundhand forms. Bold variants are distinct for
// the same reason.
type AlphaVariant uint8

const (
	VariantPlain AlphaVariant = iota
	VariantBold
	VariantItalic
	VariantBoldItalic
	VariantDoubleStruck
	VariantScript
	VariantBoldScript
	VariantFraktur
	VariantBoldFraktur
	VariantSansSerif
	VariantSansSerifBold
	VariantSansSerifItalic
	VariantSansSerifBoldItalic
	VariantMonospace
	VariantCalligraphic
	VariantBoldCalligraphic
)

// String returns the Typst-facing name of v.
func (v AlphaVariant) String() string {
	switch v {
	case VariantPlain:
		return "plain"
	case VariantBold:
		return "bold"
	case VariantItalic:
		return "italic"
	case VariantBoldItalic:
		return "bold-italic"
	case VariantDoubleStruck:
		return "double-struck"
	case VariantScript:
		return "roundhand"
	case VariantBoldScript:
		return "bold-roundhand"
	case VariantFraktur:
		return "fraktur"
	case VariantBoldFraktur:
		return "bold-fraktur"
	case VariantSansSerif:
		return "sans-serif"
	case VariantSansSerifBold:
		return "sans-serif-bold"
	case VariantSansSerifItalic:
		return "sans-serif-italic"
	case VariantSansSerifBoldItalic:
		return "sans-serif-bold-italic"
	case VariantMonospace:
		return "monospace"
	case VariantCalligraphic:
		return "chancery"
	case VariantBoldCalligraphic:
		return "bold-chancery"
	default:
		return "invalid"
	}
}

// VariantRune returns the Unicode mathematical alphanumeric form of r in v.
//
// A non-empty selector identifies a variation selector that must accompany the
// returned rune. ok is false when Unicode has no direct mapping for r in v; in
// that case the first return value is empty.
func VariantRune(r rune, v AlphaVariant) (styled rune, selector rune, ok bool) {
	if v >= AlphaVariant(len(alphaVariantData)) {
		return 0, 0, false
	}
	if v == VariantPlain {
		return r, 0, true
	}

	if mapped, found := variantException(v, r); found {
		info := alphaVariantData[v]
		if info.selector != 0 && isLatin(r) {
			return mapped, info.selector, true
		}
		return mapped, 0, true
	}

	info := alphaVariantData[v]
	switch {
	case info.latin != 0 && r >= 'A' && r <= 'Z':
		return info.latin + r - 'A', info.selector, true
	case info.latin != 0 && r >= 'a' && r <= 'z':
		return info.latin + 26 + r - 'a', info.selector, true
	case info.digits != 0 && r >= '0' && r <= '9':
		return info.digits + r - '0', 0, true
	case info.greek != 0 && (isUpperGreek(r) || isLowerGreek(r)):
		return info.greek + greekIndex(r), 0, true
	default:
		return 0, 0, false
	}
}

// ApplyVariant styles r with v.
//
// It returns the original rune and true for VariantPlain. If no direct mapping
// exists, it returns the original rune and false so callers can apply their
// documented fallback policy.
func ApplyVariant(r rune, v AlphaVariant) (string, bool) {
	styled, selector, ok := VariantRune(r, v)
	if !ok {
		return string(r), false
	}
	if selector == 0 {
		return string(styled), true
	}
	var b strings.Builder
	b.Grow(utf8.UTFMax * 2)
	b.WriteRune(styled)
	b.WriteRune(selector)
	return b.String(), true
}

// ApplyVariantString maps every rune in s independently.
//
// Unmapped runes are preserved unchanged. The returned bool reports whether
// every rune had a direct mapping.
func ApplyVariantString(s string, v AlphaVariant) (string, bool) {
	if v == VariantPlain {
		return s, true
	}
	var b strings.Builder
	b.Grow(len(s))
	all := true
	for _, r := range s {
		styled, selector, ok := VariantRune(r, v)
		if !ok {
			b.WriteRune(r)
			all = false
			continue
		}
		b.WriteRune(styled)
		if selector != 0 {
			b.WriteRune(selector)
		}
	}
	return b.String(), all
}

func variantException(v AlphaVariant, r rune) (rune, bool) {
	i := sort.Search(len(alphaVariantExceptions), func(i int) bool {
		e := alphaVariantExceptions[i]
		return e.variant > v || (e.variant == v && e.input >= r)
	})
	if i < len(alphaVariantExceptions) {
		e := alphaVariantExceptions[i]
		if e.variant == v && e.input == r {
			return e.output, true
		}
	}
	return 0, false
}

func greekIndex(r rune) rune {
	if isUpperGreek(r) && r >= 0x0391 && r <= 0x03a9 {
		return r - 0x0391
	}
	if isLowerGreek(r) && r >= 0x03b1 && r <= 0x03c9 {
		return 26 + r - 0x03b1
	}
	i := sort.Search(len(greekVariantExtras), func(i int) bool {
		return greekVariantExtras[i].input >= r
	})
	if i < len(greekVariantExtras) && greekVariantExtras[i].input == r {
		return greekVariantExtras[i].offset
	}
	return 0
}

func isLatin(r rune) bool {
	return r >= 'A' && r <= 'Z' || r >= 'a' && r <= 'z'
}

func isUpperGreek(r rune) bool {
	return r >= 0x0391 && r <= 0x03a1 ||
		r >= 0x03a3 && r <= 0x03a9 ||
		r == 0x03f4 || r == 0x2207
}

func isLowerGreek(r rune) bool {
	return r >= 0x03b1 && r <= 0x03c9 ||
		r == 0x2202 || r == 0x03f5 || r == 0x03d1 || r == 0x03f0 ||
		r == 0x03d5 || r == 0x03f1 || r == 0x03d6
}
