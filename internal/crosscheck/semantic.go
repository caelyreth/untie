package crosscheck

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"reflect"
	"slices"
	"unicode"
	"unicode/utf8"

	"github.com/caelyreth/untie/ir"
)

// semanticMode supplies the inherited root style. TeX records text/display
// size on some wrapper nodes while Typst leaves it implicit; comparing
// effective state is the only dialect-neutral representation.
type semanticMode struct {
	size                          ir.MathSize
	cramped                       bool
	deparenthesizeFractionOperand bool
}

func normalizeFormula(formula *ir.Formula, mode ir.Mode) (any, error) {
	if formula == nil || formula.Root == nil {
		return nil, fmt.Errorf("nil formula")
	}
	initial := semanticMode{size: ir.SizeText}
	if mode == ir.Display {
		initial.size = ir.SizeDisplay
	}
	return normalizeNode(formula.Root, initial)
}

func normalizeTypstFormula(formula *ir.Formula, mode ir.Mode) (any, error) {
	if formula == nil || formula.Root == nil {
		return nil, fmt.Errorf("nil formula")
	}
	initial := semanticMode{size: ir.SizeText, deparenthesizeFractionOperand: true}
	if mode == ir.Display {
		initial.size = ir.SizeDisplay
	}
	return normalizeNode(formula.Root, initial)
}

// normalizeNode deliberately removes only source locations and the documented
// dialect-irrelevant projection described in README.md. Node-specific fields
// remain strict.
func normalizeNode(n ir.Node, inherited semanticMode) (any, error) {
	if n == nil {
		return nil, nil
	}
	base := n.NodeBase()
	size := inherited.size
	if base.Size != ir.SizeInherit {
		size = base.Size
	}
	cramped := inherited.cramped || base.Cramped
	childMode := inherited
	childMode.size = size
	childMode.cramped = cramped
	baseValue := normalizedBase(n, size, cramped)

	switch node := n.(type) {
	case *ir.Row:
		items := make([]any, 0, len(node.Items))
		for _, child := range node.Items {
			value, err := normalizeNode(child, childMode)
			if err != nil {
				return nil, err
			}
			items = appendNormalized(items, value)
		}
		items = coalesceTextRuns(items)
		if len(items) == 0 {
			return map[string]any{"kind": "empty-row"}, nil
		}
		if isNeutralRow(base) {
			if len(items) == 1 {
				return items[0], nil
			}
			return items, nil
		}
		baseValue["kind"] = "row"
		baseValue["items"] = items
		return baseValue, nil
	case *ir.Glyph:
		return normalizeAtom(baseValue, node.Text, base)
	case *ir.Operator:
		return normalizeAtom(baseValue, node.Text, base)
	case *ir.Text:
		baseValue["kind"] = "text"
		baseValue["text"] = node.Text
		baseValue["font"] = fontRoleName(node.Font)
		baseValue["tracking"] = normalizedLength(node.Tracking)
		return baseValue, nil
	case *ir.Space:
		baseValue["kind"] = "space"
		baseValue["space"] = spaceKindName(node.Type)
		baseValue["amount"] = normalizedLength(node.Amount)
		baseValue["weak"] = node.Weak
		return baseValue, nil
	case *ir.Fraction:
		numerator, err := normalizeFractionOperand(node.Numerator, childMode)
		if err != nil {
			return nil, err
		}
		denominator, err := normalizeFractionOperand(node.Denominator, childMode)
		if err != nil {
			return nil, err
		}
		baseValue["kind"] = "fraction"
		baseValue["numerator"] = numerator
		baseValue["denominator"] = denominator
		baseValue["bar"] = node.Bar
		baseValue["style"] = fractionStyleName(node.Style)
		baseValue["padding"] = normalizedFractionPadding(node.Padding)
		return baseValue, nil
	case *ir.Radical:
		radicand, err := normalizeNode(node.Radicand, childMode)
		if err != nil {
			return nil, err
		}
		index, err := normalizeNode(node.Index, childMode)
		if err != nil {
			return nil, err
		}
		baseValue["kind"] = "radical"
		baseValue["radicand"] = radicand
		baseValue["index"] = index
		return baseValue, nil
	case *ir.Scripts:
		if isBareScripts(node) {
			child, err := normalizeNode(node.BaseItem, childMode)
			if err != nil {
				return nil, err
			}
			if node.Limits != ir.LimitsAuto {
				if item, ok := child.(map[string]any); ok {
					item["limits"] = limitsName(node.Limits)
					return item, nil
				}
			}
			return child, nil
		}
		children := make(map[string]any, 7)
		type childSpec struct {
			name  string
			child ir.Node
		}
		values := []childSpec{
			{"main", node.BaseItem},
			{"sup-left", node.TopLeft},
			{"sub-left", node.BottomLeft},
		}
		aboveBelow := usesLimits(node, size)
		if aboveBelow {
			values = append(values,
				childSpec{"limit-top", node.Top},
				childSpec{"limit-bottom", node.Bottom},
				childSpec{"sup-right", node.TopRight},
				childSpec{"sub-right", node.BottomRight},
			)
		} else {
			if node.Top != nil && node.TopRight != nil {
				return nil, fmt.Errorf("scripts has both semantic top and forced top-right")
			}
			if node.Bottom != nil && node.BottomRight != nil {
				return nil, fmt.Errorf("scripts has both semantic bottom and forced bottom-right")
			}
			values = append(values,
				childSpec{"sup-right", firstNode(node.TopRight, node.Top)},
				childSpec{"sub-right", firstNode(node.BottomRight, node.Bottom)},
			)
		}
		for _, value := range values {
			normalized, err := normalizeNode(value.child, childMode)
			if err != nil {
				return nil, err
			}
			children[value.name] = normalized
		}
		// Scripts' own atom class is derived from the base node by both
		// frontends; the base class is compared once inside main.
		if main := node.BaseItem; main != nil {
			baseValue["class"] = mathClassName(effectiveClass(main))
		}
		if aboveBelow {
			baseValue["limits"] = "above-below"
		} else {
			baseValue["limits"] = "side"
		}
		baseValue["kind"] = "scripts"
		for name, value := range children {
			baseValue[name] = value
		}
		return baseValue, nil
	case *ir.Fenced:
		open, err := normalizeNode(node.Open, childMode)
		if err != nil {
			return nil, err
		}
		closeNode, err := normalizeNode(node.Close, childMode)
		if err != nil {
			return nil, err
		}
		open = nullDelimiter(open)
		closeNode = nullDelimiter(closeNode)
		open, closeNode = canonicalFenceClasses(open, closeNode)
		body := make([]any, len(node.Body))
		for i, child := range node.Body {
			body[i], err = normalizeNode(child, childMode)
			if err != nil {
				return nil, err
			}
		}
		if isLiteralBalancedFence(node) {
			result := make([]any, 0, len(body)+2)
			result = append(result, open)
			for _, item := range body {
				if list, ok := item.([]any); ok {
					result = append(result, list...)
				} else {
					result = append(result, item)
				}
			}
			result = append(result, closeNode)
			return result, nil
		}
		baseValue["kind"] = "fenced"
		baseValue["open"] = open
		baseValue["close"] = closeNode
		baseValue["body"] = body
		baseValue["balanced"] = node.Balanced
		return baseValue, nil
	case *ir.Accent:
		body, err := normalizeNode(node.Body, childMode)
		if err != nil {
			return nil, err
		}
		accent, err := normalizeNode(node.Accent, childMode)
		if err != nil {
			return nil, err
		}
		accent = canonicalAccent(accent)
		baseValue["kind"] = "accent"
		baseValue["body"] = body
		baseValue["accent"] = accent
		baseValue["position"] = positionName(node.Position)
		baseValue["stretchy"] = node.Stretchy && !isSingleAccentBase(node.Body)
		baseValue["dotless"] = node.Dotless
		baseValue["exact-width"] = node.ExactFrameWidth
		return baseValue, nil
	case *ir.Line:
		body, err := normalizeNode(node.Body, childMode)
		if err != nil {
			return nil, err
		}
		baseValue["kind"] = "line"
		baseValue["body"] = body
		baseValue["position"] = positionName(node.Position)
		baseValue["thickness"] = normalizedDefaultStroke(node.Thickness)
		baseValue["gap"] = normalizedLength(node.Gap)
		return baseValue, nil
	case *ir.Cancel:
		body, err := normalizeNode(node.Body, childMode)
		if err != nil {
			return nil, err
		}
		baseValue["kind"] = "cancel"
		baseValue["body"] = body
		baseValue["cross"] = node.Cross
		baseValue["inverted"] = node.Inverted
		baseValue["thickness"] = normalizedDefaultStroke(node.Thickness)
		return baseValue, nil
	case *ir.Table:
		var err error
		rows := make([]any, len(node.Rows))
		for i, row := range node.Rows {
			cells := make([]any, len(row.Cells))
			for j, child := range row.Cells {
				cells[j], err = normalizeNode(child, childMode)
				if err != nil {
					return nil, err
				}
			}
			tag, err := normalizeNode(row.Tag, childMode)
			if err != nil {
				return nil, err
			}
			rows[i] = map[string]any{"cells": cells, "tag": tag}
		}
		baseValue["kind"] = "table"
		baseValue["rows"] = rows
		baseValue["columns"] = normalizedColumns(node.Columns)
		baseValue["row-gap"] = normalizedLength(node.RowGap)
		if len(node.Columns) <= 1 {
			baseValue["column-gap"] = nil
		} else {
			baseValue["column-gap"] = normalizedLength(node.ColumnGap)
		}
		baseValue["horizontal"] = normalizedTableLines(node.Horizontal)
		baseValue["vertical"] = normalizedTableLines(node.Vertical)
		return baseValue, nil
	case *ir.Multiline:
		rows := make([]any, len(node.Rows))
		for i, row := range node.Rows {
			items := make([]any, 0, len(row))
			for _, child := range row {
				value, err := normalizeNode(child, childMode)
				if err != nil {
					return nil, err
				}
				items = appendNormalized(items, value)
			}
			rows[i] = items
		}
		baseValue["kind"] = "multiline"
		baseValue["rows"] = rows
		baseValue["centered"] = node.Centered
		return baseValue, nil
	case *ir.Style:
		if node.SetSize {
			childMode.size = node.NewSize
		}
		if node.SetCramped {
			childMode.cramped = node.Cramped
		}
		if isTransparentSizeStyle(node) {
			return normalizeNode(node.Body, childMode)
		}
		body, err := normalizeNode(node.Body, childMode)
		if err != nil {
			return nil, err
		}
		baseValue["kind"] = "style"
		baseValue["body"] = body
		baseValue["set-size"] = node.SetSize
		baseValue["new-size"] = mathSizeName(node.NewSize)
		baseValue["set-cramped"] = node.SetCramped
		baseValue["cramped"] = node.Cramped
		return baseValue, nil
	case *ir.Color:
		body, err := normalizeNode(node.Body, childMode)
		if err != nil {
			return nil, err
		}
		baseValue["kind"] = "color"
		baseValue["body"] = body
		baseValue["paint"] = normalizedPaint(node.Paint)
		return baseValue, nil
	case *ir.Box:
		body, err := normalizeNode(node.Body, childMode)
		if err != nil {
			return nil, err
		}
		baseValue["kind"] = "box"
		baseValue["body"] = body
		baseValue["padding"] = []any{
			normalizedLength(node.Padding[0]), normalizedLength(node.Padding[1]),
			normalizedLength(node.Padding[2]), normalizedLength(node.Padding[3]),
		}
		baseValue["fill"] = normalizedPaint(node.Fill)
		baseValue["has-fill"] = node.HasFill
		baseValue["border"] = normalizedPaint(node.Border)
		baseValue["has-border"] = node.HasBorder
		baseValue["border-width"] = normalizedLength(node.BorderWidth)
		baseValue["radius"] = normalizedLength(node.Radius)
		return baseValue, nil
	case *ir.Phantom:
		body, err := normalizeNode(node.Body, childMode)
		if err != nil {
			return nil, err
		}
		baseValue["kind"] = "phantom"
		baseValue["body"] = body
		baseValue["width"] = node.Width
		baseValue["height"] = node.Height
		baseValue["depth"] = node.Depth
		return baseValue, nil
	case *ir.ErrorNode:
		baseValue["kind"] = "error"
		baseValue["text"] = node.Text
		baseValue["code"] = string(node.Code)
		return baseValue, nil
	case *ir.Placeholder:
		baseValue["kind"] = "placeholder"
		baseValue["reason"] = node.Reason
		return baseValue, nil
	case *ir.Break:
		baseValue["kind"] = "break"
		baseValue["break"] = breakKindName(node.Break)
		baseValue["penalty"] = node.Penalty
		return baseValue, nil
	case *ir.AlignMarker:
		baseValue["kind"] = "align-marker"
		baseValue["class"] = "special"
		return baseValue, nil
	default:
		return nil, fmt.Errorf("unsupported IR node %T", n)
	}
}

func nullDelimiter(value any) any {
	if item, ok := value.(map[string]any); ok && len(item) == 1 && item["kind"] == "empty-row" {
		return nil
	}
	return value
}

func appendNormalized(items []any, value any) []any {
	if list, ok := value.([]any); ok {
		return append(items, list...)
	}
	return append(items, value)
}

func coalesceTextRuns(items []any) []any {
	if len(items) < 2 {
		return items
	}
	out := make([]any, 0, len(items))
	for _, value := range items {
		current, currentOK := value.(map[string]any)
		if currentOK && current["kind"] == "text" && len(out) > 0 {
			previous, previousOK := out[len(out)-1].(map[string]any)
			if previousOK && previous["kind"] == "text" && sameTextMetadata(previous, current) {
				previous["text"] = previous["text"].(string) + current["text"].(string)
				continue
			}
		}
		out = append(out, value)
	}
	return out
}

func sameTextMetadata(left, right map[string]any) bool {
	if len(left) != len(right) {
		return false
	}
	for key, value := range left {
		if key == "text" {
			continue
		}
		if other, ok := right[key]; !ok || !reflect.DeepEqual(value, other) {
			return false
		}
	}
	return true
}

func withAtomClass(value any, class string) any {
	item, ok := value.(map[string]any)
	if !ok {
		return value
	}
	switch item["kind"] {
	case "atom", "operator-text":
		item["class"] = class
	}
	return item
}

func canonicalFenceClasses(open, close any) (any, any) {
	openMap, openOK := open.(map[string]any)
	closeMap, closeOK := close.(map[string]any)
	if openOK && closeOK {
		openText, _ := openMap["text"].(string)
		closeText, _ := closeMap["text"].(string)
		if openText != "" && openText == closeText {
			return withAtomClass(open, "fence"), withAtomClass(close, "fence")
		}
	}
	return withAtomClass(open, "opening"), withAtomClass(close, "closing")
}

func isBareScripts(node *ir.Scripts) bool {
	return node.Top == nil && node.Bottom == nil &&
		node.TopLeft == nil && node.BottomLeft == nil &&
		node.TopRight == nil && node.BottomRight == nil
}

func normalizeFractionOperand(n ir.Node, mode semanticMode) (any, error) {
	fenced, ok := n.(*ir.Fenced)
	if !mode.deparenthesizeFractionOperand || !ok ||
		!fenced.Balanced || fenced.Open == nil || fenced.Close == nil ||
		glyphText(fenced.Open) != "(" || glyphText(fenced.Close) != ")" {
		return normalizeNode(n, mode)
	}
	items := make([]any, 0, len(fenced.Body))
	for _, child := range fenced.Body {
		value, err := normalizeNode(child, mode)
		if err != nil {
			return nil, err
		}
		items = appendNormalized(items, value)
	}
	items = coalesceTextRuns(items)
	if len(items) == 1 {
		return items[0], nil
	}
	return items, nil
}

func glyphText(n ir.Node) string {
	switch node := n.(type) {
	case *ir.Glyph:
		return node.Text
	case *ir.Operator:
		return node.Text
	default:
		return ""
	}
}

func canonicalAccent(value any) any {
	item, ok := value.(map[string]any)
	if !ok || item["kind"] != "atom" {
		return value
	}
	text, ok := item["text"].(string)
	if !ok {
		return value
	}
	if canonical, ok := accentCanonicalForms[text]; ok {
		item["text"] = canonical
	}
	item["class"] = "diacritic"
	item["variant"] = "normal-or-auto"
	return item
}

var accentCanonicalForms = map[string]string{
	"^": "\u0302", "ˆ": "\u0302", "̂": "\u0302",
	"~": "\u0303", "˜": "\u0303", "∼": "\u0303", "̃": "\u0303",
	"¯": "\u0304", "‾": "\u0304", "̄": "\u0304",
	"˘": "\u0306", "̆": "\u0306",
	".": "\u0307", "˙": "\u0307", "⋅": "\u0307", "̇": "\u0307",
	"¨": "\u0308", "̈": "\u0308",
	"˚": "\u030a", "∘": "\u030a", "○": "\u030a", "̊": "\u030a",
	"`": "\u0300", "̀": "\u0300",
	"´": "\u0301", "́": "\u0301",
	"ˇ": "\u030c", "̌": "\u030c",
	"→": "\u20d7", "←": "\u20d6", "↔": "\u20e1",
}

func isTransparentSizeStyle(node *ir.Style) bool {
	base := node.NodeBase()
	return node.SetSize && !node.SetCramped &&
		base.Class == ir.ClassNormal &&
		base.Variant == ir.VariantAuto &&
		base.Paint.Role == ir.PaintInk && !base.Paint.HasExplicit &&
		base.Limits == ir.LimitsAuto &&
		base.LeftSpacing.IsZero() && base.RightSpacing.IsZero() &&
		!base.IgnoreAsSpace
}

func isLiteralBalancedFence(node *ir.Fenced) bool {
	if !node.Balanced || node.Open == nil || node.Close == nil || len(node.Body) == 0 {
		return false
	}
	base := node.NodeBase()
	if base.Class != ir.ClassInner ||
		base.Variant != ir.VariantAuto ||
		base.Paint.Role != ir.PaintInk || base.Paint.HasExplicit ||
		base.Limits != ir.LimitsAuto ||
		!base.LeftSpacing.IsZero() || !base.RightSpacing.IsZero() ||
		base.IgnoreAsSpace {
		return false
	}
	for _, child := range node.Body {
		if !isLiteralFenceNode(child) {
			return false
		}
	}
	return true
}

func isLiteralFenceNode(n ir.Node) bool {
	switch n := n.(type) {
	case *ir.Glyph, *ir.Operator, *ir.Space:
		return true
	case *ir.Row:
		for _, child := range n.Items {
			if !isLiteralFenceNode(child) {
				return false
			}
		}
		return true
	default:
		return false
	}
}

func isSingleAccentBase(n ir.Node) bool {
	switch node := n.(type) {
	case *ir.Glyph, *ir.Operator:
		return true
	case *ir.Row:
		return len(node.Items) == 1 && isSingleAccentBase(node.Items[0])
	default:
		return false
	}
}

func normalizedBase(n ir.Node, size ir.MathSize, cramped bool) map[string]any {
	base := n.NodeBase()
	return map[string]any{
		"kind":            ir.KindName(base.Kind()),
		"class":           mathClassName(effectiveClass(n)),
		"size":            mathSizeName(size),
		"cramped":         cramped,
		"variant":         variantName(base.Variant),
		"paint":           normalizedPaint(base.Paint),
		"limits":          limitsName(base.Limits),
		"left-spacing":    normalizedLength(base.LeftSpacing),
		"right-spacing":   normalizedLength(base.RightSpacing),
		"ignore-as-space": base.IgnoreAsSpace,
	}
}

func effectiveClass(n ir.Node) ir.MathClass {
	base := n.NodeBase()
	if scripts, ok := n.(*ir.Scripts); ok && scripts.BaseItem != nil {
		return effectiveClass(scripts.BaseItem)
	}
	// Operator nodes are semantic Op atoms. Both Typst's resolveOp and TeX's
	// operator paths mean ClassLarge even when one frontend leaves the default
	// class on the node.
	if _, ok := n.(*ir.Operator); ok && base.Class == ir.ClassNormal {
		return ir.ClassLarge
	}
	return base.Class
}

func normalizeAtom(base map[string]any, text string, source *ir.Base) (any, error) {
	canonicalizeAtomLimits(base, text)
	if utf8.RuneCountInString(text) > 1 && allDigits(text) {
		items := make([]any, 0, utf8.RuneCountInString(text))
		for _, r := range text {
			item := cloneMap(base)
			item["kind"] = "atom"
			item["text"] = string(r)
			items = append(items, item)
		}
		return items, nil
	}
	if text == "" {
		base["kind"] = "atom"
		base["text"] = text
		return base, nil
	}
	r, size := utf8.DecodeRuneInString(text)
	if size != len(text) {
		base["kind"] = "operator-text"
		base["text"] = text
		return base, nil
	}
	if source.Variant == ir.VariantBold && isGreekLetter(r) {
		// Math bold Greek follows the bold-italic alphabet unless an upright
		// bold face was explicitly selected.
		base["variant"] = "bold-italic"
	}
	// A single Unicode symbol may be represented as Glyph by one frontend and
	// Operator by the other. The source encoding has no neutral semantics.
	if !unicode.IsLetter(r) && !unicode.IsDigit(r) &&
		(source.Variant == ir.VariantNormal || source.Variant == ir.VariantAuto) {
		base["variant"] = "normal-or-auto"
	}
	base["kind"] = "atom"
	base["text"] = text
	return base, nil
}

func isGreekLetter(r rune) bool {
	return (r >= '\u0370' && r <= '\u03ff') || (r >= '\u1f00' && r <= '\u1fff')
}

func allDigits(text string) bool {
	for _, r := range text {
		if !unicode.IsDigit(r) {
			return false
		}
	}
	return text != ""
}

func cloneMap(source map[string]any) map[string]any {
	out := make(map[string]any, len(source))
	for key, value := range source {
		out[key] = value
	}
	return out
}

func canonicalizeAtomLimits(base map[string]any, text string) {
	limits, _ := base["limits"].(string)
	if base["class"] == "large" && isIntegralText(text) && (limits == "auto" || limits == "never") {
		base["limits"] = "never"
	}
	if base["class"] == "relation" && (limits == "auto" || limits == "always") {
		base["limits"] = "always"
	}
}

func usesLimits(node *ir.Scripts, size ir.MathSize) bool {
	placement := node.Limits
	class := effectiveClass(node.BaseItem)
	if base := node.BaseItem.NodeBase(); base != nil {
		if placement == ir.LimitsAuto {
			placement = base.Limits
		}
	}
	switch placement {
	case ir.LimitsAlways:
		return true
	case ir.LimitsNever:
		return false
	}
	if class == ir.ClassRelation {
		return true
	}
	return size == ir.SizeDisplay && class == ir.ClassLarge && !isIntegralBase(node.BaseItem)
}

func isIntegralBase(n ir.Node) bool {
	for {
		switch node := n.(type) {
		case *ir.Glyph:
			return isIntegralText(node.Text)
		case *ir.Operator:
			return isIntegralText(node.Text)
		case *ir.Scripts:
			n = node.BaseItem
			continue
		case *ir.Row:
			if len(node.Items) != 1 {
				return false
			}
			n = node.Items[0]
			continue
		default:
			return false
		}
	}
}

func isIntegralText(text string) bool {
	r, size := utf8.DecodeRuneInString(text)
	if size == 0 || size != len(text) {
		return false
	}
	return (r >= '\u222b' && r <= '\u2233') || (r >= '\u2a0b' && r <= '\u2a1c')
}

func firstNode(nodes ...ir.Node) ir.Node {
	for _, node := range nodes {
		if node != nil {
			return node
		}
	}
	return nil
}

func isNeutralRow(base *ir.Base) bool {
	return base.Class == ir.ClassNormal &&
		base.Paint.Role == ir.PaintInk && !base.Paint.HasExplicit &&
		base.Limits == ir.LimitsAuto &&
		base.LeftSpacing.IsZero() && base.RightSpacing.IsZero() &&
		!base.IgnoreAsSpace
}

func CompareSemantic(want, got any) *SemanticDifference {
	if reflect.DeepEqual(want, got) {
		return nil
	}
	path, w, g := firstDifference("root", want, got)
	return &SemanticDifference{
		Path: path, Expected: shortJSON(w), Actual: shortJSON(g),
	}
}

type SemanticDifference struct {
	Path     string
	Expected string
	Actual   string
}

func shortJSON(value any) string {
	data, err := json.Marshal(value)
	if err != nil {
		return fmt.Sprintf("<json error: %v>", err)
	}
	const maxInline = 240
	if len(data) <= maxInline {
		return string(data)
	}
	digest := sha256.Sum256(data)
	cut := maxInline
	for cut > 0 && !utf8.RuneStart(data[cut]) {
		cut--
	}
	return fmt.Sprintf("%s...<sha256:%x>", data[:cut], digest[:8])
}

func (d *SemanticDifference) String() string {
	if d == nil {
		return ""
	}
	return fmt.Sprintf("%s: expected %s, got %s", d.Path, d.Expected, d.Actual)
}

func firstDifference(path string, want, got any) (string, any, any) {
	if reflect.DeepEqual(want, got) {
		return path, want, got
	}
	switch w := want.(type) {
	case map[string]any:
		g, ok := got.(map[string]any)
		if !ok {
			return path, want, got
		}
		keys := make([]string, 0, len(w)+len(g))
		for key := range w {
			keys = append(keys, key)
		}
		for key := range g {
			if _, ok := w[key]; !ok {
				keys = append(keys, key)
			}
		}
		slices.Sort(keys)
		for _, key := range keys {
			wv, wok := w[key]
			gv, gok := g[key]
			if wok != gok || !reflect.DeepEqual(wv, gv) {
				return firstDifference(path+"."+key, wv, gv)
			}
		}
	case []any:
		g, ok := got.([]any)
		if !ok {
			return path, want, got
		}
		limit := min(len(w), len(g))
		for i := 0; i < limit; i++ {
			if !reflect.DeepEqual(w[i], g[i]) {
				return firstDifference(fmt.Sprintf("%s[%d]", path, i), w[i], g[i])
			}
		}
		if len(w) != len(g) {
			return path + ".length", len(w), len(g)
		}
	}
	return path, want, got
}

func normalizedLength(length ir.Length) any {
	if length.IsZero() {
		return nil
	}
	if length.Unit == ir.LengthMu {
		// TeX math units are exactly 1/18 em. Normalize them to the same
		// millionth-em representation Typst's named spacing uses.
		return []any{"em", roundedDiv(length.Value, 18)}
	}
	return []any{lengthUnitName(length.Unit), length.Value}
}

func normalizedFractionPadding(length ir.Length) any {
	if length.IsZero() {
		return []any{"em", int64(100_000)}
	}
	return normalizedLength(length)
}

func normalizedDefaultStroke(length ir.Length) any {
	if length.IsZero() ||
		(length.Unit == ir.LengthEm && length.Value == 50_000) {
		return []any{"em", int64(50_000)}
	}
	return normalizedLength(length)
}

func roundedDiv(value, divisor int64) int64 {
	if value >= 0 {
		return (value + divisor/2) / divisor
	}
	return -((-value + divisor/2) / divisor)
}

func normalizedPaint(paint ir.Paint) any {
	if paint.HasExplicit {
		return []any{"rgba", paint.Explicit.R, paint.Explicit.G, paint.Explicit.B, paint.Explicit.A}
	}
	return []any{"role", paintRoleName(paint.Role)}
}

func normalizedColumns(columns []ir.ColumnAlign) []string {
	out := make([]string, len(columns))
	for i, column := range columns {
		out[i] = columnName(column)
	}
	return out
}

func normalizedTableLines(lines []ir.TableLine) []any {
	out := make([]any, len(lines))
	for i, line := range lines {
		out[i] = []any{line.Position, line.Count, line.Dashed, normalizedLength(line.Thickness)}
	}
	return out
}

func mathClassName(v ir.MathClass) string {
	return enumName(v, []string{"normal", "alphabetic", "binary", "relation", "opening", "closing", "fence", "punctuation", "large", "unary", "varying", "inner", "diacritic", "space", "special"})
}

func mathSizeName(v ir.MathSize) string {
	return enumName(v, []string{"inherit", "display", "text", "script", "script-script"})
}

func variantName(v ir.Variant) string {
	return enumName(v, []string{"auto", "normal", "italic", "bold", "bold-italic", "double-struck", "script", "bold-script", "fraktur", "bold-fraktur", "sans", "bold-sans", "monospace"})
}

func limitsName(v ir.LimitsPlacement) string {
	return enumName(v, []string{"auto", "always", "never"})
}

func paintRoleName(v ir.PaintRole) string {
	return enumName(v, []string{"ink", "error", "placeholder"})
}

func fontRoleName(v ir.FontRole) string {
	return enumName(v, []string{"math", "text"})
}

func spaceKindName(v ir.SpaceKind) string {
	return enumName(v, []string{"explicit", "interword", "ignored"})
}

func fractionStyleName(v ir.FractionStyle) string {
	return enumName(v, []string{"vertical", "no-bar", "skewed", "horizontal"})
}

func positionName(v ir.Position) string {
	return enumName(v, []string{"above", "below"})
}

func columnName(v ir.ColumnAlign) string {
	return enumName(v, []string{"center", "left", "right"})
}

func breakKindName(v ir.BreakKind) string {
	return enumName(v, []string{"relation", "binary", "explicit"})
}

func lengthUnitName(v ir.LengthUnit) string {
	return enumName(v, []string{"zero", "em", "ex", "mu", "pt", "dip", "percent"})
}

func enumName[T ~uint8](value T, names []string) string {
	if int(value) >= 0 && int(value) < len(names) {
		return names[value]
	}
	return fmt.Sprintf("unknown(%d)", value)
}
