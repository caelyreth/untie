package typst

import (
	"strconv"

	"github.com/caelyreth/untie/ir"
	"github.com/caelyreth/untie/source"
)

func (l *lowerer) lowerText(call *astNode) ir.Node {
	positional := l.positionalArguments(call)
	var bodyArg *astArg
	if len(positional) > 0 {
		bodyArg = &positional[len(positional)-1]
	}
	if named := namedArgument(call, "body"); named != nil {
		bodyArg = named
	}
	if bodyArg == nil {
		if named := namedArgument(call, "text"); named != nil {
			bodyArg = named
		}
	}
	if bodyArg == nil {
		l.errorAt(call.span, source.CodeMissingArgument, "missing argument: body")
		return l.placeholder(call.span, "missing text body")
	}
	body := l.lowerTextBody(bodyArg.node)
	if arg := namedArgument(call, "tracking"); arg != nil {
		if tracking, ok := l.valueLength(arg.node); ok {
			applyTextTracking(body, tracking)
		}
	}
	if arg := namedArgument(call, "weight"); arg != nil {
		if boldWeight(l, arg.node) {
			applyVariant(body, ir.VariantBold)
		}
	}
	if arg := namedArgument(call, "style"); arg != nil {
		style, _ := l.valueString(arg.node)
		if style == "italic" || style == "oblique" {
			applyVariant(body, ir.VariantItalic)
		}
	}
	if arg := namedArgument(call, "fill"); arg != nil {
		if paint, ok := l.valuePaint(arg.node); ok {
			return ir.NewColor(call.span, body, paint)
		}
		l.errorAt(arg.span, source.CodeInvalidValue, "expected color, found %s", l.valueType(arg.node))
	} else if len(positional) >= 2 {
		// Typst math also accepts the fill as the first positional argument:
		// text(#red, x).
		if paint, ok := l.valuePaint(positional[0].node); ok {
			return ir.NewColor(call.span, body, paint)
		}
	}
	if arg := namedArgument(call, "size"); arg != nil {
		// A continuous text size is not representable in the neutral IR's
		// discrete math-size field. Reject it explicitly rather than silently
		// producing the wrong scale.
		l.warningAt(arg.span, source.CodeUnsupported, "continuous text size is not represented by the neutral IR")
	}
	handled := setOf("body", "text", "fill", "tracking", "weight", "style", "size")
	for _, arg := range call.args {
		if arg.kind == argNamed && !handled[arg.name] {
			l.warningAt(arg.span, source.CodeUnsupported, "text %s is not represented by the neutral Text node", arg.name)
		}
	}
	return body
}

func boldWeight(l *lowerer, n *astNode) bool {
	if v, ok := l.evalValue(n); ok {
		if v.kind == valueInteger {
			return v.integer >= 600
		}
		if v.kind == valueString {
			switch v.text {
			case "semibold", "bold", "extrabold", "black":
				return true
			}
		}
	}
	return false
}

func (l *lowerer) lowerTextBody(n *astNode) ir.Node {
	if n == nil {
		return l.placeholder(source.Detached(), "missing text body")
	}
	switch n.kind {
	case astString:
		return l.text(n.span, n.text)
	case astIdentifier, astField:
		if def, ok := symbol(n.text); ok {
			return l.text(n.span, def.text)
		}
		return l.text(n.span, n.text)
	case astGlyph:
		return l.text(n.span, n.text)
	case astNumber:
		return l.text(n.span, n.text)
	case astCode:
		if len(n.items) == 1 {
			return l.lowerTextBody(n.items[0])
		}
	case astLiteral:
		return l.text(n.span, valueText(n.value))
	case astRow, astDelimited:
		items := n.items
		if n.kind == astDelimited && len(items) == 1 {
			return l.lowerTextBody(items[0])
		}
		row := ir.NewRow(n.span)
		previousEnd := -1
		for _, item := range items {
			if item.kind == astLinebreak || item.kind == astAlign {
				continue
			}
			if previousEnd >= 0 && item.span.Start > previousEnd && hasWhitespace(l.sourceText(source.NewRange(previousEnd, item.span.Start))) {
				row.Items = append(row.Items, l.text(source.NewRange(previousEnd, item.span.Start), " "))
			}
			row.Items = append(row.Items, l.lowerTextBody(item))
			previousEnd = item.span.End
		}
		return row
	}
	return l.lower(n)
}

func hasWhitespace(text string) bool {
	for i := 0; i < len(text); i++ {
		if isSpace(text[i]) {
			return true
		}
	}
	return false
}

func valueText(v value) string {
	switch v.kind {
	case valueString:
		return v.text
	case valueInteger:
		return strconv.FormatInt(v.integer, 10)
	case valueFloat:
		return strconv.FormatFloat(v.float, 'g', -1, 64)
	case valueBool:
		if v.boolean {
			return "true"
		}
		return "false"
	case valueNone:
		return "none"
	case valueAuto:
		return "auto"
	case valueSymbol:
		return v.symbol
	default:
		return ""
	}
}

func applyTextTracking(n ir.Node, tracking ir.Length) {
	if n == nil {
		return
	}
	if text, ok := n.(*ir.Text); ok {
		text.Tracking = tracking
	}
	for _, child := range ir.Children(n) {
		applyTextTracking(child, tracking)
	}
}

func (l *lowerer) lowerBox(call *astNode) ir.Node {
	body := l.contentArg(call, "body", 0, true)
	box := ir.NewBox(call.span, body)
	if arg := namedArgument(call, "inset"); arg != nil {
		if padding, ok := l.valueLength(arg.node); ok {
			box.Padding = [4]ir.Length{padding, padding, padding, padding}
		} else {
			l.errorAt(arg.span, source.CodeInvalidValue, "expected length, found %s", l.valueType(arg.node))
		}
	}
	if arg := namedArgument(call, "stroke"); arg != nil {
		if paint, ok := l.valuePaint(arg.node); ok {
			box.HasBorder = true
			box.Border = paint
			box.BorderWidth = builtinEmLength(0.05)
		} else if !isNoneAST(arg.node) {
			l.errorAt(arg.span, source.CodeInvalidValue, "expected color or stroke")
		}
	} else if arg := namedArgument(call, "border"); arg != nil {
		if paint, ok := l.valuePaint(arg.node); ok {
			box.HasBorder = true
			box.Border = paint
		} else {
			l.errorAt(arg.span, source.CodeInvalidValue, "expected color")
		}
	}
	if arg := namedArgument(call, "fill"); arg != nil {
		if paint, ok := l.valuePaint(arg.node); ok {
			box.HasFill = true
			box.Fill = paint
		} else if !isNoneAST(arg.node) {
			l.errorAt(arg.span, source.CodeInvalidValue, "expected color, found %s", l.valueType(arg.node))
		}
	}
	if arg := namedArgument(call, "radius"); arg != nil {
		if radius, ok := l.valueLength(arg.node); ok {
			box.Radius = radius
		} else {
			l.errorAt(arg.span, source.CodeInvalidValue, "expected length, found %s", l.valueType(arg.node))
		}
	}
	for _, name := range []string{"width", "height", "baseline", "outset", "clip"} {
		if arg := namedArgument(call, name); arg != nil {
			l.warningAt(arg.span, source.CodeUnsupported, "box %s is not represented by the neutral Box node", name)
		}
	}
	return box
}

func (l *lowerer) lowerHSpace(call *astNode) ir.Node {
	arg := namedArgument(call, "amount")
	if arg == nil {
		positional := l.positionalArguments(call)
		if len(positional) > 0 {
			arg = &positional[0]
		}
	}
	if arg == nil {
		l.errorAt(call.span, source.CodeMissingArgument, "missing argument: amount")
		return l.placeholder(call.span, "missing horizontal space amount")
	}
	amount, ok := l.valueLength(arg.node)
	if !ok {
		l.errorAt(arg.span, source.CodeInvalidValue, "expected length, found %s", l.valueType(arg.node))
		return l.placeholder(arg.span, "invalid horizontal space amount")
	}
	space := ir.NewSpace(call.span, ir.SpaceExplicit, amount)
	if weak := namedArgument(call, "weak"); weak != nil {
		if value, ok := l.valueBool(weak.node); ok {
			space.Weak = value
		}
	}
	return space
}

func (l *lowerer) lowerHide(call *astNode) ir.Node {
	body := l.contentArg(call, "body", 0, true)
	return ir.NewPhantom(call.span, body)
}

func (l *lowerer) lowerNamedOrPositional(call *astNode, name string, fallback int) ir.Node {
	if named := l.contentNamed(call, name); named != nil {
		return named
	}
	return l.contentArg(call, name, fallback, true)
}

func (l *lowerer) lowerEquation(call *astNode) ir.Node {
	for _, arg := range call.args {
		if arg.kind == argNamed && arg.name != "body" {
			l.warningAt(arg.span, source.CodeUnsupported, "equation %s metadata is not represented in the neutral IR", arg.name)
		}
	}
	return l.lowerNamedOrPositional(call, "body", len(call.args)-1)
}

func (l *lowerer) lowerValueContent(span source.Range, v value) ir.Node {
	switch v.kind {
	case valueNone:
		return l.placeholder(span, "none has no visual content")
	case valueAuto:
		return l.glyph(span, "auto")
	case valueBool:
		if v.boolean {
			return l.glyph(span, "true")
		}
		return l.glyph(span, "false")
	case valueInteger:
		return l.glyph(span, strconv.FormatInt(v.integer, 10))
	case valueFloat:
		return l.glyph(span, strconv.FormatFloat(v.float, 'g', -1, 64))
	case valueString:
		return l.text(span, v.text)
	case valueSymbol:
		return l.glyph(span, v.symbol)
	case valueContent:
		return l.lower(v.content)
	case valueColor:
		l.errorAt(span, source.CodeInvalidValue, "color used where content was expected")
		return l.errNode(span, l.sourceText(span), source.CodeInvalidValue)
	case valueArray, valueDictionary, valueLength:
		l.errorAt(span, source.CodeInvalidValue, "%s used where content was expected", v.typeName())
		return l.errNode(span, l.sourceText(span), source.CodeInvalidValue)
	default:
		return l.placeholder(span, v.typeName()+" used where content was expected")
	}
}
