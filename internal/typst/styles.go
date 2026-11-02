package typst

import (
	"strings"

	"github.com/caelyreth/untie/ir"
	"github.com/caelyreth/untie/source"
)

func (l *lowerer) lowerStyle(call *astNode, name string) ir.Node {
	body := l.contentArg(call, "body", 0, true)
	style := ir.NewStyle(call.span, body)
	switch name {
	case "display":
		style.SetSize = true
		style.NewSize = ir.SizeDisplay
	case "inline":
		style.SetSize = true
		style.NewSize = ir.SizeText
	case "script":
		style.SetSize = true
		style.NewSize = ir.SizeScript
	case "sscript":
		style.SetSize = true
		style.NewSize = ir.SizeScriptScript
	}
	if arg := namedArgument(call, "cramped"); arg != nil {
		if cramped, ok := l.valueBool(arg.node); ok {
			style.SetCramped = true
			style.Cramped = cramped
		}
	}
	return style
}

func (l *lowerer) lowerAlphabet(call *astNode, name string) ir.Node {
	body := l.contentArg(call, "body", 0, true)
	variant := ir.VariantAuto
	switch name {
	case "bold":
		variant = ir.VariantBold
	case "italic":
		variant = ir.VariantItalic
	case "upright", "serif":
		variant = ir.VariantNormal
	case "sans":
		variant = ir.VariantSans
	case "mono":
		variant = ir.VariantMonospace
	case "bb":
		variant = ir.VariantDoubleStruck
	case "cal", "scr":
		variant = ir.VariantScript
	case "frak":
		variant = ir.VariantFraktur
	}
	applyVariant(body, variant)
	return body
}

func applyVariant(n ir.Node, variant ir.Variant) {
	if n == nil {
		return
	}
	base := n.NodeBase()
	base.Variant = combineVariant(base.Variant, variant)
	for _, child := range ir.Children(n) {
		applyVariant(child, variant)
	}
}

func combineVariant(current, incoming ir.Variant) ir.Variant {
	if current == ir.VariantAuto {
		return incoming
	}
	switch incoming {
	case ir.VariantBold:
		switch current {
		case ir.VariantItalic:
			return ir.VariantBoldItalic
		case ir.VariantScript:
			return ir.VariantBoldScript
		case ir.VariantFraktur:
			return ir.VariantBoldFraktur
		case ir.VariantSans:
			return ir.VariantBoldSans
		case ir.VariantNormal:
			return ir.VariantBold
		default:
			return current
		}
	case ir.VariantItalic:
		if current == ir.VariantBold {
			return ir.VariantBoldItalic
		}
	case ir.VariantNormal:
		return ir.VariantNormal
	}
	return incoming
}

func (l *lowerer) lowerClass(call *astNode) ir.Node {
	positional := l.positionalArguments(call)
	var classNode, bodyNode *astNode
	if len(positional) >= 2 {
		classNode, bodyNode = positional[0].node, positional[1].node
	}
	if named := namedArgument(call, "class"); named != nil {
		classNode = named.node
	}
	if named := namedArgument(call, "body"); named != nil {
		bodyNode = named.node
	}
	if bodyNode == nil {
		l.errorAt(call.span, source.CodeMissingArgument, "missing argument: body")
		return l.placeholder(call.span, "missing class body")
	}
	body := l.lower(bodyNode)
	if classNode == nil {
		return body
	}
	className, ok := l.valueString(classNode)
	if !ok {
		l.errorAt(classNode.span, source.CodeInvalidValue, "expected string, found %s", l.valueType(classNode))
		return body
	}
	class, ok := parseClass(className)
	if !ok {
		l.errorAt(classNode.span, source.CodeInvalidValue, "unknown math class %q", className)
		return body
	}
	applyClass(body, class)
	return body
}

func parseClass(name string) (ir.MathClass, bool) {
	classes := map[string]ir.MathClass{
		"normal": ir.ClassNormal, "punctuation": ir.ClassPunctuation,
		"opening": ir.ClassOpening, "closing": ir.ClassClosing,
		"fence": ir.ClassFence, "large": ir.ClassLarge,
		"relation": ir.ClassRelation, "unary": ir.ClassUnary,
		"binary": ir.ClassBinary, "vary": ir.ClassVarying,
	}
	class, ok := classes[name]
	return class, ok
}

func applyClass(n ir.Node, class ir.MathClass) {
	if n == nil {
		return
	}
	n.NodeBase().Class = class
	for _, child := range ir.Children(n) {
		applyClass(child, class)
	}
}

func (l *lowerer) lowerOperator(call *astNode) ir.Node {
	text := l.contentArg(call, "text", 0, true)
	operator := ir.NewOperator(call.span, nodeText(text))
	operator.Variant = ir.VariantNormal
	operator.Class = ir.ClassNormal
	if arg := namedArgument(call, "limits"); arg != nil {
		limit, _ := l.valueBool(arg.node)
		if limit {
			operator.Limits = ir.LimitsAlways
		} else {
			operator.Limits = ir.LimitsNever
		}
	} else {
		operator.Limits = ir.LimitsNever
	}
	return operator
}

func nodeText(n ir.Node) string {
	switch n := n.(type) {
	case *ir.Text:
		return n.Text
	case *ir.Glyph:
		return n.Text
	case *ir.Operator:
		return n.Text
	case *ir.Row:
		var b strings.Builder
		for _, item := range n.Items {
			b.WriteString(nodeText(item))
		}
		return b.String()
	default:
		return ""
	}
}

func (l *lowerer) lowerStretch(call *astNode) ir.Node {
	if size := namedArgument(call, "size"); size != nil && !isDefaultPercent(l, size.node, 100) {
		l.warningAt(size.span, source.CodeUnsupported, "stretch size is not represented by the neutral IR")
	}
	return l.contentArg(call, "body", 0, true)
}

func (l *lowerer) lowerPrimes(call *astNode) ir.Node {
	count := int64(1)
	if arg := namedArgument(call, "count"); arg != nil {
		if n, ok := l.valueInteger(arg.node); ok {
			count = n
		}
	} else if positional := l.positionalArguments(call); len(positional) > 0 {
		if n, ok := l.valueInteger(positional[0].node); ok {
			count = n
		}
	}
	if count < 0 {
		l.errorAt(call.span, source.CodeInvalidValue, "expected non-negative integer")
		count = 0
	}
	if count > int64(l.limits.MaxTextBytes) {
		l.errorAt(call.span, source.CodeLimitExceeded, "prime count exceeds configured limit")
		count = int64(l.limits.MaxTextBytes)
	}
	return l.primeRow(call.span, int(count))
}

func (l *lowerer) primeRow(span source.Range, count int) ir.Node {
	if count <= 0 {
		return l.glyph(span, "′")
	}
	items := make([]ir.Node, 0, count)
	for range count {
		items = append(items, l.glyph(span, "′"))
	}
	if len(items) == 1 {
		return items[0]
	}
	row := ir.NewRow(span, items...)
	row.Class = ir.ClassInner
	return row
}
