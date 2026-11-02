package typst

import (
	"math"
	"strconv"
	"strings"

	"github.com/caelyreth/untie/ir"
)

type valueKind uint8

const (
	valueInvalid valueKind = iota
	valueNone
	valueAuto
	valueBool
	valueInteger
	valueFloat
	valueString
	valueLength
	valueArray
	valueDictionary
	valueSymbol
	valueColor
	valueContent
)

// value is the deliberately small Typst value domain needed by math calls.
// Dictionaries retain insertion order; observable behavior never depends on Go
// map iteration.
type value struct {
	kind valueKind

	boolean bool
	integer int64
	float   float64
	text    string
	length  ir.Length
	symbol  string
	color   ir.Paint
	array   []value
	entries []dictEntry
	content *astNode
}

type dictEntry struct {
	key   string
	value value
}

func noneValue() value              { return value{kind: valueNone} }
func autoValue() value              { return value{kind: valueAuto} }
func boolValue(v bool) value        { return value{kind: valueBool, boolean: v} }
func integerValue(v int64) value    { return value{kind: valueInteger, integer: v} }
func floatValue(v float64) value    { return value{kind: valueFloat, float: v} }
func stringValue(v string) value    { return value{kind: valueString, text: v} }
func lengthValue(v ir.Length) value { return value{kind: valueLength, length: v} }
func arrayValue(v []value) value    { return value{kind: valueArray, array: append([]value(nil), v...)} }
func dictionaryValue(v []dictEntry) value {
	return value{kind: valueDictionary, entries: append([]dictEntry(nil), v...)}
}
func symbolValue(v string) value    { return value{kind: valueSymbol, symbol: v} }
func colorValue(v ir.Paint) value   { return value{kind: valueColor, color: v} }
func contentValue(v *astNode) value { return value{kind: valueContent, content: v} }

func (v value) typeName() string {
	switch v.kind {
	case valueNone:
		return "none"
	case valueAuto:
		return "auto"
	case valueBool:
		return "boolean"
	case valueInteger:
		return "integer"
	case valueFloat:
		return "float"
	case valueString:
		return "string"
	case valueLength:
		return "length"
	case valueArray:
		return "array"
	case valueDictionary:
		return "dictionary"
	case valueSymbol:
		return "symbol"
	case valueColor:
		return "color"
	case valueContent:
		return "content"
	default:
		return "invalid"
	}
}

func (v value) entry(name string) (value, bool) {
	for _, entry := range v.entries {
		if entry.key == name {
			return entry.value, true
		}
	}
	return value{}, false
}

func parseNumericLiteral(text string) (value, bool) {
	if strings.ContainsAny(text, ".eE") {
		v, err := strconv.ParseFloat(text, 64)
		if err != nil || math.IsInf(v, 0) || math.IsNaN(v) {
			return value{}, false
		}
		return floatValue(v), true
	}
	v, err := strconv.ParseInt(text, 10, 64)
	if err != nil {
		return value{}, false
	}
	return integerValue(v), true
}

func lengthFromParts(number, unit string) (ir.Length, bool) {
	v, err := strconv.ParseFloat(number, 64)
	if err != nil || math.IsInf(v, 0) || math.IsNaN(v) {
		return ir.Length{}, false
	}
	switch unit {
	case "em":
		return ir.EmLength(v), true
	case "ex":
		return ir.ExLength(v), true
	case "pt":
		return ir.PtLength(v), true
	case "mm":
		// One inch is 72 PostScript points and 96 host DIPs.
		return ir.DipLength(v * 96.0 / 25.4), true
	case "cm":
		return ir.DipLength(v * 96.0 / 2.54), true
	case "in":
		return ir.DipLength(v * 96.0), true
	case "%":
		return ir.PercentLength(v), true
	default:
		return ir.Length{}, false
	}
}

var namedColors = map[string]ir.RGBA{
	"aqua":    {R: 0x00, G: 0xff, B: 0xff, A: 0xff},
	"black":   {R: 0x00, G: 0x00, B: 0x00, A: 0xff},
	"blue":    {R: 0x00, G: 0x00, B: 0xff, A: 0xff},
	"eastern": {R: 0x23, G: 0x9e, B: 0xa3, A: 0xff},
	"fuchsia": {R: 0xff, G: 0x00, B: 0xff, A: 0xff},
	"gray":    {R: 0x80, G: 0x80, B: 0x80, A: 0xff},
	"green":   {R: 0x00, G: 0x80, B: 0x00, A: 0xff},
	"lime":    {R: 0x00, G: 0xff, B: 0x00, A: 0xff},
	"maroon":  {R: 0x80, G: 0x00, B: 0x00, A: 0xff},
	"navy":    {R: 0x00, G: 0x00, B: 0x80, A: 0xff},
	"olive":   {R: 0x80, G: 0x80, B: 0x00, A: 0xff},
	"orange":  {R: 0xff, G: 0xa5, B: 0x00, A: 0xff},
	"purple":  {R: 0x80, G: 0x00, B: 0x80, A: 0xff},
	"red":     {R: 0xff, G: 0x00, B: 0x00, A: 0xff},
	"silver":  {R: 0xc0, G: 0xc0, B: 0xc0, A: 0xff},
	"teal":    {R: 0x00, G: 0x80, B: 0x80, A: 0xff},
	"white":   {R: 0xff, G: 0xff, B: 0xff, A: 0xff},
	"yellow":  {R: 0xff, G: 0xff, B: 0x00, A: 0xff},
}

func namedColor(name string) (value, bool) {
	c, ok := namedColors[name]
	if !ok {
		return value{}, false
	}
	return colorValue(ir.Paint{Explicit: c, HasExplicit: true}), true
}
