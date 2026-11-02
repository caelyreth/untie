package tex

import (
	"strings"

	"github.com/caelyreth/untie/internal/mathdata"
	"github.com/caelyreth/untie/ir"
	"github.com/caelyreth/untie/source"
)

func (p *parser) parseCommand(t token, stop stopMode) astNode {
	if spec, ok := symbolCommands[t.text]; ok {
		if spec.class == ir.ClassLarge {
			return &astOperatorNode{
				astBase: newASTBaseAt(astOperator, t.span, tokenOrigin(t)),
				text:    spec.text,
				variant: spec.variant,
				limits:  spec.limits,
			}
		}
		return astGlyphSpec(t.span, t.origin, spec)
	}
	if spec, ok := spacingCommands[t.text]; ok {
		return &astSpaceNode{
			astBase: newASTBaseAt(astSpace, t.span, tokenOrigin(t)),
			kind:    spec.kind,
			amount:  spec.amount,
		}
	}
	if spec, ok := accentCommands[t.text]; ok {
		body := p.parseRequiredArgument(t)
		mark := newGlyphAST(t.span, spec.text, ir.ClassDiacritic)
		return &astAccentNode{
			astBase:  newASTBaseAt(astAccent, source.Merge(t.span, body.base().span), tokenOrigin(t)),
			body:     body,
			mark:     mark,
			position: ir.PositionAbove,
			stretchy: spec.stretchy,
		}
	}
	if op, ok := namedOperators[t.text]; ok {
		limits := ir.LimitsNever
		if limitOperators[t.text] {
			limits = ir.LimitsAuto
		}
		return &astOperatorNode{
			astBase: newASTBaseAt(astOperator, t.span, tokenOrigin(t)),
			text:    op,
			limits:  limits,
		}
	}
	if variant, ok := fontCommands[t.text]; ok {
		body := p.parseRequiredArgument(t)
		return &astVariantNode{
			astBase: newASTBaseAt(astVariant, source.Merge(t.span, body.base().span), tokenOrigin(t)),
			body:    body,
			variant: variant,
		}
	}
	if spec, ok := delimiterCommands[t.text]; ok {
		spec.class = fixedDelimiterClass(spec.class)
		return astGlyphSpec(t.span, t.origin, symbolSpec{text: spec.text, class: spec.class, variant: ir.VariantAuto})
	}

	switch t.text {
	case "\\frac", "\\dfrac", "\\tfrac", "\\cfrac":
		return p.parseFraction(t)
	case "\\genfrac":
		return p.parseGenfrac(t)
	case "\\binom", "\\dbinom", "\\tbinom":
		return p.parseBinomial(t)
	case "\\sqrt":
		return p.parseRadical(t)
	case "\\left":
		return p.parseLeftRight(t)
	case "\\right", "\\middle":
		if p.leftDepth > 0 {
			class := ir.ClassFence
			if t.text == "\\right" {
				class = ir.ClassClosing
			}
			if delimiter := p.parseDelimiter(t, class); delimiter != nil {
				return delimiter
			}
			return nil
		}
		p.report(source.Diagnostic{
			Range:    normalizeRange(t.span),
			Severity: source.SeverityError,
			Code:     source.CodeUnexpectedToken,
			Message:  t.text + " without matching \\left",
		})
		_ = p.parseDelimiter(t, ir.ClassFence)
		return astErr(t.span, t.text, source.CodeUnexpectedToken)
	case "\\overline":
		return p.parseLine(t, ir.PositionAbove)
	case "\\underline", "\\underbar":
		return p.parseLine(t, ir.PositionBelow)
	case "\\overbrace":
		return p.parseOverUnderAccent(t, ir.PositionAbove, "⏞", true)
	case "\\underbrace":
		return p.parseOverUnderAccent(t, ir.PositionBelow, "⏟", true)
	case "\\overbracket":
		return p.parseOverUnderAccent(t, ir.PositionAbove, "⎴", true)
	case "\\underbracket":
		return p.parseOverUnderAccent(t, ir.PositionBelow, "⎵", true)
	case "\\overgroup":
		return p.parseOverUnderAccent(t, ir.PositionAbove, "⏠", true)
	case "\\undergroup":
		return p.parseOverUnderAccent(t, ir.PositionBelow, "⏡", true)
	case "\\underlinesegment":
		return p.parseOverUnderAccent(t, ir.PositionBelow, "‾", true)
	case "\\underrightarrow":
		return p.parseOverUnderAccent(t, ir.PositionBelow, "→", true)
	case "\\underleftarrow":
		return p.parseOverUnderAccent(t, ir.PositionBelow, "←", true)
	case "\\underleftrightarrow":
		return p.parseOverUnderAccent(t, ir.PositionBelow, "↔", true)
	case "\\Overrightarrow":
		return p.parseOverUnderAccent(t, ir.PositionAbove, "→", true)
	case "\\utilde":
		return p.parseOverUnderAccent(t, ir.PositionBelow, "˜", false)
	case "\\overset", "\\stackrel":
		return p.parseStacked(t, ir.PositionAbove)
	case "\\underset":
		return p.parseStacked(t, ir.PositionBelow)
	case "\\xleftarrow", "\\xrightarrow", "\\xLeftarrow", "\\xRightarrow",
		"\\xleftrightarrow", "\\xLeftrightarrow", "\\xhookleftarrow",
		"\\xhookrightarrow", "\\xmapsto", "\\xlongequal", "\\xleftharpoondown",
		"\\xleftharpoonup", "\\xrightharpoondown", "\\xrightharpoonup",
		"\\xleftrightharpoons", "\\xrightleftharpoons", "\\xtofrom",
		"\\xtwoheadleftarrow", "\\xtwoheadrightarrow":
		return p.parseExtensibleArrow(t)
	case "\\cancel", "\\bcancel", "\\xcancel":
		return p.parseCancel(t)
	case "\\sout":
		return p.parseCancel(t)
	case "\\text", "\\textrm", "\\textnormal", "\\textsf", "\\texttt",
		"\\textbf", "\\textmd", "\\textit", "\\textup", "\\textsl", "\\emph",
		"\\mbox", "\\hbox":
		return p.parseTextCommand(t)
	case "\\textcolor":
		return p.parseTextColorCommand(t, false)
	case "\\color":
		return p.parseColorCommand(t, stop)
	case "\\colorbox":
		return p.parseColorBox(t)
	case "\\fcolorbox":
		return p.parseColorBox(t)
	case "\\boxed", "\\fbox":
		return p.parseFramedBox(t)
	case "\\phantom", "\\hphantom", "\\vphantom":
		return p.parsePhantom(t)
	case "\\smash":
		return p.parsePhantom(t)
	case "\\operatorname", "\\operatornamewithlimits":
		return p.parseOperatorName(t)
	case "\\mathchoice":
		return p.parseMathChoice(t)
	case "\\mathbin", "\\mathrel", "\\mathopen", "\\mathclose", "\\mathpunct",
		"\\mathinner", "\\mathord", "\\mathop":
		return p.parseMathClass(t)
	case "\\not":
		body := p.parseRequiredArgument(t)
		return &astCancelNode{
			astBase: newASTBaseAt(astCancel, source.Merge(t.span, body.base().span), tokenOrigin(t)),
			body:    body,
		}
	case "\\dblcolon", "\\coloncolon":
		return p.parseCompositeRelation(t, "∷")
	case "\\coloneqq", "\\colonequals":
		return p.parseCompositeRelation(t, "≔")
	case "\\Coloneqq", "\\coloncolonequals":
		return p.parseCompositeRelation(t, "⩴")
	case "\\eqqcolon", "\\equalscolon":
		return p.parseCompositeRelation(t, "≕")
	case "\\eqcolon":
		return p.parseCompositeRelation(t, "∹")
	case "\\Eqqcolon":
		return p.parseCompositeRelation(t, "=", "∷")
	case "\\Eqcolon", "\\equalscoloncolon":
		return p.parseCompositeRelation(t, "−", "∷")
	case "\\coloneq", "\\colonminus":
		return p.parseCompositeRelation(t, ":", "−")
	case "\\Coloneq", "\\coloncolonminus":
		return p.parseCompositeRelation(t, "∷", "−")
	case "\\minuscolon":
		return p.parseCompositeRelation(t, "−", ":")
	case "\\minuscoloncolon":
		return p.parseCompositeRelation(t, "−", "∷")
	case "\\colonapprox":
		return p.parseCompositeRelation(t, ":", "≈")
	case "\\Colonapprox", "\\coloncolonapprox":
		return p.parseCompositeRelation(t, "∷", "≈")
	case "\\colonsim":
		return p.parseCompositeRelation(t, ":", "∼")
	case "\\Colonsim", "\\coloncolonsim":
		return p.parseCompositeRelation(t, "∷", "∼")
	case "\\simcolon":
		return p.parseCompositeRelation(t, "∼", ":")
	case "\\simcoloncolon":
		return p.parseCompositeRelation(t, "∼", "∷")
	case "\\approxcolon":
		return p.parseCompositeRelation(t, "≈", ":")
	case "\\approxcoloncolon":
		return p.parseCompositeRelation(t, "≈", "∷")
	case "\\vcentcolon":
		return p.parseCompositeRelation(t, ":")
	case "\\bra", "\\ket", "\\braket", "\\Bra", "\\Ket", "\\Braket",
		"\\set", "\\Set", "\\angl":
		return p.parseDelimitedArgument(t)
	case "\\substack":
		return p.parseSubstack(t)
	case "\\sideset":
		return p.parseSideSet(t, stop)
	case "\\href":
		return p.parseHref(t)
	case "\\url":
		return p.parseURL(t)
	case "\\htmlClass", "\\htmlData", "\\htmlId":
		return p.parseHTMLMetadataWrapper(t)
	case "\\htmlStyle":
		return p.parseHTMLStyle(t)
	case "\\KaTeX", "\\LaTeX", "\\TeX":
		return astLiteralText(t, strings.TrimPrefix(t.text, "\\"))
	case "\\verb":
		return p.parseVerb(t)
	case "\\char":
		return p.parseChar(t)
	case "\\newline":
		return &astBreakNode{astBase: newASTBaseAt(astBreak, t.span, tokenOrigin(t)), kind: ir.BreakExplicit}
	case "\\ce", "\\pu":
		_ = p.parseRequiredArgument(t)
		p.report(source.Diagnostic{
			Range:    normalizeRange(t.span),
			Severity: source.SeverityError,
			Code:     source.CodeUnsupported,
			Message:  "chemistry markup is outside the neutral math IR",
		})
		return newPlaceholderAST(t.span, "unsupported chemistry markup")
	case "\\includegraphics":
		_, _, _ = p.consumeRawOptionalGroup()
		_, _, _ = p.consumeRawGroupTextChecked(t)
		p.report(source.Diagnostic{
			Range:    normalizeRange(t.span),
			Severity: source.SeverityError,
			Code:     source.CodeUnsupported,
			Message:  "images are outside the neutral math IR",
		})
		return newPlaceholderAST(t.span, "unsupported image")
	case "\\raisebox":
		_, _, _ = p.consumeRawGroupTextChecked(t)
		_ = p.parseRequiredArgument(t)
		p.report(source.Diagnostic{
			Range:    normalizeRange(t.span),
			Severity: source.SeverityError,
			Code:     source.CodeUnsupported,
			Message:  "vertical raises are not represented in the neutral math IR",
		})
		return newPlaceholderAST(t.span, "unsupported raise")
	case "\\vcenter":
		_ = p.parseRequiredArgument(t)
		p.report(source.Diagnostic{
			Range:    normalizeRange(t.span),
			Severity: source.SeverityError,
			Code:     source.CodeUnsupported,
			Message:  "vertical centering is not represented in the neutral math IR",
		})
		return newPlaceholderAST(t.span, "unsupported vcenter")
	case "\\mathclap", "\\mathllap", "\\mathrlap", "\\llap", "\\rlap", "\\clap":
		_ = p.parseRequiredArgument(t)
		p.report(source.Diagnostic{
			Range:    normalizeRange(t.span),
			Severity: source.SeverityError,
			Code:     source.CodeUnsupported,
			Message:  "overlapping boxes are not represented in the neutral math IR",
		})
		return newPlaceholderAST(t.span, "unsupported overlap")
	case "\\phase":
		_ = p.parseRequiredArgument(t)
		p.report(source.Diagnostic{
			Range:    normalizeRange(t.span),
			Severity: source.SeverityError,
			Code:     source.CodeUnsupported,
			Message:  "phase enclosure is not represented in the neutral math IR",
		})
		return newPlaceholderAST(t.span, "unsupported phase enclosure")
	case "\\rule":
		return p.parseRule(t)
	case "\\limits", "\\nolimits":
		p.report(source.Diagnostic{
			Range:    normalizeRange(t.span),
			Severity: source.SeverityError,
			Code:     source.CodeUnexpectedToken,
			Message:  t.text + " must follow a math operator",
		})
		return astErr(t.span, t.text, source.CodeUnexpectedToken)
	case "\\displaystyle":
		return p.parseDeclarativeStyle(t, stop, ir.SizeDisplay)
	case "\\textstyle":
		return p.parseDeclarativeStyle(t, stop, ir.SizeText)
	case "\\scriptstyle":
		return p.parseDeclarativeStyle(t, stop, ir.SizeScript)
	case "\\scriptscriptstyle":
		return p.parseDeclarativeStyle(t, stop, ir.SizeScriptScript)
	case "\\rm":
		return p.parseDeclarativeVariant(t, stop, ir.VariantNormal)
	case "\\it":
		return p.parseDeclarativeVariant(t, stop, ir.VariantItalic)
	case "\\bf":
		return p.parseDeclarativeVariant(t, stop, ir.VariantBold)
	case "\\sf":
		return p.parseDeclarativeVariant(t, stop, ir.VariantSans)
	case "\\tt":
		return p.parseDeclarativeVariant(t, stop, ir.VariantMonospace)
	case "\\cal":
		return p.parseDeclarativeVariant(t, stop, ir.VariantScript)
	case "\\kern", "\\mkern":
		amount := p.parseDimension(t, false)
		return &astSpaceNode{astBase: newASTBaseAt(astSpace, t.span, tokenOrigin(t)), kind: ir.SpaceExplicit, amount: amount}
	case "\\tmspace":
		return p.parseTMspace(t)
	case "\\hspace", "\\mspace", "\\mskip":
		amount := p.parseDimension(t, true)
		return &astSpaceNode{astBase: newASTBaseAt(astSpace, t.span, tokenOrigin(t)), kind: ir.SpaceExplicit, amount: amount}
	case "\\hskip":
		amount := p.parseDimension(t, false)
		return &astSpaceNode{astBase: newASTBaseAt(astSpace, t.span, tokenOrigin(t)), kind: ir.SpaceExplicit, amount: amount}
	case "\\allowbreak":
		return &astBreakNode{astBase: newASTBaseAt(astBreak, t.span, tokenOrigin(t)), kind: ir.BreakExplicit}
	case "\\nonumber", "\\notag":
		p.reportEquationMetadata(t, "equation numbering is outside the formula IR")
		return nil
	case "\\tag", "\\tag*":
		p.exp.consumeSpaces()
		if star := p.exp.peek(); star.is(tokChar) && star.text == "*" {
			p.exp.next()
		}
		p.parseRequiredArgument(t)
		p.reportEquationMetadata(t, "equation tags are outside the formula IR")
		return nil
	case "\\nobreak":
		p.report(source.Diagnostic{
			Range:    normalizeRange(t.span),
			Severity: source.SeverityError,
			Code:     source.CodeUnsupported,
			Message:  "line-break suppression is not represented in the neutral IR",
		})
		return nil
	case "\\relax":
		return nil
	case "\\def", "\\gdef", "\\edef", "\\xdef":
		p.parseDefinition(t, definitionKindFor(t.text), t.text == "\\gdef" || t.text == "\\xdef")
		return nil
	case "\\newcommand", "\\renewcommand", "\\providecommand":
		p.parseNewCommand(t)
		return nil
	case "\\bmod":
		return p.parseBmod(t)
	case "\\mod", "\\pmod", "\\pod":
		return p.parseModArgument(t)
	case "\\let":
		p.parseLet(t)
		return nil
	case "\\global":
		p.parseGlobal(t)
		return nil
	case "\\begin":
		return p.parseEnvironment(t)
	case "\\begingroup":
		return p.parseBeginGroup(t)
	case "\\endgroup":
		p.report(source.Diagnostic{
			Range:    normalizeRange(t.span),
			Severity: source.SeverityError,
			Code:     source.CodeUnexpectedToken,
			Message:  "\\endgroup without matching \\begingroup",
		})
		return astErr(t.span, t.text, source.CodeUnexpectedToken)
	case "\\end":
		p.report(source.Diagnostic{
			Range:    normalizeRange(t.span),
			Severity: source.SeverityError,
			Code:     source.CodeInvalidEnvironment,
			Message:  "\\end without matching \\begin",
		})
		_ = p.consumeRawGroupText()
		return astErr(t.span, t.text, source.CodeInvalidEnvironment)
	case "\\big", "\\Big", "\\bigg", "\\Bigg",
		"\\bigl", "\\Bigl", "\\biggl", "\\Biggl",
		"\\bigr", "\\Bigr", "\\biggr", "\\Biggr",
		"\\bigm", "\\Bigm", "\\biggm", "\\Biggm":
		return p.parseSizedDelimiter(t)
	case "\\\\":
		p.report(source.Diagnostic{
			Range:    normalizeRange(t.span),
			Severity: source.SeverityError,
			Code:     source.CodeUnexpectedToken,
			Message:  "line break is only valid in a math environment",
		})
		return astErr(t.span, t.text, source.CodeUnexpectedToken)
	}

	if node := p.parseSharedTeXCommand(t); node != nil {
		return node
	}

	if strings.HasPrefix(t.text, "\\") {
		p.report(source.Diagnostic{
			Range:    normalizeRange(t.span),
			Severity: source.SeverityError,
			Code:     source.CodeUnknownCommand,
			Message:  "undefined control sequence: " + t.text,
		})
	} else {
		p.report(source.Diagnostic{
			Range:    normalizeRange(t.span),
			Severity: source.SeverityError,
			Code:     source.CodeSyntax,
			Message:  "unexpected token '" + t.text + "'",
		})
	}
	return astErr(t.span, t.text, source.CodeUnknownCommand)
}

func (p *parser) parseBeginGroup(command token) astNode {
	p.exp.beginScope()
	defer p.exp.endScope()
	body := p.parseSequence(stopEndgroup)
	span := source.Merge(command.span, body.base().span)
	if end := p.exp.peek(); end.isControl() && end.text == "\\endgroup" {
		p.exp.next()
		span = source.Merge(span, end.span)
	} else {
		p.report(source.Diagnostic{
			Range:    normalizeRange(command.span),
			Severity: source.SeverityError,
			Code:     source.CodeUnclosedGroup,
			Message:  "unclosed \\begingroup",
		})
	}
	body.base().span = span
	body.base().origin = containerOrigin(command, body, span)
	return body
}

func (p *parser) reportEquationMetadata(command token, message string) {
	p.report(source.Diagnostic{
		Range:    normalizeRange(command.span),
		Severity: source.SeverityWarning,
		Code:     source.CodeUnsupported,
		Message:  message,
	})
}

// parseSharedTeXCommand resolves command metadata that is generated once and
// shared with the Typst frontend. Local tables retain precedence for TeX
// parser behavior that is richer than the portable symbol metadata.
func (p *parser) parseSharedTeXCommand(t token) astNode {
	if operator, ok := mathdata.LookupTeXOperator(t.text); ok {
		if operator.Text != "" {
			limits := ir.LimitsNever
			if operator.Limits {
				limits = ir.LimitsAuto
			}
			return &astOperatorNode{
				astBase: newASTBaseAt(astOperator, t.span, tokenOrigin(t)),
				text:    operator.Text,
				variant: ir.VariantNormal,
				limits:  limits,
			}
		}
		if symbol, ok := mathdata.LookupTeXSymbol(t.text); ok &&
			symbol.Kind == mathdata.TeXSymbolPublic && symbol.Text != "" {
			limits := ir.LimitsNever
			if operator.Limits {
				limits = ir.LimitsAuto
			}
			return &astOperatorNode{
				astBase: newASTBaseAt(astOperator, t.span, tokenOrigin(t)),
				text:    symbol.Text,
				variant: ir.VariantNormal,
				limits:  limits,
			}
		}
	}
	symbol, ok := mathdata.LookupTeXSymbol(t.text)
	if !ok || symbol.Kind != mathdata.TeXSymbolPublic || symbol.Text == "" {
		return nil
	}
	class := irClassForMathdataClass(symbol.Class)
	if class == ir.ClassLarge {
		return &astOperatorNode{
			astBase: newASTBaseAt(astOperator, t.span, tokenOrigin(t)),
			text:    symbol.Text,
			variant: ir.VariantNormal,
			limits:  ir.LimitsAuto,
		}
	}
	return astGlyphSpec(t.span, t.origin, symbolSpec{
		text:    symbol.Text,
		class:   class,
		variant: ir.VariantAuto,
	})
}

func irClassForMathdataClass(class mathdata.Class) ir.MathClass {
	switch class {
	case mathdata.ClassAlphabetic:
		return ir.ClassAlphabetic
	case mathdata.ClassBinary:
		return ir.ClassBinary
	case mathdata.ClassClosing:
		return ir.ClassClosing
	case mathdata.ClassDiacritic:
		return ir.ClassDiacritic
	case mathdata.ClassFence:
		return ir.ClassFence
	case mathdata.ClassLarge:
		return ir.ClassLarge
	case mathdata.ClassOpening:
		return ir.ClassOpening
	case mathdata.ClassPunctuation:
		return ir.ClassPunctuation
	case mathdata.ClassRelation:
		return ir.ClassRelation
	case mathdata.ClassSpace:
		return ir.ClassSpace
	case mathdata.ClassUnary:
		return ir.ClassUnary
	case mathdata.ClassVary:
		return ir.ClassVarying
	case mathdata.ClassSpecial:
		return ir.ClassSpecial
	default:
		return ir.ClassNormal
	}
}

func (p *parser) parseFraction(command token) astNode {
	numerator := p.parseRequiredArgument(command)
	denominator := p.parseRequiredArgument(command)
	size := ir.SizeInherit
	switch command.text {
	case "\\dfrac", "\\cfrac":
		size = ir.SizeDisplay
	case "\\tfrac":
		size = ir.SizeText
	}
	return &astFractionNode{
		astBase:     newASTBaseAt(astFraction, source.Merge(command.span, denominator.base().span), tokenOrigin(command)),
		numerator:   numerator,
		denominator: denominator,
		bar:         true,
		style:       ir.FractionVertical,
		size:        size,
	}
}

func (p *parser) parseBinomial(command token) astNode {
	fraction := p.parseFraction(command)
	f := fraction.(*astFractionNode)
	f.bar = false
	switch command.text {
	case "\\dbinom":
		f.size = ir.SizeDisplay
	case "\\tbinom":
		f.size = ir.SizeText
	}
	return &astFencedNode{
		astBase:  newASTBaseAt(astFenced, f.span, tokenOrigin(command)),
		open:     newGlyphAST(command.span, "(", ir.ClassOpening),
		close:    newGlyphAST(command.span, ")", ir.ClassClosing),
		body:     []astNode{f},
		balanced: false,
	}
}

func (p *parser) parseRadical(command token) astNode {
	index := p.parseOptionalArgument(command)
	radicand := p.parseRequiredArgument(command)
	span := source.Merge(command.span, radicand.base().span)
	if index != nil {
		span = source.Merge(span, index.base().span)
	}
	return &astRadicalNode{
		astBase:  newASTBaseAt(astRadical, span, tokenOrigin(command)),
		radicand: radicand,
		index:    index,
	}
}

func (p *parser) parseLeftRight(command token) astNode {
	p.leftDepth++
	defer func() { p.leftDepth-- }()
	open := p.parseDelimiter(command, ir.ClassOpening)
	startSpan := command.span
	if open != nil {
		startSpan = source.Merge(startSpan, open.base().span)
	}

	var body []astNode
	balanced := false
	endSpan := startSpan
	for {
		part := p.parseSequence(stopRight)
		if row, ok := part.(*astRowNode); ok {
			body = append(body, row.items...)
		} else if part != nil {
			body = append(body, part)
		}

		next := p.exp.peek()
		switch {
		case next.isControl() && next.text == "\\middle":
			middle := p.exp.next()
			delim := p.parseDelimiter(middle, ir.ClassFence)
			if delim != nil {
				body = append(body, delim)
				endSpan = delim.base().span
			} else {
				endSpan = middle.span
			}
			continue
		case next.isControl() && next.text == "\\right":
			right := p.exp.next()
			close := p.parseDelimiter(right, ir.ClassClosing)
			balanced = true
			endSpan = right.span
			if close != nil {
				endSpan = close.base().span
			}
			return &astFencedNode{
				astBase:  newASTBaseAt(astFenced, source.Merge(command.span, endSpan), tokenOrigin(command)),
				open:     open,
				close:    close,
				body:     body,
				balanced: balanced,
				size:     inheritedTableSize(body),
			}
		default:
			p.report(source.Diagnostic{
				Range:    normalizeRange(next.span),
				Severity: source.SeverityError,
				Code:     source.CodeUnclosedGroup,
				Message:  "missing \\right for \\left",
			})
			return &astFencedNode{
				astBase:  newASTBaseAt(astFenced, source.Merge(command.span, endSpan), tokenOrigin(command)),
				open:     open,
				body:     body,
				balanced: false,
			}
		}
	}
}

func inheritedTableSize(body []astNode) ir.MathSize {
	if len(body) != 1 {
		return ir.SizeInherit
	}
	if table, ok := body[0].(*astTableNode); ok {
		return table.size
	}
	return ir.SizeInherit
}

func (p *parser) parseDelimiter(owner token, fallbackClass ir.MathClass) astNode {
	p.exp.consumeSpaces()
	t := p.exp.next()
	if t.isEOF() || t.is(tokEndGroup) {
		p.report(source.Diagnostic{
			Range:    normalizeRange(owner.span),
			Severity: source.SeverityError,
			Code:     source.CodeMissingArgument,
			Message:  "missing delimiter after '" + owner.text + "'",
		})
		return nil
	}
	spec, ok := delimiterCommands[t.text]
	if !ok {
		p.report(source.Diagnostic{
			Range:    normalizeRange(t.span),
			Severity: source.SeverityError,
			Code:     source.CodeInvalidValue,
			Message:  "invalid delimiter '" + t.text + "'",
		})
		return newGlyphAST(t.span, t.text, fallbackClass)
	}
	if spec.text == "." {
		return nil
	}
	class := spec.class
	if fallbackClass != ir.ClassFence {
		class = fallbackClass
	}
	return astGlyphSpec(t.span, t.origin, symbolSpec{text: spec.text, class: class, variant: ir.VariantAuto})
}

func (p *parser) parseLine(command token, position ir.Position) astNode {
	body := p.parseRequiredArgument(command)
	return &astLineNode{
		astBase:  newASTBaseAt(astLine, source.Merge(command.span, body.base().span), tokenOrigin(command)),
		body:     body,
		position: position,
	}
}

func (p *parser) parseCancel(command token) astNode {
	body := p.parseRequiredArgument(command)
	return &astCancelNode{
		astBase:  newASTBaseAt(astCancel, source.Merge(command.span, body.base().span), tokenOrigin(command)),
		body:     body,
		cross:    command.text == "\\xcancel",
		inverted: command.text == "\\bcancel",
	}
}

func (p *parser) parseDeclarativeStyle(command token, stop stopMode, size ir.MathSize) astNode {
	body := p.parseSequence(stop)
	return &astStyleNode{
		astBase: newASTBaseAt(astStyle, source.Merge(command.span, body.base().span), tokenOrigin(command)),
		body:    body,
		size:    size,
	}
}

func (p *parser) parseDeclarativeVariant(command token, stop stopMode, variant ir.Variant) astNode {
	body := p.parseSequence(stop)
	return &astVariantNode{
		astBase: newASTBaseAt(astVariant, source.Merge(command.span, body.base().span), tokenOrigin(command)),
		body:    body,
		variant: variant,
	}
}

func (p *parser) parseSizedDelimiter(command token) astNode {
	p.exp.consumeSpaces()
	t := p.exp.next()
	spec, ok := delimiterCommands[t.text]
	if !ok || spec.text == "." {
		p.report(source.Diagnostic{
			Range:    normalizeRange(t.span),
			Severity: source.SeverityError,
			Code:     source.CodeInvalidValue,
			Message:  "invalid delimiter after '" + command.text + "'",
		})
		return astErr(t.span, t.text, source.CodeInvalidValue)
	}
	p.report(source.Diagnostic{
		Range:    normalizeRange(source.Merge(command.span, t.span)),
		Severity: source.SeverityError,
		Code:     source.CodeUnsupported,
		Message:  "explicit delimiter size command is not represented in the current neutral IR",
	})
	return astErr(source.Merge(command.span, t.span), command.text, source.CodeUnsupported)
}

func fixedDelimiterClass(class ir.MathClass) ir.MathClass {
	if class == ir.ClassNormal {
		return ir.ClassFence
	}
	return class
}
