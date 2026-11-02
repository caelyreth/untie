package tex

import (
	"strings"

	"github.com/caelyreth/untie/ir"
	"github.com/caelyreth/untie/source"
)

func (p *parser) parseTextCommand(command token) astNode {
	variant := ir.VariantAuto
	switch command.text {
	case "\\textrm", "\\textnormal":
		variant = ir.VariantNormal
	case "\\textsf":
		variant = ir.VariantSans
	case "\\texttt":
		variant = ir.VariantMonospace
	case "\\textbf":
		variant = ir.VariantBold
	case "\\textmd", "\\textup":
		variant = ir.VariantNormal
	case "\\textit", "\\textsl", "\\emph":
		variant = ir.VariantItalic
	}
	body := p.parseTextArgument(command)
	if body == nil {
		return newPlaceholderAST(normalizeRange(command.span), "missing text argument")
	}
	if variant == ir.VariantAuto {
		return body
	}
	return &astVariantNode{
		astBase: newASTBaseAt(astVariant, source.Merge(command.span, body.base().span), tokenOrigin(command)),
		body:    body,
		variant: variant,
	}
}

func (p *parser) parseOperatorName(command token) astNode {
	limits := ir.LimitsNever
	if command.text == "\\operatornamewithlimits" {
		limits = ir.LimitsAlways
	}
	p.exp.consumeSpaces()
	if star := p.exp.peek(); star.is(tokChar) && star.text == "*" {
		p.exp.next()
		limits = ir.LimitsAlways
	}
	body := p.parseTextArgument(command)
	if body == nil {
		return newPlaceholderAST(normalizeRange(command.span), "missing operator name")
	}
	text, ok := plainText(body)
	text = strings.TrimSpace(text)
	if !ok || text == "" {
		p.report(source.Diagnostic{
			Range:    normalizeRange(body.base().span),
			Severity: source.SeverityError,
			Code:     source.CodeUnsupported,
			Message:  "operator name must contain plain text",
		})
		return newPlaceholderAST(normalizeRange(body.base().span), "unsupported operator name")
	}
	return &astOperatorNode{
		astBase:        newASTBaseAt(astOperator, source.Merge(command.span, body.base().span), tokenOrigin(command)),
		text:           text,
		limits:         limits,
		semanticLimits: limits != ir.LimitsNever,
	}
}

func (p *parser) parseTextArgument(owner token) astNode {
	p.metadataDepth++
	defer func() { p.metadataDepth-- }()
	p.exp.consumeSpaces()
	t := p.exp.peek()
	if t.isEOF() || t.is(tokEndGroup) || t.isControl() && (t.text == "\\right" || t.text == "\\end") {
		p.report(source.Diagnostic{
			Range:    normalizeRange(owner.span),
			Severity: source.SeverityError,
			Code:     source.CodeMissingArgument,
			Message:  "missing text argument to '" + owner.text + "'",
		})
		return newPlaceholderAST(normalizeRange(owner.span), "missing text argument to "+owner.text)
	}
	if t.is(tokBeginGroup) {
		p.exp.next()
		return p.parseTextGroup(t)
	}
	return p.parseTextNucleus(t)
}

func (p *parser) parseTextGroup(open token) astNode {
	p.exp.beginScope()
	defer p.exp.endScope()
	body := p.parseTextSequence()
	close := p.exp.peek()
	span := open.span
	if close.is(tokEndGroup) {
		p.exp.next()
		span = source.Merge(open.span, close.span)
	} else {
		p.report(source.Diagnostic{
			Range:    normalizeRange(open.span),
			Severity: source.SeverityError,
			Code:     source.CodeUnclosedGroup,
			Message:  "unclosed text group; expected '}'",
		})
		span = source.Merge(span, body.base().span)
	}
	body.base().span = span
	body.base().origin = containerOrigin(open, body, span)
	return body
}

func (p *parser) parseTextSequence() astNode {
	p.depth++
	defer func() { p.depth-- }()
	if p.depth > p.limits.MaxNesting {
		p.limit(source.Detached(), "TeX text nesting limit exceeded")
		return &astRowNode{astBase: newASTBase(astRow, source.Detached())}
	}

	var items []astNode
	iterations := 0
	for {
		iterations++
		if iterations > p.limits.MaxTokens*2 {
			p.limit(source.Detached(), "TeX text parser iteration limit exceeded")
			return textRow(items)
		}

		t := p.exp.peek()
		switch {
		case t.isEOF(), t.is(tokEndGroup):
			return textRow(items)
		case t.is(tokSpace):
			// KaTeX text mode contracts adjacent input spaces into one spacing
			// node while retaining a single leading or trailing space.
			span := t.span
			for {
				next := p.exp.peek()
				if !next.is(tokSpace) {
					break
				}
				p.exp.next()
				span = source.Merge(span, next.span)
			}
			items = append(items, astLiteralText(token{span: span, origin: span}, " "))
		case t.is(tokBeginGroup):
			p.exp.next()
			items = append(items, p.parseTextGroup(t))
		case t.is(tokChar):
			p.exp.next()
			text := t.text
			if text == "$" {
				items = append(items, p.parseDollarMath(t))
				continue
			}
			if text == "~" {
				text = " "
			}
			items = append(items, astLiteralText(t, text))
		case t.is(tokSuperscript), t.is(tokSubscript), t.is(tokAlignment), t.is(tokParameter):
			p.exp.next()
			items = append(items, astLiteralText(t, t.text))
		case t.is(tokError):
			p.exp.next()
			items = append(items, astErr(t.span, t.text, source.CodeSyntax))
		case t.isControl():
			// parseTextCommandToken expects the command to have already been
			// consumed. Peeking without consuming here made every command in
			// text mode loop forever.
			p.exp.next()
			node := p.parseTextCommandToken(t)
			if node != nil {
				items = append(items, node)
			}
		default:
			p.exp.next()
			items = append(items, astErr(t.span, t.text, source.CodeUnexpectedToken))
		}
	}
}

func (p *parser) parseTextNucleus(t token) astNode {
	switch t.kind {
	case tokBeginGroup:
		p.exp.next()
		return p.parseTextGroup(t)
	case tokChar:
		p.exp.next()
		return astLiteralText(t, t.text)
	case tokSpace:
		p.exp.next()
		return astLiteralText(t, " ")
	case tokControlWord, tokControlSymbol:
		p.exp.next()
		node := p.parseTextCommandToken(t)
		if node == nil {
			return &astRowNode{astBase: newASTBase(astRow, source.Detached())}
		}
		return node
	default:
		p.exp.next()
		return astErr(t.span, t.text, source.CodeUnexpectedToken)
	}
}

func (p *parser) parseTextCommandToken(t token) astNode {
	if text, ok := textSymbolCommands[t.text]; ok {
		return astLiteralText(t, text)
	}
	if mark, ok := textAccentCommands[t.text]; ok {
		body := p.parseTextArgument(t)
		position := ir.PositionAbove
		switch t.text {
		case "\\c", "\\k", "\\b", "\\d":
			position = ir.PositionBelow
		}
		return &astAccentNode{
			astBase:  newASTBaseAt(astAccent, source.Merge(t.span, body.base().span), tokenOrigin(t)),
			body:     body,
			mark:     newGlyphAST(t.span, mark, ir.ClassDiacritic),
			position: position,
		}
	}

	switch t.text {
	case "\\(":
		return p.parseInlineMathText(t)
	case "\\)", "\\]":
		p.report(source.Diagnostic{
			Range:    normalizeRange(t.span),
			Severity: source.SeverityError,
			Code:     source.CodeUnexpectedToken,
			Message:  "mismatched math delimiter " + t.text,
		})
		return astErr(t.span, t.text, source.CodeUnexpectedToken)
	case "\\tiny", "\\sixptsize", "\\scriptsize", "\\footnotesize", "\\small",
		"\\normalsize", "\\large", "\\Large", "\\LARGE", "\\huge", "\\Huge":
		return p.parseUnsupportedTextSizing(t)
	case "\\ ", "\\space", "\\nobreakspace":
		return astLiteralText(t, " ")
	case "\\relax":
		return nil
	case "\\%", "\\$", "\\#", "\\&", "\\_", "\\{", "\\}":
		return astLiteralText(t, strings.TrimPrefix(t.text, "\\"))
	case "\\textbackslash":
		return astLiteralText(t, "\\")
	case "\\textasciitilde":
		return astLiteralText(t, "~")
	case "\\textasciicircum":
		return astLiteralText(t, "^")
	case "\\text", "\\textrm", "\\textnormal", "\\textsf", "\\texttt",
		"\\textbf", "\\textmd", "\\textit", "\\textup", "\\textsl", "\\emph",
		"\\mbox", "\\hbox":
		return p.parseTextCommand(t)
	case "\\textcolor":
		return p.parseTextColorCommand(t, true)
	case "\\sout":
		body := p.parseTextArgument(t)
		return &astCancelNode{
			astBase: newASTBaseAt(astCancel, source.Merge(t.span, body.base().span), tokenOrigin(t)),
			body:    body,
		}
	case "\\def", "\\gdef", "\\edef", "\\xdef":
		p.parseDefinition(t, definitionKindFor(t.text), t.text == "\\gdef" || t.text == "\\xdef")
		return nil
	case "\\newcommand", "\\renewcommand", "\\providecommand":
		p.parseNewCommand(t)
		return nil
	case "\\let":
		p.parseLet(t)
		return nil
	case "\\global":
		p.parseGlobal(t)
		return nil
	}
	if spec, ok := spacingCommands[t.text]; ok {
		return &astSpaceNode{
			astBase: newASTBaseAt(astSpace, t.span, tokenOrigin(t)),
			kind:    spec.kind,
			amount:  spec.amount,
		}
	}
	if spec, ok := accentCommands[t.text]; ok {
		body := p.parseTextArgument(t)
		return &astAccentNode{
			astBase:  newASTBaseAt(astAccent, source.Merge(t.span, body.base().span), tokenOrigin(t)),
			body:     body,
			mark:     newGlyphAST(t.span, spec.text, ir.ClassDiacritic),
			position: ir.PositionAbove,
			stretchy: false,
		}
	}
	p.report(source.Diagnostic{
		Range:    normalizeRange(t.span),
		Severity: source.SeverityError,
		Code:     source.CodeUnsupported,
		Message:  "command '" + t.text + "' is not supported in text mode",
	})
	return astErr(t.span, t.text, source.CodeUnsupported)
}

func (p *parser) parseInlineMathText(open token) astNode {
	// KaTeX math.ts switches text groups back to math mode for \(...\),
	// resets the font, and then returns to the outer text mode.
	p.metadataDepth++
	defer func() { p.metadataDepth-- }()
	body := p.parseSequence(stopParen)
	span := source.Merge(open.span, body.base().span)
	close := p.exp.peek()
	if close.isControl() && close.text == "\\)" {
		p.exp.next()
		span = source.Merge(span, close.span)
	} else {
		p.report(source.Diagnostic{
			Range:    normalizeRange(open.span),
			Severity: source.SeverityError,
			Code:     source.CodeUnclosedGroup,
			Message:  "unclosed inline math group; expected '\\)'",
		})
	}
	body.base().span = span
	body.base().origin = containerOrigin(open, body, span)
	return &astStyleNode{
		astBase: newASTBaseAt(astStyle, span, tokenOrigin(open)),
		body:    body,
		size:    ir.SizeText,
	}
}

func (p *parser) parseUnsupportedTextSizing(command token) astNode {
	body := p.parseTextSequence()
	span := source.Merge(command.span, body.base().span)
	body.base().span = span
	body.base().origin = containerOrigin(command, body, span)
	p.report(source.Diagnostic{
		Range:    normalizeRange(command.span),
		Severity: source.SeverityError,
		Code:     source.CodeUnsupported,
		Message:  "math font-size declaration '" + command.text + "' requires a neutral font-size field",
	})
	return body
}

var textSymbolCommands = map[string]string{
	"\\AA":                "Å",
	"\\aa":                "å",
	"\\AE":                "Æ",
	"\\ae":                "æ",
	"\\O":                 "Ø",
	"\\o":                 "ø",
	"\\OE":                "Œ",
	"\\oe":                "œ",
	"\\i":                 "ı",
	"\\j":                 "ȷ",
	"\\P":                 "¶",
	"\\S":                 "§",
	"\\sect":              "§",
	"\\ss":                "ß",
	"\\textcopyright":     "©",
	"\\textregistered":    "®",
	"\\texttrademark":     "™",
	"\\textdollar":        "$",
	"\\textsterling":      "£",
	"\\texteuro":          "€",
	"\\textyen":           "¥",
	"\\textgreater":       ">",
	"\\textless":          "<",
	"\\textquotedblleft":  "“",
	"\\textquotedblright": "”",
	"\\textquoteleft":     "‘",
	"\\textquoteright":    "’",
	"\\textunderscore":    "_",
	"\\textbar":           "|",
	"\\textbraceleft":     "{",
	"\\textbraceright":    "}",
	"\\textendash":        "–",
	"\\textemdash":        "—",
	"\\textellipsis":      "…",
	"\\textdegree":        "°",
	"\\textmu":            "µ",
	"\\texttimes":         "×",
	"\\textdiv":           "÷",
	"\\textpm":            "±",
	"\\textdagger":        "†",
	"\\textdaggerdbl":     "‡",
}

var textAccentCommands = map[string]string{
	"\\'":           "´",
	"\\`":           "ˋ",
	"\\\"":          "¨",
	"\\.":           "˙",
	"\\=":           "¯",
	"\\~":           "˜",
	"\\^":           "ˆ",
	"\\H":           "˝",
	"\\r":           "˚",
	"\\u":           "˘",
	"\\v":           "ˇ",
	"\\c":           "¸",
	"\\k":           "˛",
	"\\b":           "̱",
	"\\d":           "̣",
	"\\textcircled": "◯",
}

func textRow(items []astNode) astNode {
	row := &astRowNode{
		astBase: newASTBase(astRow, source.Detached()),
		items:   items,
	}
	if len(items) > 0 {
		row.span = astSpan(items...)
		row.origin = astOrigin(items...)
	}
	return row
}

func astLiteralText(t token, text string) *astTextNode {
	return &astTextNode{
		astBase: newASTBaseAt(astText, normalizeRange(t.span), tokenOrigin(t)),
		text:    text,
		variant: ir.VariantAuto,
	}
}

func plainText(n astNode) (string, bool) {
	switch n := n.(type) {
	case *astRowNode:
		var b strings.Builder
		for _, item := range n.items {
			text, ok := plainText(item)
			if !ok {
				return "", false
			}
			b.WriteString(text)
		}
		return b.String(), true
	case *astTextNode:
		return n.text, true
	case *astGlyphNode:
		return n.text, true
	case *astSpaceNode:
		return " ", true
	case *astVariantNode:
		return plainText(n.body)
	case *astErrorNode:
		return n.text, true
	default:
		return "", false
	}
}
