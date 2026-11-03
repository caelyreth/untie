package engine

import (
	"fmt"
	"strconv"
	"strings"
	"unicode"

	"github.com/caelyreth/untie/font"
	"github.com/caelyreth/untie/internal/layout"
	"github.com/caelyreth/untie/internal/mathdata"
	"github.com/caelyreth/untie/ir"
)

var (
	scriptFeatures       = []font.Feature{{Tag: "ssty", Value: 1}}
	scriptScriptFeatures = []font.Feature{{Tag: "ssty", Value: 2}}
)

func styledText(text string, variant ir.Variant, autoMath bool) (string, bool) {
	styled, _, _, ok := styledTextWithMap(text, variant, autoMath)
	return styled, ok
}

// styledTextWithMap returns the transformed runes and, for every transformed
// rune, the index of the source rune that produced it. One source rune may
// produce a base rune plus a variation selector.
func styledTextWithMap(text string, variant ir.Variant, autoMath bool) (string, []int, rune, bool) {
	runes := []rune(text)
	styledRunes := make([]rune, 0, len(runes))
	sourceRunes := make([]int, 0, len(runes))
	for i := 0; i < len(runes); i++ {
		styled, ok := styleRune(runes[i], variant, autoMath)
		if !ok {
			return "", nil, runes[i], false
		}
		produced := []rune(styled)
		for range produced {
			sourceRunes = append(sourceRunes, i)
		}
		styledRunes = append(styledRunes, produced...)
		if i+1 < len(runes) && isVariationSelector(runes[i+1]) {
			if len(produced) == 1 || !isVariationSelector(produced[len(produced)-1]) {
				styledRunes = append(styledRunes, runes[i+1])
				sourceRunes = append(sourceRunes, i+1)
			} else {
				// The styled rune already carries a selector. Attribute that
				// styled selector to the source selector so a combined shaping
				// cluster still spans both original runes.
				sourceRunes[len(sourceRunes)-1] = i + 1
			}
			i++
		}
	}
	return string(styledRunes), sourceRunes, 0, true
}

func styleRune(r rune, variant ir.Variant, autoMath bool) (string, bool) {
	if !isVariantRune(r) || isMathematicalAlphanumeric(r) {
		return string(r), true
	}
	if variant == ir.VariantAuto {
		if !autoMath {
			return string(r), true
		}
		variant = autoVariant(r)
	}
	if variant == ir.VariantNormal {
		return string(r), true
	}
	alpha, ok := alphaVariant(variant)
	if !ok {
		return "", false
	}
	styled, selector, ok := mathdata.VariantRune(r, alpha)
	if !ok {
		return "", false
	}
	if selector == 0 {
		return string(styled), true
	}
	return string([]rune{styled, selector}), true
}

func autoVariant(r rune) ir.Variant {
	if isUpperGreek(r) {
		return ir.VariantNormal
	}
	if isLatin(r) || isLowerGreek(r) {
		return ir.VariantItalic
	}
	return ir.VariantNormal
}

func alphaVariant(variant ir.Variant) (mathdata.AlphaVariant, bool) {
	switch variant {
	case ir.VariantNormal:
		return mathdata.VariantPlain, true
	case ir.VariantItalic:
		return mathdata.VariantItalic, true
	case ir.VariantBold:
		return mathdata.VariantBold, true
	case ir.VariantBoldItalic:
		return mathdata.VariantBoldItalic, true
	case ir.VariantDoubleStruck:
		return mathdata.VariantDoubleStruck, true
	case ir.VariantScript:
		return mathdata.VariantScript, true
	case ir.VariantBoldScript:
		return mathdata.VariantBoldScript, true
	case ir.VariantFraktur:
		return mathdata.VariantFraktur, true
	case ir.VariantBoldFraktur:
		return mathdata.VariantBoldFraktur, true
	case ir.VariantSans:
		return mathdata.VariantSansSerif, true
	case ir.VariantBoldSans:
		return mathdata.VariantSansSerifBold, true
	case ir.VariantMonospace:
		return mathdata.VariantMonospace, true
	default:
		return 0, false
	}
}

func textFeatures(features []string) ([]font.Feature, error) {
	out := make([]font.Feature, 0, len(features))
	for _, feature := range features {
		tag, value, err := parseFeature(feature)
		if err != nil {
			return nil, err
		}
		out = append(out, font.Feature{Tag: tag, Value: value})
	}
	return out, nil
}

func mathFeatures(features []string, style layout.Style) ([]font.Feature, error) {
	if len(features) == 0 {
		switch style.Size {
		case layout.Script:
			return scriptFeatures, nil
		case layout.ScriptScript:
			return scriptScriptFeatures, nil
		default:
			return nil, nil
		}
	}
	requested, err := textFeatures(features)
	if err != nil {
		return nil, err
	}
	out := make([]font.Feature, 0, len(requested)+2)
	dtls := false
	for _, feature := range requested {
		switch feature.Tag {
		case "flac":
			dtls = dtls || feature.Value != 0
		case "dtls":
			dtls = dtls || feature.Value != 0
		case "ssty":
			continue
		default:
			out = append(out, feature)
		}
	}
	if dtls {
		out = append(out, font.Feature{Tag: "dtls", Value: 1})
	}
	switch style.Size {
	case layout.Script:
		out = append(out, font.Feature{Tag: "ssty", Value: 1})
	case layout.ScriptScript:
		out = append(out, font.Feature{Tag: "ssty", Value: 2})
	}
	return out, nil
}

func parseFeature(feature string) (string, uint16, error) {
	tag, rawValue, hasValue := strings.Cut(feature, "=")
	if tag == "" || len(tag) > 4 {
		return "", 0, fmt.Errorf("%w: invalid feature %q", layout.ErrInvalidInput, feature)
	}
	for _, r := range tag {
		if r > unicode.MaxASCII || !unicode.IsLetter(r) && !unicode.IsDigit(r) {
			return "", 0, fmt.Errorf("%w: invalid feature %q", layout.ErrInvalidInput, feature)
		}
	}
	value := uint16(1)
	if hasValue {
		parsed, err := strconv.ParseUint(rawValue, 10, 16)
		if err != nil {
			return "", 0, fmt.Errorf("%w: invalid feature %q", layout.ErrInvalidInput, feature)
		}
		value = uint16(parsed)
	}
	return tag, value, nil
}

func isTextLike(text string) bool {
	for _, r := range text {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			continue
		}
		return false
	}
	return text != ""
}

func isVariationSelector(r rune) bool {
	return r >= 0xFE00 && r <= 0xFE0F
}

func isVariantRune(r rune) bool {
	return isLatin(r) || isUpperGreek(r) || isLowerGreek(r) || r >= '0' && r <= '9'
}

func isLatin(r rune) bool {
	return r >= 'A' && r <= 'Z' || r >= 'a' && r <= 'z'
}

func isUpperGreek(r rune) bool {
	return r >= 0x0391 && r <= 0x03A1 ||
		r >= 0x03A3 && r <= 0x03A9 ||
		r == 0x03F4 || r == 0x2207
}

func isLowerGreek(r rune) bool {
	return r >= 0x03B1 && r <= 0x03C9 ||
		r == 0x2202 || r == 0x03F5 || r == 0x03D1 || r == 0x03F0 ||
		r == 0x03D5 || r == 0x03F1 || r == 0x03D6
}

func isMathematicalAlphanumeric(r rune) bool {
	return r >= 0x1D400 && r <= 0x1D7FF
}
