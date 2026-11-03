package tex

import (
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/caelyreth/untie/ir"
	"github.com/caelyreth/untie/source"
)

func (p *parser) parseDelimitedArgument(command token) astNode {
	var openText, closeText string
	switch command.text {
	case "\\bra", "\\Bra":
		openText, closeText = "⟨", "|"
	case "\\ket", "\\Ket":
		openText, closeText = "|", "⟩"
	case "\\braket", "\\Braket", "\\angl":
		openText, closeText = "⟨", "⟩"
	case "\\set":
		openText, closeText = "{", "}"
	case "\\Set":
		openText, closeText = "⦃", "⦄"
	}
	body := p.parseRequiredArgument(command)
	return &astFencedNode{
		astBase:  newASTBaseAt(astFenced, source.Merge(command.span, body.base().span), tokenOrigin(command)),
		open:     p.newGlyphAST(command.span, openText, ir.ClassOpening),
		close:    p.newGlyphAST(command.span, closeText, ir.ClassClosing),
		body:     []astNode{body},
		balanced: true,
	}
}

func (p *parser) parseSubstack(command token) astNode {
	p.metadataDepth++
	defer func() { p.metadataDepth-- }()
	p.exp.consumeSpaces()
	open := p.exp.peek()
	if !open.is(tokBeginGroup) {
		p.report(source.Diagnostic{
			Range:    normalizeRange(command.span),
			Severity: source.SeverityError,
			Code:     source.CodeMissingArgument,
			Message:  "\\substack requires a braced argument",
		})
		return newPlaceholderAST(command.span, "missing substack body")
	}
	p.exp.next()
	span := open.span
	var rows [][]astNode
	cells := 0
	for {
		if cells >= p.limits.MaxTableCells {
			t := p.exp.peek()
			p.limit(normalizeRange(t.span), "TeX table cell limit exceeded")
			span = p.discardSubstackTail(open, span)
			return &astMultilineNode{
				astBase:  newASTBaseAt(astMultiline, source.Merge(command.span, span), tokenOrigin(command)),
				rows:     rows,
				centered: true,
			}
		}
		cell := p.parseTableCell()
		span = source.Merge(span, cell.base().span)
		cells++
		rows = append(rows, []astNode{cell})
		t := p.exp.peek()
		switch {
		case t.isControl() && isRowBreak(t.text):
			p.exp.next()
			span = source.Merge(span, t.span)
			p.consumeRowBreakOptions()
		case t.is(tokEndGroup):
			p.exp.next()
			span = source.Merge(span, t.span)
			return &astMultilineNode{
				astBase:  newASTBaseAt(astMultiline, source.Merge(command.span, span), tokenOrigin(command)),
				rows:     rows,
				centered: true,
			}
		case t.isEOF():
			p.report(source.Diagnostic{
				Range:    normalizeRange(open.span),
				Severity: source.SeverityError,
				Code:     source.CodeUnclosedGroup,
				Message:  "unclosed \\substack group",
			})
			return &astMultilineNode{
				astBase:  newASTBaseAt(astMultiline, source.Merge(command.span, span), tokenOrigin(command)),
				rows:     rows,
				centered: true,
			}
		default:
			p.report(source.Diagnostic{
				Range:    normalizeRange(t.span),
				Severity: source.SeverityError,
				Code:     source.CodeUnexpectedToken,
				Message:  "unexpected token in \\substack",
			})
			p.exp.next()
		}
	}
}

func (p *parser) discardSubstackTail(open token, span source.Range) source.Range {
	depth := 1
	for depth > 0 {
		t := p.exp.next()
		if t.isEOF() {
			p.report(source.Diagnostic{
				Range:    normalizeRange(open.span),
				Severity: source.SeverityError,
				Code:     source.CodeUnclosedGroup,
				Message:  "unclosed \\substack group",
			})
			return source.Merge(span, t.span)
		}
		span = source.Merge(span, t.span)
		switch t.kind {
		case tokBeginGroup:
			depth++
		case tokEndGroup:
			depth--
		}
	}
	return span
}

func (p *parser) parseSideSet(command token, stop stopMode) astNode {
	leftTop, leftBottom, leftSpan, leftOK := p.parseScriptGroup(command, "left")
	rightTop, rightBottom, rightSpan, rightOK := p.parseScriptGroup(command, "right")
	if !leftOK || !rightOK {
		return newPlaceholderAST(source.Merge(command.span, rightSpan), "missing sideset script group")
	}

	base := p.parseAtom(stop)
	if base == nil {
		p.report(source.Diagnostic{
			Range:    normalizeRange(source.Merge(command.span, rightSpan)),
			Severity: source.SeverityError,
			Code:     source.CodeMissingArgument,
			Message:  "\\sideset requires a base expression",
		})
		return newPlaceholderAST(source.Merge(command.span, rightSpan), "missing sideset base")
	}

	span := source.Merge(command.span, leftSpan)
	span = source.Merge(span, rightSpan)
	span = source.Merge(span, base.base().span)
	limitsBase := &astScriptsNode{
		astBase: newASTBaseAt(astScripts, base.base().span, tokenOrigin(command)),
		main:    base,
		limits:  ir.LimitsAlways,
		role:    ir.ScriptRoleLimits,
	}
	return &astScriptsNode{
		astBase:     newASTBaseAt(astScripts, span, tokenOrigin(command)),
		main:        limitsBase,
		topLeft:     leftTop,
		bottomLeft:  leftBottom,
		topRight:    rightTop,
		bottomRight: rightBottom,
		limits:      ir.LimitsAuto,
		role:        ir.ScriptRoleLimits,
	}
}

func (p *parser) parseScriptGroup(owner token, side string) (top, bottom astNode, span source.Range, ok bool) {
	p.metadataDepth++
	defer func() { p.metadataDepth-- }()
	p.exp.consumeSpaces()
	open := p.exp.peek()
	if !open.is(tokBeginGroup) {
		p.report(source.Diagnostic{
			Range:    normalizeRange(open.span),
			Severity: source.SeverityError,
			Code:     source.CodeMissingArgument,
			Message:  "\\sideset requires two script groups",
		})
		return nil, nil, open.span, false
	}
	p.exp.next()
	span = open.span
	for {
		p.exp.consumeSpaces()
		t := p.exp.peek()
		switch {
		case t.is(tokEndGroup):
			p.exp.next()
			return top, bottom, source.Merge(span, t.span), true
		case t.isEOF():
			p.report(source.Diagnostic{
				Range:    normalizeRange(source.Merge(owner.span, span)),
				Severity: source.SeverityError,
				Code:     source.CodeUnclosedGroup,
				Message:  "unclosed \\sideset " + side + " script group",
			})
			return top, bottom, span, false
		case !t.isControl() && t.is(tokSuperscript):
			p.exp.next()
			arg, argSpan := p.parseScriptArgument("superscript", t)
			if top != nil {
				p.report(source.Diagnostic{
					Range:    normalizeRange(t.span),
					Severity: source.SeverityError,
					Code:     source.CodeUnexpectedToken,
					Message:  "double superscript in \\sideset",
				})
			} else {
				top = arg
			}
			span = source.Merge(span, argSpan)
		case !t.isControl() && t.is(tokSubscript):
			p.exp.next()
			arg, argSpan := p.parseScriptArgument("subscript", t)
			if bottom != nil {
				p.report(source.Diagnostic{
					Range:    normalizeRange(t.span),
					Severity: source.SeverityError,
					Code:     source.CodeUnexpectedToken,
					Message:  "double subscript in \\sideset",
				})
			} else {
				bottom = arg
			}
			span = source.Merge(span, argSpan)
		default:
			p.report(source.Diagnostic{
				Range:    normalizeRange(t.span),
				Severity: source.SeverityError,
				Code:     source.CodeUnexpectedToken,
				Message:  "unexpected token in \\sideset script group",
			})
			p.exp.next()
		}
	}
}

func (p *parser) parseCompositeRelation(command token, glyphs ...string) astNode {
	items := make([]astNode, 0, len(glyphs))
	for _, glyph := range glyphs {
		items = append(items, p.newGlyphAST(command.span, glyph, ir.ClassRelation))
	}
	return &astRowNode{
		astBase: newASTBaseAt(astRow, command.span, tokenOrigin(command)),
		items:   items,
	}
}

func (p *parser) parseGenfrac(command token) astNode {
	openText, openOK := p.consumeRawTokenText()
	closeText, closeOK := p.consumeRawTokenText()
	thickness, _, thicknessOK := p.consumeRawGroupTextChecked(command)
	styleText, _, styleOK := p.consumeRawGroupTextChecked(command)
	numerator := p.parseRequiredArgument(command)
	denominator := p.parseRequiredArgument(command)
	if !openOK || !closeOK || !thicknessOK || !styleOK {
		p.report(source.Diagnostic{
			Range:    normalizeRange(command.span),
			Severity: source.SeverityError,
			Code:     source.CodeMissingArgument,
			Message:  "\\genfrac requires six arguments",
		})
		return newPlaceholderAST(command.span, "missing \\genfrac argument")
	}

	// KaTeX genfrac.ts leaves an empty style group unresolved; 0..3 force
	// display, text, script, and script-script respectively.
	size := ir.SizeInherit
	switch strings.TrimSpace(styleText) {
	case "":
		size = ir.SizeInherit
	case "0":
		size = ir.SizeDisplay
	case "1":
		size = ir.SizeText
	case "2":
		size = ir.SizeScript
	case "3":
		size = ir.SizeScriptScript
	default:
		p.report(source.Diagnostic{
			Range:    normalizeRange(command.span),
			Severity: source.SeverityError,
			Code:     source.CodeInvalidValue,
			Message:  "\\genfrac style must be 0, 1, 2, or 3",
		})
	}
	thickness = strings.TrimSpace(thickness)
	bar := true
	if thickness != "" {
		rule, valid := parseLengthString(thickness)
		if !valid || rule.Value < 0 {
			p.report(source.Diagnostic{
				Range:    normalizeRange(command.span),
				Severity: source.SeverityError,
				Code:     source.CodeInvalidValue,
				Message:  "invalid \\genfrac rule thickness",
			})
		}
		bar = valid && rule.Value != 0
		if bar {
			p.report(source.Diagnostic{
				Range:    normalizeRange(command.span),
				Severity: source.SeverityWarning,
				Code:     source.CodeUnsupported,
				Message:  "custom \\genfrac rule thickness is not represented in the neutral IR",
			})
		}
	}
	span := source.Merge(command.span, denominator.base().span)
	fraction := &astFractionNode{
		astBase:     newASTBaseAt(astFraction, span, tokenOrigin(command)),
		numerator:   numerator,
		denominator: denominator,
		bar:         bar,
		style:       ir.FractionVertical,
		size:        size,
	}
	if !bar {
		fraction.role = ir.FractionRoleStacked
	}
	if strings.TrimSpace(openText) == "" && strings.TrimSpace(closeText) == "" {
		return fraction
	}
	return &astFencedNode{
		astBase:  newASTBaseAt(astFenced, span, tokenOrigin(command)),
		open:     p.rawDelimiter(command.span, openText, ir.ClassOpening),
		close:    p.rawDelimiter(command.span, closeText, ir.ClassClosing),
		body:     []astNode{fraction},
		balanced: true,
	}
}

func (p *parser) consumeRawTokenText() (string, bool) {
	p.exp.consumeSpaces()
	t := p.exp.popRaw()
	if t.isEOF() || t.is(tokSpace) {
		p.exp.push(t)
		return "", false
	}
	if t.is(tokBeginGroup) {
		body, _ := p.consumeRawGroupAfter(t)
		var text strings.Builder
		for _, token := range body {
			text.WriteString(token.text)
		}
		return text.String(), true
	}
	return t.text, true
}

func (p *parser) rawDelimiter(span source.Range, text string, fallback ir.MathClass) astNode {
	text = strings.TrimSpace(text)
	if text == "" || text == "." {
		return nil
	}
	if spec, ok := delimiterCommands[text]; ok {
		return p.newGlyphAST(span, spec.text, fallback)
	}
	if len([]rune(text)) == 1 {
		return p.newGlyphAST(span, text, fallback)
	}
	p.report(source.Diagnostic{
		Range:    normalizeRange(span),
		Severity: source.SeverityError,
		Code:     source.CodeInvalidValue,
		Message:  "invalid \\genfrac delimiter '" + text + "'",
	})
	return nil
}

func (p *parser) parseMathChoice(command token) astNode {
	display := p.parseRequiredArgument(command)
	text := p.parseRequiredArgument(command)
	_ = p.parseRequiredArgument(command)
	_ = p.parseRequiredArgument(command)
	p.report(source.Diagnostic{
		Range:    normalizeRange(command.span),
		Severity: source.SeverityWarning,
		Code:     source.CodeUnsupported,
		Message:  "\\mathchoice is resolved from the root mode because the neutral IR has no style-conditional node",
	})
	if p.mode == ir.Display {
		return display
	}
	return text
}

func (p *parser) parseTMspace(command token) astNode {
	p.exp.consumeSpaces()
	sign := p.exp.next()
	negative := false
	if !sign.isControl() && sign.is(tokChar) && (sign.text == "+" || sign.text == "-") {
		negative = sign.text == "-"
	} else {
		p.exp.push(sign)
	}
	_, _, firstOK := p.consumeRawGroupTextChecked(command)
	text, span, secondOK := p.consumeRawGroupTextChecked(command)
	if !firstOK || !secondOK {
		p.report(source.Diagnostic{
			Range:    normalizeRange(span),
			Severity: source.SeverityError,
			Code:     source.CodeMissingArgument,
			Message:  "\\tmspace requires a sign and two dimensions",
		})
		return newPlaceholderAST(span, "missing tmspace dimension")
	}
	length, ok := parseLengthString(strings.TrimSpace(text))
	if !ok {
		p.report(source.Diagnostic{
			Range:    normalizeRange(span),
			Severity: source.SeverityError,
			Code:     source.CodeInvalidValue,
			Message:  "invalid \\tmspace dimension",
		})
		return newPlaceholderAST(span, "invalid tmspace dimension")
	}
	if negative {
		length.Value = -length.Value
	}
	return &astSpaceNode{
		astBase: newASTBaseAt(astSpace, source.Merge(command.span, span), tokenOrigin(command)),
		kind:    ir.SpaceExplicit,
		amount:  length,
	}
}

func (p *parser) parseBmod(command token) astNode {
	mod := &astTextNode{
		astBase: newASTBaseAt(astText, command.span, tokenOrigin(command)),
		text:    "mod",
		variant: ir.VariantNormal,
	}
	items := []astNode{explicitSpaceAST(command.span, 1), mod, explicitSpaceAST(command.span, 1)}
	row := &astRowNode{
		astBase: newASTBaseAt(astRow, command.span, tokenOrigin(command)),
		items:   items,
	}
	return &astClassNode{
		astBase:   newASTBaseAt(astClass, command.span, tokenOrigin(command)),
		body:      row,
		class:     ir.ClassBinary,
		limits:    ir.LimitsNever,
		setLimits: true,
	}
}

func (p *parser) parseModArgument(command token) astNode {
	arg := p.parseRequiredArgument(command)
	mod := &astOperatorNode{
		astBase: newASTBaseAt(astOperator, command.span, tokenOrigin(command)),
		text:    "mod",
		variant: ir.VariantNormal,
		limits:  ir.LimitsNever,
	}
	var body astNode
	switch command.text {
	case "\\pod":
		body = arg
	case "\\pmod":
		body = &astRowNode{
			astBase: newASTBaseAt(astRow, source.Merge(command.span, arg.base().span), tokenOrigin(command)),
			items:   []astNode{mod, arg},
		}
	case "\\mod":
		body = &astRowNode{
			astBase: newASTBaseAt(astRow, source.Merge(command.span, arg.base().span), tokenOrigin(command)),
			items:   []astNode{mod, arg},
		}
	}
	if command.text == "\\pmod" || command.text == "\\pod" {
		body = &astFencedNode{
			astBase:  newASTBaseAt(astFenced, source.Merge(command.span, arg.base().span), tokenOrigin(command)),
			open:     p.newGlyphAST(command.span, "(", ir.ClassOpening),
			close:    p.newGlyphAST(command.span, ")", ir.ClassClosing),
			body:     []astNode{body},
			balanced: true,
		}
	}
	return &astRowNode{
		astBase: newASTBaseAt(astRow, source.Merge(command.span, arg.base().span), tokenOrigin(command)),
		items:   []astNode{body},
	}
}

func explicitSpaceAST(span source.Range, mus float64) *astSpaceNode {
	amount, _ := ir.MuLength(mus)
	return &astSpaceNode{
		astBase: newASTBaseAt(astSpace, span, span),
		kind:    ir.SpaceExplicit,
		amount:  amount,
	}
}

func explicitEmSpaceAST(span source.Range, ems float64) *astSpaceNode {
	amount, _ := ir.EmLength(ems)
	return &astSpaceNode{
		astBase: newASTBaseAt(astSpace, span, span),
		kind:    ir.SpaceExplicit,
		amount:  amount,
	}
}

func (p *parser) parseVerb(command token) astNode {
	p.exp.consumeSpaces()
	delimiter := p.exp.next()
	if delimiter.isEOF() || delimiter.isControl() {
		p.report(source.Diagnostic{
			Range:    normalizeRange(command.span),
			Severity: source.SeverityError,
			Code:     source.CodeMissingArgument,
			Message:  "\\verb requires a delimiter",
		})
		return newPlaceholderAST(command.span, "missing verb delimiter")
	}
	var text strings.Builder
	span := source.Merge(command.span, delimiter.span)
	for {
		t := p.exp.popRaw()
		if t.isEOF() {
			p.report(source.Diagnostic{
				Range:    normalizeRange(command.span),
				Severity: source.SeverityError,
				Code:     source.CodeUnclosedGroup,
				Message:  "unclosed \\verb delimiter",
			})
			break
		}
		span = source.Merge(span, t.span)
		if !t.isControl() && t.is(tokChar) && t.text == delimiter.text {
			break
		}
		text.WriteString(t.text)
	}
	return astLiteralText(token{span: span, origin: tokenOrigin(command)}, text.String())
}

func (p *parser) parseChar(command token) astNode {
	p.exp.consumeSpaces()
	if marker := p.exp.peek(); marker.is(tokChar) && marker.text == "`" {
		p.exp.next()
		value := p.exp.next()
		if value.isEOF() {
			p.report(source.Diagnostic{
				Range:    normalizeRange(command.span),
				Severity: source.SeverityError,
				Code:     source.CodeMissingArgument,
				Message:  "\\char` requires a character",
			})
			return newPlaceholderAST(command.span, "missing character code")
		}
		text := value.text
		if value.isControl() {
			text = strings.TrimPrefix(text, "\\")
		}
		r, size := utf8.DecodeRuneInString(text)
		if size == 0 || size != len(text) {
			p.report(source.Diagnostic{
				Range:    normalizeRange(value.span),
				Severity: source.SeverityError,
				Code:     source.CodeInvalidValue,
				Message:  "\\char` requires exactly one character",
			})
			return newPlaceholderAST(source.Merge(command.span, value.span), "invalid character code")
		}
		span := source.Merge(command.span, value.span)
		return p.newGlyphASTAt(span, tokenOrigin(command), string(r), symbolForChar(string(r)).class)
	}

	var b strings.Builder
	span := command.span
	characters := 0
	for characters < 12 {
		t := p.exp.peek()
		if t.isEOF() || t.is(tokSpace) || t.is(tokBeginGroup) || t.is(tokEndGroup) || t.isControl() {
			break
		}
		p.exp.next()
		b.WriteString(t.text)
		characters++
		span = source.Merge(span, t.span)
	}
	text := b.String()
	base := 10
	switch {
	case strings.HasPrefix(text, "\""):
		base = 16
		text = strings.TrimPrefix(text, "\"")
	case strings.HasPrefix(text, "'"):
		base = 8
		text = strings.TrimPrefix(text, "'")
	}
	value, err := strconv.ParseInt(text, base, 32)
	if err != nil || value <= 0 || value > 0x10ffff || value >= 0xd800 && value <= 0xdfff {
		p.report(source.Diagnostic{
			Range:    normalizeRange(span),
			Severity: source.SeverityError,
			Code:     source.CodeInvalidValue,
			Message:  "invalid \\char code",
		})
		return newPlaceholderAST(span, "invalid character code")
	}
	return p.newGlyphAST(span, string(rune(value)), symbolForChar(string(rune(value))).class)
}

func (p *parser) parseHTMLMetadataWrapper(command token) astNode {
	_, _, _ = p.consumeRawGroupTextChecked(command)
	body := p.parseRequiredArgument(command)
	p.report(source.Diagnostic{
		Range:    normalizeRange(command.span),
		Severity: source.SeverityWarning,
		Code:     source.CodeUnsupported,
		Message:  "HTML metadata is outside the formula IR and was discarded",
	})
	return body
}

func (p *parser) parseHTMLStyle(command token) astNode {
	style, _, ok := p.consumeRawGroupTextChecked(command)
	body := p.parseRequiredArgument(command)
	p.report(source.Diagnostic{
		Range:    normalizeRange(command.span),
		Severity: source.SeverityWarning,
		Code:     source.CodeUnsupported,
		Message:  "HTML/CSS styling is reduced to supported formula paint",
	})
	if !ok {
		return body
	}
	for _, declaration := range strings.Split(style, ";") {
		key, value, found := strings.Cut(declaration, ":")
		if !found || !strings.EqualFold(strings.TrimSpace(key), "color") {
			continue
		}
		if paint, valid := parseColor(strings.TrimSpace(value)); valid {
			return &astColorNode{
				astBase: newASTBaseAt(astColor, source.Merge(command.span, body.base().span), tokenOrigin(command)),
				body:    body,
				paint:   paint,
			}
		}
	}
	return body
}
