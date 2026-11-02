package tex

import (
	"strconv"
	"strings"

	"github.com/caelyreth/untie/ir"
	"github.com/caelyreth/untie/source"
)

var namedColors = map[string]ir.RGBA{
	"aqua":    {R: 0, G: 255, B: 255, A: 255},
	"black":   {R: 0, G: 0, B: 0, A: 255},
	"blue":    {R: 0, G: 0, B: 255, A: 255},
	"brown":   {R: 165, G: 42, B: 42, A: 255},
	"cyan":    {R: 0, G: 255, B: 255, A: 255},
	"fuchsia": {R: 255, G: 0, B: 255, A: 255},
	"gray":    {R: 128, G: 128, B: 128, A: 255},
	"green":   {R: 0, G: 128, B: 0, A: 255},
	"grey":    {R: 128, G: 128, B: 128, A: 255},
	"lime":    {R: 0, G: 255, B: 0, A: 255},
	"magenta": {R: 255, G: 0, B: 255, A: 255},
	"maroon":  {R: 128, G: 0, B: 0, A: 255},
	"navy":    {R: 0, G: 0, B: 128, A: 255},
	"olive":   {R: 128, G: 128, B: 0, A: 255},
	"orange":  {R: 255, G: 165, B: 0, A: 255},
	"purple":  {R: 128, G: 0, B: 128, A: 255},
	"red":     {R: 255, G: 0, B: 0, A: 255},
	"silver":  {R: 192, G: 192, B: 192, A: 255},
	"teal":    {R: 0, G: 128, B: 128, A: 255},
	"white":   {R: 255, G: 255, B: 255, A: 255},
	"yellow":  {R: 255, G: 255, B: 0, A: 255},
}

func (p *parser) parseColorCommand(command token, stop stopMode) astNode {
	paint := p.parseColorArgument(command)
	body := p.parseSequence(stop)
	return &astColorNode{
		astBase: newASTBaseAt(astColor, source.Merge(command.span, body.base().span), tokenOrigin(command)),
		body:    body,
		paint:   paint,
	}
}

func (p *parser) parseTextColorCommand(command token, textMode bool) astNode {
	paint := p.parseColorArgument(command)
	var body astNode
	if textMode {
		body = p.parseTextArgument(command)
	} else {
		body = p.parseRequiredArgument(command)
	}
	return &astColorNode{
		astBase: newASTBaseAt(astColor, source.Merge(command.span, body.base().span), tokenOrigin(command)),
		body:    body,
		paint:   paint,
	}
}

func (p *parser) parseColorBox(command token) astNode {
	if command.text == "\\fcolorbox" {
		border := p.parseColorArgument(command)
		fill := p.parseColorArgument(command)
		body := p.parseTextArgument(command)
		return &astBoxNode{
			astBase:   newASTBaseAt(astBox, source.Merge(command.span, body.base().span), tokenOrigin(command)),
			body:      body,
			fill:      fill,
			hasFill:   true,
			border:    border,
			hasBorder: true,
		}
	}
	fill := p.parseColorArgument(command)
	body := p.parseTextArgument(command)
	return &astBoxNode{
		astBase: newASTBaseAt(astBox, source.Merge(command.span, body.base().span), tokenOrigin(command)),
		body:    body,
		fill:    fill,
		hasFill: true,
	}
}

func (p *parser) parseFramedBox(command token) astNode {
	var body astNode
	if command.text == "\\fbox" {
		body = p.parseTextArgument(command)
	} else {
		body = p.parseRequiredArgument(command)
	}
	return &astBoxNode{
		astBase:   newASTBaseAt(astBox, source.Merge(command.span, body.base().span), tokenOrigin(command)),
		body:      body,
		border:    ir.Paint{Role: ir.PaintInk},
		hasBorder: true,
	}
}

func (p *parser) parsePhantom(command token) astNode {
	body := p.parseRequiredArgument(command)
	return &astPhantomNode{
		astBase: newASTBaseAt(astPhantom, source.Merge(command.span, body.base().span), tokenOrigin(command)),
		body:    body,
		width:   command.text != "\\vphantom",
		height:  command.text != "\\hphantom",
		depth:   command.text != "\\hphantom",
	}
}

func (p *parser) parseMathClass(command token) astNode {
	class := ir.ClassNormal
	limits := ir.LimitsAuto
	switch command.text {
	case "\\mathbin":
		class = ir.ClassBinary
	case "\\mathrel":
		class = ir.ClassRelation
	case "\\mathopen":
		class = ir.ClassOpening
	case "\\mathclose":
		class = ir.ClassClosing
	case "\\mathpunct":
		class = ir.ClassPunctuation
	case "\\mathinner":
		class = ir.ClassInner
	case "\\mathop":
		class = ir.ClassLarge
	default:
		limits = ir.LimitsNever
	}
	body := p.parseRequiredArgument(command)
	return &astClassNode{
		astBase: newASTBaseAt(astClass, source.Merge(command.span, body.base().span), tokenOrigin(command)),
		body:    body,
		class:   class,
		limits:  limits,
	}
}

func (p *parser) parseRule(command token) astNode {
	_, _, _ = p.consumeRawOptionalGroup()
	widthText, widthSpan, widthOK := p.consumeRawGroupTextChecked(command)
	heightText, heightSpan, heightOK := p.consumeRawGroupTextChecked(command)
	span := source.Merge(widthSpan, heightSpan)
	if !widthOK || !heightOK {
		p.report(source.Diagnostic{
			Range:    normalizeRange(span),
			Severity: source.SeverityError,
			Code:     source.CodeMissingArgument,
			Message:  "\\rule requires width and height",
		})
		return newPlaceholderAST(span, "missing rule dimension")
	}
	width, widthValid := parseLengthString(strings.TrimSpace(widthText))
	height, heightValid := parseLengthString(strings.TrimSpace(heightText))
	if !widthValid || !heightValid || width.Value < 0 || height.Value < 0 {
		p.report(source.Diagnostic{
			Range:    normalizeRange(span),
			Severity: source.SeverityError,
			Code:     source.CodeInvalidValue,
			Message:  "\\rule requires non-negative TeX dimensions",
		})
		return newPlaceholderAST(span, "invalid rule dimension")
	}
	return &astRuleNode{
		astBase: newASTBaseAt(astRule, source.Merge(command.span, span), tokenOrigin(command)),
		width:   width,
		height:  height,
	}
}

func (p *parser) parseColorArgument(owner token) ir.Paint {
	text, span, ok := p.consumeRawGroupTextChecked(owner)
	if !ok {
		p.report(source.Diagnostic{
			Range:    normalizeRange(span),
			Severity: source.SeverityError,
			Code:     source.CodeInvalidValue,
			Message:  "invalid color argument to '" + owner.text + "'",
		})
		return ir.Paint{Role: ir.PaintInk}
	}
	paint, ok := parseColor(text)
	if !ok {
		p.report(source.Diagnostic{
			Range:    normalizeRange(span),
			Severity: source.SeverityError,
			Code:     source.CodeInvalidValue,
			Message:  "invalid color '" + strings.TrimSpace(text) + "'",
		})
		return ir.Paint{Role: ir.PaintInk}
	}
	return paint
}

func parseColor(text string) (ir.Paint, bool) {
	text = strings.ToLower(strings.TrimSpace(text))
	if rgba, ok := namedColors[text]; ok {
		return ir.Paint{Explicit: rgba, HasExplicit: true}, true
	}
	if !strings.HasPrefix(text, "#") {
		return ir.Paint{}, false
	}
	hex := strings.TrimPrefix(text, "#")
	switch len(hex) {
	case 3, 4:
		var expanded strings.Builder
		for _, digit := range hex {
			expanded.WriteRune(digit)
			expanded.WriteRune(digit)
		}
		hex = expanded.String()
	case 6, 8:
	default:
		return ir.Paint{}, false
	}
	value, err := strconv.ParseUint(hex, 16, 32)
	if err != nil {
		return ir.Paint{}, false
	}
	rgba := ir.RGBA{A: 255}
	if len(hex) == 8 {
		rgba.R = uint8(value >> 24)
		rgba.G = uint8(value >> 16)
		rgba.B = uint8(value >> 8)
		rgba.A = uint8(value)
	} else {
		rgba.R = uint8(value >> 16)
		rgba.G = uint8(value >> 8)
		rgba.B = uint8(value)
	}
	return ir.Paint{Explicit: rgba, HasExplicit: true}, true
}
