package typst

import (
	"fmt"
	"strconv"
	"strings"

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
	if l.exhausted {
		return nil
	}
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
			return l.newColor(call.span, body, paint)
		}
		l.errorAt(arg.span, source.CodeInvalidValue, "expected color, found %s", l.valueType(arg.node))
	} else if len(positional) >= 2 {
		// Typst math also accepts the fill as the first positional argument:
		// text(#red, x).
		if paint, ok := l.valuePaint(positional[0].node); ok {
			return l.newColor(call.span, body, paint)
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

func (l *lowerer) lowerContentEmphasis(call *astNode, name string) ir.Node {
	body := l.contentArg(call, "body", 0, true)
	switch name {
	case "strong":
		if delta := namedArgument(call, "delta"); delta != nil {
			l.warningAt(delta.span, source.CodeUnsupported, "strong text weight delta is not represented by the neutral Text node")
		}
		applyVariant(body, ir.VariantBold)
	case "emph":
		applyVariant(body, ir.VariantItalic)
	}
	return body
}

func (l *lowerer) lowerLink(call *astNode) ir.Node {
	destinationArg, bodyArg, valid := l.linkArguments(call)
	if !valid {
		return l.placeholder(call.span, "invalid link arguments")
	}
	if destinationArg == nil {
		l.errorAt(call.span, source.CodeMissingArgument, "missing argument: dest")
		return l.placeholder(call.span, "missing link destination")
	}

	destination, ok := l.evalValue(destinationArg.node)
	if !ok || destination.kind != valueString {
		l.errorAt(destinationArg.span, source.CodeInvalidValue,
			"link destination expects a non-empty string in the bounded evaluator, found %s",
			l.valueType(destinationArg.node))
		return l.placeholder(call.span, "invalid link destination")
	}
	if destination.text == "" {
		l.errorAt(destinationArg.span, source.CodeInvalidValue, "link URL must not be empty")
		return l.placeholder(call.span, "invalid link destination")
	}
	if len(destination.text) > 8000 {
		l.errorAt(destinationArg.span, source.CodeInvalidValue, "link URL exceeds 8000 bytes")
		return l.placeholder(call.span, "invalid link destination")
	}

	var body ir.Node
	if bodyArg == nil {
		// Typst displays a URL string when the body parameter is omitted.
		body = l.text(destinationArg.span, destination.text)
	} else {
		if emptyAST(bodyArg.node) {
			return l.placeholder(call.span, "missing link body")
		}
		if isNoneAST(bodyArg.node) {
			l.errorAt(bodyArg.span, source.CodeInvalidValue, "link body expects content, found none")
			return l.placeholder(call.span, "invalid link body")
		}
		if value, ok := l.evalValue(bodyArg.node); ok && value.kind != valueContent {
			l.errorAt(bodyArg.span, source.CodeInvalidValue,
				"link body expects content, found %s", value.typeName())
			return l.placeholder(call.span, "invalid link body")
		}
		body = l.lowerArgument(bodyArg.node)
		if body == nil {
			l.errorAt(bodyArg.span, source.CodeInvalidValue, "link body expects content")
			return l.placeholder(call.span, "invalid link body")
		}
	}
	l.warningAt(call.span, source.CodeUnsupported, "link destination is not represented by the neutral math IR")
	return body
}

func emptyAST(node *astNode) bool {
	return node == nil || node.kind == astRow && len(node.items) == 0
}

func (l *lowerer) linkArguments(call *astNode) (destination, body *astArg, valid bool) {
	seenNamed := make(map[string]bool)
	for i := range call.args {
		arg := &call.args[i]
		if arg.kind != argNamed {
			continue
		}
		if (arg.name != "dest" && arg.name != "body") || seenNamed[arg.name] {
			return nil, nil, false
		}
		seenNamed[arg.name] = true
		switch arg.name {
		case "dest":
			destination = arg
		case "body":
			body = arg
		}
	}

	positional := l.positionalArguments(call)
	position := 0
	if destination == nil {
		if position >= len(positional) {
			return destination, body, true
		}
		destination = &positional[position]
		position++
	}
	if body == nil {
		if position >= len(positional) {
			return destination, body, true
		}
		body = &positional[position]
		position++
	}
	return destination, body, position == len(positional)
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
		row := l.newRow(n.span)
		if row == nil {
			return nil
		}
		previousEnd := -1
		for _, item := range items {
			if item.kind == astLinebreak || item.kind == astAlign {
				continue
			}
			if previousEnd >= 0 && item.span.Start > previousEnd && hasWhitespace(l.sourceText(source.NewRange(previousEnd, item.span.Start))) {
				row.Items = append(row.Items, l.text(source.NewRange(previousEnd, item.span.Start), " "))
				if l.exhausted {
					return nil
				}
			}
			row.Items = append(row.Items, l.lowerTextBody(item))
			if l.exhausted {
				return nil
			}
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
	if l.exhausted {
		return nil
	}
	box := l.newBox(call.span, body)
	if box == nil {
		return nil
	}
	if arg := namedArgument(call, "inset"); arg != nil {
		if padding, ok := l.valueSides(arg.node); ok {
			box.Padding = padding
		} else {
			l.errorAt(arg.span, source.CodeInvalidValue, "expected length or side dictionary, found %s", l.valueType(arg.node))
		}
	}
	if arg := namedArgument(call, "stroke"); arg != nil {
		if stroke, ok := l.valueStroke(arg.node); ok {
			box.HasBorder = true
			box.Border = stroke.paint
			box.BorderWidth = stroke.width
		} else if !isNoneAST(arg.node) {
			l.errorAt(arg.span, source.CodeInvalidValue, "expected color, length, or stroke")
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

type strokeValue struct {
	paint ir.Paint
	width ir.Length
}

func (l *lowerer) valueStroke(n *astNode) (strokeValue, bool) {
	if width, ok := l.valueLength(n); ok {
		return strokeValue{paint: ir.Paint{Role: ir.PaintInk}, width: width}, true
	}
	if paint, ok := l.valuePaint(n); ok {
		return strokeValue{paint: paint, width: builtinEmLength(0.05)}, true
	}
	v, ok := l.evalValue(n)
	if !ok || v.kind != valueDictionary {
		return strokeValue{}, false
	}
	stroke := strokeValue{paint: ir.Paint{Role: ir.PaintInk}, width: builtinEmLength(0.05)}
	if thickness, ok := v.entry("thickness"); ok {
		switch thickness.kind {
		case valueLength:
			stroke.width = thickness.length
		case valueAuto:
		default:
			return strokeValue{}, false
		}
	}
	if value, ok := v.entry("paint"); ok {
		switch value.kind {
		case valueColor:
			stroke.paint = value.color
		case valueAuto:
		default:
			return strokeValue{}, false
		}
	}
	return stroke, true
}

func (l *lowerer) valueSides(n *astNode) ([4]ir.Length, bool) {
	if length, ok := l.valueLength(n); ok {
		return [4]ir.Length{length, length, length, length}, true
	}
	v, ok := l.evalValue(n)
	if !ok || v.kind != valueDictionary {
		return [4]ir.Length{}, false
	}
	var sides [4]ir.Length
	read := func(name string, target *ir.Length) bool {
		value, ok := v.entry(name)
		if !ok {
			return true
		}
		if value.kind == valueNone || value.kind == valueAuto {
			return true
		}
		if value.kind != valueLength {
			return false
		}
		*target = value.length
		return true
	}
	if !read("left", &sides[0]) || !read("top", &sides[1]) ||
		!read("right", &sides[2]) || !read("bottom", &sides[3]) {
		return [4]ir.Length{}, false
	}
	if value, ok := v.entry("x"); ok {
		if value.kind != valueLength {
			return [4]ir.Length{}, false
		}
		sides[0], sides[2] = value.length, value.length
	}
	if value, ok := v.entry("y"); ok {
		if value.kind != valueLength {
			return [4]ir.Length{}, false
		}
		sides[1], sides[3] = value.length, value.length
	}
	if value, ok := v.entry("rest"); ok {
		if value.kind != valueLength {
			return [4]ir.Length{}, false
		}
		sides = [4]ir.Length{value.length, value.length, value.length, value.length}
	}
	return sides, true
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
	space := l.newSpace(call.span, ir.SpaceExplicit, amount)
	if space == nil {
		return nil
	}
	if weak := namedArgument(call, "weak"); weak != nil {
		if value, ok := l.valueBool(weak.node); ok {
			space.Weak = value
		}
	}
	return space
}

func (l *lowerer) lowerHide(call *astNode) ir.Node {
	body := l.contentArg(call, "body", 0, true)
	if l.exhausted {
		return nil
	}
	return l.newPhantom(call.span, body)
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
		return l.monospaceValueText(span, "auto")
	case valueBool:
		if v.boolean {
			return l.monospaceValueText(span, "true")
		}
		return l.monospaceValueText(span, "false")
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
		return l.monospaceValueText(span, formatColor(v.color))
	case valueLength:
		text := l.sourceText(span)
		if text == "" {
			text = formatLength(v.length)
		}
		return l.monospaceValueText(span, text)
	case valueArray:
		text := l.sourceText(span)
		if text == "" {
			text = formatArray(v.array)
		}
		return l.monospaceValueText(span, text)
	case valueDictionary:
		return l.monospaceValueText(span, l.sourceText(span))
	default:
		return l.placeholder(span, v.typeName()+" used where content was expected")
	}
}

func (l *lowerer) monospaceValueText(span source.Range, text string) ir.Node {
	if text == "" {
		return l.placeholder(span, "empty value representation")
	}
	node := l.text(span, text)
	if rendered, ok := node.(*ir.Text); ok {
		rendered.Font = ir.FontText
		rendered.Variant = ir.VariantMonospace
	}
	return node
}

func formatColor(paint ir.Paint) string {
	if !paint.HasExplicit {
		return "rgb(\"#000000\")"
	}
	return fmt.Sprintf("rgb(\"#%02x%02x%02x\")", paint.Explicit.R, paint.Explicit.G, paint.Explicit.B)
}

func formatLength(length ir.Length) string {
	units := map[ir.LengthUnit]string{
		ir.LengthZero: "0", ir.LengthEm: "em", ir.LengthEx: "ex", ir.LengthMu: "mu",
		ir.LengthPt: "pt", ir.LengthDip: "pt", ir.LengthPercent: "%",
	}
	return strconv.FormatFloat(length.Float(), 'g', -1, 64) + units[length.Unit]
}

func formatArray(values []value) string {
	var builder strings.Builder
	builder.WriteByte('(')
	for i, item := range values {
		if i > 0 {
			builder.WriteString(", ")
		}
		builder.WriteString(valueText(item))
	}
	builder.WriteByte(')')
	return builder.String()
}
