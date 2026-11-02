package typst

import (
	"math"
	"strconv"
	"strings"

	"github.com/caelyreth/untie/ir"
	"github.com/caelyreth/untie/source"
)

func (l *lowerer) evalValue(n *astNode) (value, bool) {
	if n == nil {
		return value{}, false
	}
	if n.kind == astRow && len(n.items) == 1 {
		return l.evalValue(n.items[0])
	}
	switch n.kind {
	case astLiteral:
		return n.value, true
	case astString:
		return stringValue(n.text), true
	case astNumber:
		if n.value.kind != valueInvalid {
			return n.value, true
		}
		if v, ok := parseNumericLiteral(n.text); ok {
			return v, true
		}
	case astGlyph, astShorthand:
		return symbolValue(n.text), true
	case astIdentifier, astField:
		if v, ok := codeKeywordValue(n.text); ok {
			return v, true
		}
		if v, ok := namedColor(n.text); ok {
			return v, true
		}
		switch n.text {
		case "left", "right", "center", "start", "end", "top", "bottom", "horizon":
			return symbolValue(n.text), true
		}
		if def, ok := symbol(n.text); ok {
			return symbolValue(def.text), true
		}
		return contentValue(n), true
	case astCode:
		if len(n.items) == 1 {
			return l.evalValue(n.items[0])
		}
	case astArray:
		values := make([]value, 0, len(n.items))
		for _, item := range n.items {
			v, ok := l.evalValue(item)
			if !ok {
				l.errorAt(item.span, source.CodeInvalidValue, "expected value, found %s", l.valueType(item))
				v = noneValue()
			}
			values = append(values, v)
		}
		return arrayValue(values), true
	case astDict:
		entries := make([]dictEntry, 0, len(n.args))
		for _, arg := range n.args {
			v, ok := l.evalValue(arg.node)
			if !ok {
				l.errorAt(arg.span, source.CodeInvalidValue, "expected value for %s, found %s", arg.name, l.valueType(arg.node))
				v = noneValue()
			}
			entries = append(entries, dictEntry{key: arg.name, value: v})
		}
		return dictionaryValue(entries), true
	case astUnary:
		if len(n.items) != 1 {
			return value{}, false
		}
		v, ok := l.evalValue(n.items[0])
		if !ok {
			return value{}, false
		}
		switch v.kind {
		case valueInteger:
			v.integer = -v.integer
			return v, true
		case valueFloat:
			v.float = -v.float
			return v, true
		case valueLength:
			v.length.Value = -v.length.Value
			return v, true
		default:
			return value{}, false
		}
	case astCall:
		switch callName(n) {
		case "rgb":
			return l.evalRGB(n)
		case "luma":
			return l.evalLuma(n)
		default:
			return contentValue(n), true
		}
	case astDelimited:
		return contentValue(n), true
	}
	return value{}, false
}

func (l *lowerer) valueType(n *astNode) string {
	if v, ok := l.evalValue(n); ok {
		return v.typeName()
	}
	return "content"
}

func (l *lowerer) valueString(n *astNode) (string, bool) {
	v, ok := l.evalValue(n)
	if !ok {
		return "", false
	}
	switch v.kind {
	case valueString:
		return v.text, true
	case valueSymbol:
		return v.symbol, true
	case valueContent:
		if text := nodeText(l.lower(v.content)); text != "" {
			return text, true
		}
	}
	return "", false
}

func (l *lowerer) valueBool(n *astNode) (bool, bool) {
	v, ok := l.evalValue(n)
	if !ok || v.kind != valueBool {
		return false, false
	}
	return v.boolean, true
}

func (l *lowerer) valueInteger(n *astNode) (int64, bool) {
	v, ok := l.evalValue(n)
	if !ok || v.kind != valueInteger {
		return 0, false
	}
	return v.integer, true
}

func (l *lowerer) valueLength(n *astNode) (ir.Length, bool) {
	v, ok := l.evalValue(n)
	if !ok || v.kind != valueLength {
		return ir.Length{}, false
	}
	return v.length, true
}

func (l *lowerer) valuePaint(n *astNode) (ir.Paint, bool) {
	v, ok := l.evalValue(n)
	if !ok {
		return ir.Paint{}, false
	}
	switch v.kind {
	case valueColor:
		return v.color, true
	case valueString:
		return parseHexColor(v.text)
	case valueDictionary:
		if paint, ok := v.entry("paint"); ok && paint.kind == valueColor {
			return paint.color, true
		}
	}
	return ir.Paint{}, false
}

func parseHexColor(text string) (ir.Paint, bool) {
	text = strings.TrimSpace(text)
	if !strings.HasPrefix(text, "#") {
		return ir.Paint{}, false
	}
	hex := text[1:]
	if len(hex) == 3 || len(hex) == 4 {
		var expanded strings.Builder
		for _, r := range hex {
			expanded.WriteRune(r)
			expanded.WriteRune(r)
		}
		hex = expanded.String()
	}
	if len(hex) != 6 && len(hex) != 8 {
		return ir.Paint{}, false
	}
	value, err := strconv.ParseUint(hex, 16, 32)
	if err != nil {
		return ir.Paint{}, false
	}
	var c ir.RGBA
	if len(hex) == 6 {
		c = ir.RGBA{R: uint8(value >> 16), G: uint8(value >> 8), B: uint8(value), A: 0xff}
	} else {
		c = ir.RGBA{R: uint8(value >> 24), G: uint8(value >> 16), B: uint8(value >> 8), A: uint8(value)}
	}
	return ir.Paint{Explicit: c, HasExplicit: true}, true
}

func (l *lowerer) evalRGB(call *astNode) (value, bool) {
	positional := l.positionalArguments(call)
	if hexArg := namedArgument(call, "hex"); hexArg != nil {
		if hex, ok := l.valueString(hexArg.node); ok {
			if paint, ok := parseHexColor(hex); ok {
				return colorValue(paint), true
			}
		}
		return value{}, false
	}
	if len(positional) == 1 {
		if hex, ok := l.valueString(positional[0].node); ok {
			if paint, ok := parseHexColor(hex); ok {
				return colorValue(paint), true
			}
		}
	}
	channel := func(name string, index int, fallback uint8) uint8 {
		var node *astNode
		if arg := namedArgument(call, name); arg != nil {
			node = arg.node
		} else if index < len(positional) {
			node = positional[index].node
		}
		if node == nil {
			return fallback
		}
		v, ok := l.evalValue(node)
		if !ok {
			return fallback
		}
		switch v.kind {
		case valueInteger:
			return clampByte(v.integer)
		case valueFloat:
			return clampByte(int64(math.Round(v.float * 255)))
		case valueLength:
			if v.length.Unit == ir.LengthPercent {
				return clampByte(int64(math.Round(v.length.Float() * 2.55)))
			}
		}
		return fallback
	}
	c := ir.RGBA{
		R: channel("red", 0, 0),
		G: channel("green", 1, 0),
		B: channel("blue", 2, 0),
		A: channel("alpha", 3, 0xff),
	}
	return colorValue(ir.Paint{Explicit: c, HasExplicit: true}), true
}

func (l *lowerer) evalLuma(call *astNode) (value, bool) {
	positional := l.positionalArguments(call)
	if len(positional) == 0 {
		return colorValue(ir.Paint{Explicit: ir.RGBA{A: 0xff}, HasExplicit: true}), true
	}
	v, ok := l.evalValue(positional[0].node)
	if !ok {
		return value{}, false
	}
	var lightness uint8
	switch v.kind {
	case valueInteger:
		lightness = clampByte(v.integer)
	case valueFloat:
		lightness = clampByte(int64(math.Round(v.float * 255)))
	case valueLength:
		lightness = clampByte(int64(math.Round(v.length.Float() * 2.55)))
	default:
		return value{}, false
	}
	return colorValue(ir.Paint{
		Explicit:    ir.RGBA{R: lightness, G: lightness, B: lightness, A: 0xff},
		HasExplicit: true,
	}), true
}

func clampByte(v int64) uint8 {
	if v < 0 {
		return 0
	}
	if v > 255 {
		return 255
	}
	return uint8(v)
}
