package mathml

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/caelyreth/untie/ir"
	"github.com/caelyreth/untie/source"
)

const (
	attrNodeID        = "data-untie-node-id"
	attrKind          = "data-untie-kind"
	attrVersion       = "data-untie-version"
	attrClass         = "data-untie-class"
	attrBaseSize      = "data-untie-node-size"
	attrBaseCramped   = "data-untie-node-cramped"
	attrStyleSize     = "data-untie-style-size"
	attrStyleCramped  = "data-untie-style-cramped"
	attrLimits        = "data-untie-limits"
	attrPaintRole     = "data-untie-paint-role"
	attrIgnoreSpace   = "data-untie-ignore-as-space"
	attrAltText       = "data-untie-alt-text"
	attrSuppressed    = "data-untie-suppressed-tags"
	attrSpaceType     = "data-untie-space-type"
	attrSpaceAmount   = "data-untie-space-amount"
	attrSpaceWeak     = "data-untie-space-weak"
	attrFont          = "data-untie-font"
	attrTracking      = "data-untie-tracking"
	attrFractionStyle = "data-untie-fraction-style"
	attrFractionRole  = "data-untie-fraction-role"
	attrFractionPad   = "data-untie-fraction-padding"
	attrScriptRole    = "data-untie-script-role"
	attrScriptLayout  = "data-untie-script-layout"
	attrBalanced      = "data-untie-balanced"
	attrHasOpen       = "data-untie-has-open"
	attrHasClose      = "data-untie-has-close"
	attrSizingKind    = "data-untie-sizing-kind"
	attrSizingValue   = "data-untie-sizing-value"
	attrPosition      = "data-untie-position"
	attrStretchy      = "data-untie-stretchy"
	attrNaturalFit    = "data-untie-natural-fit"
	attrDotless       = "data-untie-dotless"
	attrExactWidth    = "data-untie-exact-frame-width"
	attrThickness     = "data-untie-thickness"
	attrGap           = "data-untie-gap"
	attrCross         = "data-untie-cross"
	attrInverted      = "data-untie-inverted"
	attrCancelRole    = "data-untie-cancel-role"
	attrTableRole     = "data-untie-table-role"
	attrRowGap        = "data-untie-row-gap"
	attrColumnGap     = "data-untie-column-gap"
	attrHSeparators   = "data-untie-horizontal-lines"
	attrVSeparators   = "data-untie-vertical-lines"
	attrCentered      = "data-untie-centered"
	attrSetSize       = "data-untie-set-size"
	attrSetCramped    = "data-untie-set-cramped"
	attrStyleWrapper  = "data-untie-style-wrapper"
	attrPaddingTop    = "data-untie-padding-top"
	attrPaddingRight  = "data-untie-padding-right"
	attrPaddingBottom = "data-untie-padding-bottom"
	attrPaddingLeft   = "data-untie-padding-left"
	attrFill          = "data-untie-fill"
	attrHasFill       = "data-untie-has-fill"
	attrBorder        = "data-untie-border"
	attrHasBorder     = "data-untie-has-border"
	attrBorderWidth   = "data-untie-border-width"
	attrRadius        = "data-untie-radius"
	attrWidthExtent   = "data-untie-width-extent"
	attrHeightExtent  = "data-untie-height-extent"
	attrDepthExtent   = "data-untie-depth-extent"
	attrErrorCode     = "data-untie-error-code"
	attrReason        = "data-untie-reason"
	attrBreakKind     = "data-untie-break-kind"
	attrTagLine       = "data-untie-tag-line"
	attrTagManual     = "data-untie-tag-manual"
	attrTagSuppressed = "data-untie-tag-suppressed"
	attrTagStarred    = "data-untie-tag-starred"
)

var classNames = [...]string{
	"normal", "alphabetic", "binary", "relation", "opening", "closing",
	"fence", "punctuation", "large", "unary", "varying", "inner",
	"diacritic", "space", "special",
}

var sizeNames = [...]string{"inherit", "display", "text", "script", "script-script"}

var limitsNames = [...]string{"auto", "always", "never", "display"}

var variantNames = map[ir.Variant]string{
	ir.VariantNormal:       "normal",
	ir.VariantItalic:       "italic",
	ir.VariantBold:         "bold",
	ir.VariantBoldItalic:   "bold-italic",
	ir.VariantDoubleStruck: "double-struck",
	ir.VariantScript:       "script",
	ir.VariantBoldScript:   "bold-script",
	ir.VariantFraktur:      "fraktur",
	ir.VariantBoldFraktur:  "bold-fraktur",
	ir.VariantSans:         "sans-serif",
	ir.VariantBoldSans:     "bold-sans-serif",
	ir.VariantMonospace:    "monospace",
}

var variantsByName = reverseVariantNames()

var paintRoleNames = [...]string{"ink", "error", "placeholder"}

var spaceTypeNames = [...]string{"explicit", "interword", "ignored"}

var fontNames = [...]string{"math", "text"}

var fractionStyleNames = [...]string{"vertical", "no-bar", "skewed", "horizontal"}

var fractionRoleNames = [...]string{"fraction", "binomial", "stacked"}

var scriptRoleNames = [...]string{"ordinary", "limits", "overlay", "brace-label"}

var cancelRoleNames = [...]string{"strike", "negation"}

var tableRoleNames = [...]string{"matrix", "array", "cases", "vector"}

var breakKindNames = [...]string{"relation", "binary", "explicit"}

var sizingKindNames = [...]string{"content", "relative", "absolute"}

func reverseVariantNames() map[string]ir.Variant {
	out := make(map[string]ir.Variant, len(variantNames))
	for variant, name := range variantNames {
		out[name] = variant
	}
	return out
}

func enumName[T ~uint8](names []string, value T) (string, bool) {
	if int(value) < 0 || int(value) >= len(names) {
		return "", false
	}
	return names[value], true
}

func enumOrInvalid[T ~uint8](names []string, value T) string {
	name, ok := enumName(names, value)
	if !ok {
		return "invalid"
	}
	return name
}

func enumValue[T ~uint8](names []string, value string) (T, bool) {
	for i, name := range names {
		if name == value {
			return T(i), true
		}
	}
	return 0, false
}

func formatLength(value ir.Length) (string, error) {
	var unit string
	switch value.Unit {
	case ir.LengthZero:
		if value.Value != 0 {
			return "", fmt.Errorf("%w: zero-unit length has value %d", ErrInvalidFormula, value.Value)
		}
		return "0", nil
	case ir.LengthEm:
		unit = "em"
	case ir.LengthEx:
		unit = "ex"
	case ir.LengthMu:
		unit = "mu"
	case ir.LengthPt:
		unit = "pt"
	case ir.LengthDip:
		unit = "px"
	case ir.LengthPercent:
		unit = "%"
	default:
		return "", fmt.Errorf("%w: unknown length unit %d", ErrInvalidFormula, value.Unit)
	}
	number := strconv.FormatFloat(float64(value.Value)/1_000_000, 'f', 6, 64)
	number = strings.TrimRight(strings.TrimRight(number, "0"), ".")
	if number == "-0" || number == "" {
		number = "0"
	}
	return number + unit, nil
}

func parseLength(text string) (ir.Length, error) {
	text = strings.TrimSpace(text)
	if text == "0" || text == "+0" || text == "-0" {
		return ir.Length{}, nil
	}
	type suffix struct {
		name string
		unit ir.LengthUnit
	}
	units := [...]suffix{
		{"em", ir.LengthEm},
		{"ex", ir.LengthEx},
		{"mu", ir.LengthMu},
		{"pt", ir.LengthPt},
		{"px", ir.LengthDip},
		{"%", ir.LengthPercent},
	}
	for _, candidate := range units {
		if !strings.HasSuffix(text, candidate.name) {
			continue
		}
		number := strings.TrimSpace(strings.TrimSuffix(text, candidate.name))
		if number == "" {
			break
		}
		parsed, parseErr := strconv.ParseFloat(number, 64)
		if parseErr != nil {
			return ir.Length{}, fmt.Errorf("%w: invalid length %q", ErrInvalidMathML, text)
		}
		var length ir.Length
		var err error
		switch candidate.unit {
		case ir.LengthEm:
			length, err = ir.EmLength(parsed)
		case ir.LengthEx:
			length, err = ir.ExLength(parsed)
		case ir.LengthMu:
			length, err = ir.MuLength(parsed)
		case ir.LengthPt:
			length, err = ir.PtLength(parsed)
		case ir.LengthDip:
			length, err = ir.DipLength(parsed)
		case ir.LengthPercent:
			length, err = ir.PercentLength(parsed)
		default:
			err = fmt.Errorf("unknown unit")
		}
		if err != nil {
			return ir.Length{}, fmt.Errorf("%w: %v", ErrInvalidMathML, err)
		}
		return length, nil
	}
	return ir.Length{}, fmt.Errorf("%w: unsupported length %q", ErrUnsupported, text)
}

func nonNegativeLength(text, field string) (ir.Length, error) {
	length, err := parseLength(text)
	if err != nil {
		return ir.Length{}, err
	}
	if length.Value < 0 {
		return ir.Length{}, fmt.Errorf("%w: %s must be non-negative", ErrInvalidMathML, field)
	}
	return length, nil
}

func diagnostic(span source.Range, code source.Code, err error) source.Diagnostic {
	return source.Diagnostic{
		Range:    span,
		Severity: source.SeverityError,
		Code:     code,
		Message:  err.Error(),
	}
}
