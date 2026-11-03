package tex

import (
	"unicode"
	"unicode/utf8"

	"github.com/caelyreth/untie/ir"
	"github.com/caelyreth/untie/source"
)

// chemistryParser intentionally implements a bounded mhchem grammar. It
// produces the same neutral TeX AST used by ordinary math so chemistry gains
// source hits, recovery, canonical validation, and alternative text without a
// chemistry-specific IR node.
type chemistryParser struct {
	p         *parser
	tokens    []token
	pos       int
	nodes     int
	exhausted bool
}

type chemistryClose struct {
	kind    tokenKind
	text    string
	control bool
}

func (p *parser) parseChemistryCommand(command token) astNode {
	tokens, groupSpan, ok := p.consumeChemistryArgument(command)
	if !ok {
		return newPlaceholderAST(command.span, "missing chemistry argument")
	}
	c := chemistryParser{p: p, tokens: tokens}
	var body astNode
	if command.text == "\\pu" {
		body = c.parsePhysicalQuantity()
	} else {
		body = c.parseChemicalExpression()
	}
	span := source.Merge(command.span, groupSpan)
	origin := tokenOrigin(command)
	if body == nil {
		c.report(groupSpan, source.SeverityError, source.CodeInvalidValue, command.text+" argument is empty")
		return newPlaceholderAST(span, "empty chemistry markup")
	}
	if row, ok := body.(*astRowNode); ok {
		row.span = span
		row.origin = origin
		return row
	}
	row := &astRowNode{
		astBase: newASTBaseAt(astRow, span, origin),
		items:   []astNode{body},
	}
	return row
}

func (p *parser) consumeChemistryArgument(owner token) ([]token, source.Range, bool) {
	p.exp.consumeSpaces()
	open := p.exp.popRaw()
	if !open.is(tokBeginGroup) {
		p.exp.push(open)
		p.report(source.Diagnostic{
			Range:    normalizeRange(owner.span),
			Severity: source.SeverityError,
			Code:     source.CodeMissingArgument,
			Message:  owner.text + " requires a braced argument",
		})
		return nil, open.span, false
	}

	var body []token
	depth := 1
	span := open.span
	for {
		t := p.exp.popRaw()
		if t.isEOF() {
			p.report(source.Diagnostic{
				Range:    normalizeRange(open.span),
				Severity: source.SeverityError,
				Code:     source.CodeUnclosedGroup,
				Message:  "unclosed " + owner.text + " argument",
			})
			break
		}
		span = source.Merge(span, t.span)
		if t.is(tokBeginGroup) {
			depth++
			body = append(body, t)
			continue
		}
		if t.is(tokEndGroup) {
			depth--
			if depth == 0 {
				break
			}
			body = append(body, t)
			continue
		}
		body = append(body, t)
	}
	return body, span, true
}

func (c *chemistryParser) parseChemicalExpression() astNode {
	items, _, _ := c.parseItems(chemistryClose{}, 1, false)
	return c.row(items, source.Detached())
}

func (c *chemistryParser) parseItems(
	close chemistryClose,
	depth int,
	compact bool,
) ([]astNode, bool, source.Range) {
	if depth > c.p.limits.MaxNesting {
		span := c.remainingSpan()
		c.p.limit(span, "chemistry nesting limit exceeded")
		c.exhausted = true
		return []astNode{c.errorNode(span, "chemistry nesting limit exceeded", source.CodeLimitExceeded)}, true, span
	}

	var items []astNode
	pendingSpace := source.Detached()
	atTermStart := true
	for c.pos < len(c.tokens) && !c.exhausted {
		if c.closes(close) {
			closing := c.tokens[c.pos]
			c.pos++
			return items, true, closing.span
		}
		if c.tokens[c.pos].is(tokSpace) {
			pendingSpace = source.Merge(pendingSpace, c.tokens[c.pos].span)
			c.pos++
			continue
		}

		if !compact {
			if arrow, ok := c.matchArrow(); ok {
				items = append(items, arrow)
				atTermStart = true
				pendingSpace = source.Detached()
				continue
			}
			if c.current().isControl() && c.current().text == "\\bond" {
				items = append(items, c.parseBond())
				atTermStart = false
				pendingSpace = source.Detached()
				continue
			}
			if c.matchExplicitOperator() {
				items = append(items, c.parseExplicitOperator())
				atTermStart = true
				pendingSpace = source.Detached()
				continue
			}
			if atTermStart && c.coefficientAhead() {
				coefficient := c.parseCoefficient()
				items = append(items, coefficient)
				items = append(items, c.generatedSpace(3))
				c.consumeSpaces()
				atTermStart = false
				pendingSpace = source.Detached()
				continue
			}
			if c.current().is(tokChar) &&
				(c.current().text == "*" || c.current().text == ".") &&
				c.additionCompoundAhead() {
				t := c.current()
				c.pos++
				items = append(items, c.additionCompound(t))
				atTermStart = true
				pendingSpace = source.Detached()
				continue
			}
		}

		if !pendingSpace.IsDetached() && len(items) > 0 && !compact {
			items = append(items, c.explicitSpace(pendingSpace, 3))
		}
		pendingSpace = source.Detached()
		if compact && c.current().is(tokSpace) {
			c.consumeSpaces()
		}

		item := c.parseEntity(depth, compact)
		if item == nil {
			continue
		}
		items = append(items, item)
		atTermStart = false
	}
	if c.exhausted {
		return items, true, c.currentSpan()
	}
	if close.kind != tokEOF {
		c.report(c.currentSpan(), source.SeverityError, source.CodeUnclosedGroup, "unclosed chemistry group")
	}
	return items, false, c.currentSpan()
}

func (c *chemistryParser) parseEntity(depth int, compact bool) astNode {
	start := c.currentSpan()
	var topLeft, bottomLeft astNode
	for c.pos < len(c.tokens) {
		t := c.current()
		if !t.is(tokSuperscript) && !t.is(tokSubscript) {
			break
		}
		c.pos++
		script := c.parseScriptContent(t, depth, compact)
		if t.is(tokSuperscript) {
			topLeft = c.appendScript(topLeft, script)
		} else {
			bottomLeft = c.appendScript(bottomLeft, script)
		}
	}

	base := c.parseAtom(depth, compact)
	if base == nil {
		if topLeft == nil && bottomLeft == nil {
			return nil
		}
		c.report(start, source.SeverityError, source.CodeMissingArgument, "chemistry script has no base")
		base = c.errorNode(start, "missing chemistry base", source.CodeMissingArgument)
	}

	var top, bottom astNode
	foldScripts := func() {
		base = c.scripts(base, topLeft, bottomLeft, top, bottom, start)
		topLeft, bottomLeft, top, bottom = nil, nil, nil, nil
	}
	for c.pos < len(c.tokens) {
		t := c.current()
		if t.is(tokSpace) || c.closesAnyGroup() || c.arrowAhead() {
			break
		}
		switch {
		case t.is(tokSuperscript):
			c.pos++
			script := c.parseScriptContent(t, depth, compact)
			if top != nil {
				foldScripts()
			}
			top = script
		case t.is(tokSubscript):
			c.pos++
			script := c.parseScriptContent(t, depth, compact)
			if bottom != nil {
				foldScripts()
			}
			bottom = script
		case !compact && c.implicitChargeAhead():
			top = c.parseImplicitCharge()
		case !compact && c.implicitSubscriptAhead():
			bottom = c.parseBareScript()
		default:
			goto suffixesDone
		}
	}

suffixesDone:
	if top == nil && bottom == nil && topLeft == nil && bottomLeft == nil {
		return base
	}
	return c.scripts(base, topLeft, bottomLeft, top, bottom, start)
}

func (c *chemistryParser) parseAtom(depth int, compact bool) astNode {
	if c.pos >= len(c.tokens) {
		return nil
	}
	t := c.current()
	switch {
	case t.is(tokBeginGroup):
		c.pos++
		return c.parseDelimitedBlock(t, chemistryClose{kind: tokEndGroup, text: "}"}, depth, compact, "", "")
	case t.is(tokChar) && (t.text == "(" || t.text == "["):
		c.pos++
		closeText := ")"
		if t.text == "[" {
			closeText = "]"
		}
		return c.parseDelimitedBlock(
			t,
			chemistryClose{kind: tokChar, text: closeText},
			depth,
			compact,
			t.text,
			closeText,
		)
	case t.is(tokChar) && t.text == "$":
		return c.parseUnsupportedMathSegment()
	case t.is(tokChar) && unicode.IsLetter([]rune(t.text)[0]):
		return c.parseTextAtom()
	case t.is(tokChar) && unicode.IsDigit([]rune(t.text)[0]):
		return c.parseTextAtom()
	case t.is(tokChar) && (t.text == "*" || t.text == "."):
		if t.text == "." && !compact && c.additionCompoundAhead() {
			c.pos++
			return c.additionCompound(t)
		}
		c.pos++
		return c.glyph(t, t.text, ir.ClassNormal)
	case t.isControl():
		return c.parseControlAtom(depth, compact)
	case t.is(tokAlignment) || t.is(tokParameter):
		c.pos++
		c.report(t.span, source.SeverityError, source.CodeUnsupported, "unsupported character in chemistry markup")
		return c.errorNode(t.span, t.text, source.CodeUnsupported)
	default:
		c.pos++
		c.report(t.span, source.SeverityError, source.CodeSyntax, "unexpected token in chemistry markup")
		return c.errorNode(t.span, t.text, source.CodeSyntax)
	}
}

func (c *chemistryParser) parseTextAtom() astNode {
	start := c.pos
	isUpper := false
	runes := []rune(c.current().text)
	if len(runes) == 1 {
		isUpper = unicode.IsUpper(runes[0])
	}
	c.pos++
	if isUpper {
		lower := 0
		for c.pos < len(c.tokens) && lower < 3 && c.current().is(tokChar) {
			r, size := utf8.DecodeRuneInString(c.current().text)
			if size != len(c.current().text) || !unicode.IsLower(r) {
				break
			}
			c.pos++
			lower++
		}
	} else {
		for c.pos < len(c.tokens) && c.current().is(tokChar) {
			r, size := utf8.DecodeRuneInString(c.current().text)
			if size != len(c.current().text) || !unicode.IsLetter(r) && !unicode.IsDigit(r) {
				break
			}
			c.pos++
		}
	}
	return c.textFromTokens(c.tokens[start:c.pos], ir.VariantNormal)
}

func (c *chemistryParser) parseDigitRun() astNode {
	start := c.pos
	for c.pos < len(c.tokens) && c.current().is(tokChar) {
		r, size := utf8.DecodeRuneInString(c.current().text)
		if size != len(c.current().text) || !unicode.IsDigit(r) {
			break
		}
		c.pos++
	}
	if c.pos == start {
		return nil
	}
	return c.numberFromTokens(c.tokens[start:c.pos])
}

func (c *chemistryParser) parseDelimitedBlock(
	open token,
	close chemistryClose,
	depth int,
	compact bool,
	visibleOpen string,
	visibleClose string,
) astNode {
	items, closed, closeSpan := c.parseItems(close, depth+1, compact)
	span := source.Merge(open.span, closeSpan)
	origin := source.Merge(tokenOrigin(open), closeSpan)
	if !closed {
		c.report(open.span, source.SeverityError, source.CodeUnclosedGroup, "unclosed chemistry group")
	}
	if visibleOpen == "" {
		return c.rowWithBase(items, span, origin)
	}
	fenced := &astFencedNode{
		astBase:  newASTBaseAt(astFenced, span, origin),
		body:     items,
		balanced: closed,
	}
	if !c.reserve(open.span) {
		return c.errorNode(span, "chemistry node limit exceeded", source.CodeLimitExceeded)
	}
	fenced.open = c.glyph(open, visibleOpen, ir.ClassOpening)
	if closed && c.pos > 0 {
		closing := c.tokens[c.pos-1]
		fenced.close = c.glyph(closing, visibleClose, ir.ClassClosing)
	}
	return fenced
}

func (c *chemistryParser) parseControlAtom(depth int, compact bool) astNode {
	t := c.current()
	switch t.text {
	case "\\{", "\\}":
		if t.text == "\\}" {
			c.pos++
			c.report(t.span, source.SeverityError, source.CodeUnexpectedToken, "unexpected escaped closing brace in chemistry markup")
			return c.errorNode(t.span, "}", source.CodeUnexpectedToken)
		}
		c.pos++
		return c.parseDelimitedBlock(
			t,
			chemistryClose{kind: tokControlSymbol, text: "\\}", control: true},
			depth,
			compact,
			"{",
			"}",
		)
	case "\\text", "\\textrm", "\\textnormal", "\\mathrm":
		c.pos++
		return c.parseLiteralTextCommand(t)
	case "\\ce", "\\pu":
		c.pos++
		return c.parseNestedChemistry(t, depth)
	case "\\bond":
		return c.parseBond()
	}
	if spec, ok := symbolCommands[t.text]; ok {
		c.pos++
		if spec.class == ir.ClassLarge {
			c.reserve(t.span)
			return &astOperatorNode{
				astBase:        newASTBaseAt(astOperator, t.span, tokenOrigin(t)),
				text:           spec.text,
				variant:        spec.variant,
				limits:         spec.limits,
				semanticLimits: spec.limits != ir.LimitsNever,
			}
		}
		return c.astGlyphSpec(t, spec)
	}
	c.pos++
	c.report(t.span, source.SeverityError, source.CodeUnsupported, "unsupported command in chemistry markup: "+t.text)
	return c.errorNode(t.span, t.text, source.CodeUnsupported)
}

func (c *chemistryParser) parseLiteralTextCommand(command token) astNode {
	if c.pos >= len(c.tokens) || !c.current().is(tokBeginGroup) {
		c.report(command.span, source.SeverityError, source.CodeMissingArgument, command.text+" requires a braced argument in chemistry markup")
		return c.errorNode(command.span, command.text, source.CodeMissingArgument)
	}
	open := c.current()
	c.pos++
	var content []token
	depth := 1
	for c.pos < len(c.tokens) {
		t := c.current()
		c.pos++
		if t.is(tokBeginGroup) {
			depth++
			content = append(content, t)
			continue
		}
		if t.is(tokEndGroup) {
			depth--
			if depth == 0 {
				break
			}
			content = append(content, t)
			continue
		}
		content = append(content, t)
	}
	if depth != 0 {
		c.report(open.span, source.SeverityError, source.CodeUnclosedGroup, "unclosed "+command.text+" argument")
	}
	text := c.textFromTokens(content, ir.VariantNormal)
	if text == nil {
		c.report(source.Merge(command.span, open.span), source.SeverityError, source.CodeInvalidValue, command.text+" argument is empty")
		return newPlaceholderAST(source.Merge(command.span, open.span), "empty chemistry text")
	}
	text.span = source.Merge(command.span, open.span)
	text.origin = tokenOrigin(command)
	return text
}

func (c *chemistryParser) parseNestedChemistry(command token, depth int) astNode {
	if depth >= c.p.limits.MaxNesting {
		c.p.limit(command.span, "chemistry nesting limit exceeded")
		return c.errorNode(command.span, "chemistry nesting limit exceeded", source.CodeLimitExceeded)
	}
	if c.pos >= len(c.tokens) || !c.current().is(tokBeginGroup) {
		c.report(command.span, source.SeverityError, source.CodeMissingArgument, command.text+" requires a braced argument")
		return c.errorNode(command.span, command.text, source.CodeMissingArgument)
	}
	open := c.current()
	c.pos++
	body, closed, closeSpan := c.parseItems(chemistryClose{kind: tokEndGroup, text: "}"}, depth+1, command.text == "\\pu")
	if !closed {
		c.report(open.span, source.SeverityError, source.CodeUnclosedGroup, "unclosed "+command.text+" argument")
	}
	span := source.Merge(command.span, closeSpan)
	if len(body) == 0 {
		c.report(span, source.SeverityError, source.CodeInvalidValue, command.text+" argument is empty")
		return newPlaceholderAST(span, "empty chemistry markup")
	}
	row := c.rowWithBase(body, span, tokenOrigin(command))
	return row
}

func (c *chemistryParser) parseScriptContent(marker token, depth int, compact bool) astNode {
	c.consumeSpaces()
	if c.pos >= len(c.tokens) || c.closesAnyGroup() {
		c.report(marker.span, source.SeverityError, source.CodeMissingArgument, "empty chemistry script")
		return c.errorNode(marker.span, marker.text, source.CodeMissingArgument)
	}
	if c.current().is(tokBeginGroup) {
		open := c.current()
		c.pos++
		items, closed, closeSpan := c.parseItems(chemistryClose{kind: tokEndGroup, text: "}"}, depth+1, true)
		if !closed {
			c.report(open.span, source.SeverityError, source.CodeUnclosedGroup, "unclosed chemistry script")
		}
		if len(items) == 1 {
			return items[0]
		}
		return c.rowWithBase(items, source.Merge(open.span, closeSpan), tokenOrigin(open))
	}
	return c.parseBareScript()
}

func (c *chemistryParser) parseBareScript() astNode {
	start := c.pos
	var items []astNode
	if c.pos < len(c.tokens) {
		t := c.current()
		switch {
		case t.is(tokChar) && isChemSign(t.text):
			for c.pos < len(c.tokens) && c.current().is(tokChar) && isChemSign(c.current().text) {
				items = append(items, c.glyph(c.current(), chemistrySignText(c.current().text), ir.ClassNormal))
				c.pos++
			}
			if c.pos < len(c.tokens) && c.current().is(tokChar) && unicode.IsDigit([]rune(c.current().text)[0]) {
				items = append(items, c.parseDigitRun())
			}
		case t.is(tokChar) && unicode.IsDigit([]rune(t.text)[0]):
			items = append(items, c.parseDigitRun())
			for c.pos < len(c.tokens) && c.current().is(tokChar) && isChemSign(c.current().text) {
				items = append(items, c.glyph(c.current(), chemistrySignText(c.current().text), ir.ClassNormal))
				c.pos++
			}
		case t.is(tokChar) && (t.text == "*" || t.text == "."):
			c.pos++
			items = append(items, c.glyph(t, t.text, ir.ClassNormal))
		case t.is(tokChar) && unicode.IsLetter([]rune(t.text)[0]):
			items = append(items, c.parseTextAtom())
			for c.pos < len(c.tokens) && c.current().is(tokChar) && isChemSign(c.current().text) {
				items = append(items, c.glyph(c.current(), chemistrySignText(c.current().text), ir.ClassNormal))
				c.pos++
			}
		default:
			item := c.parseAtom(1, true)
			if item != nil {
				items = append(items, item)
			}
		}
	}
	if len(items) == 0 {
		return c.errorNode(c.currentSpan(), "empty chemistry script", source.CodeMissingArgument)
	}
	span := source.Detached()
	for _, item := range items {
		if text, ok := item.(*astTextNode); ok && len(text.clusters) > 0 {
			span = source.Merge(span, text.span)
		} else {
			span = source.Merge(span, item.base().span)
		}
	}
	if span.IsDetached() && start < len(c.tokens) {
		span = c.tokens[start].span
	}
	if len(items) == 1 {
		return items[0]
	}
	return c.rowWithBase(items, span, span)
}

func (c *chemistryParser) parseImplicitCharge() astNode {
	start := c.pos
	var items []astNode
	for c.pos < len(c.tokens) && c.current().is(tokChar) && isChemSign(c.current().text) {
		items = append(items, c.glyph(c.current(), chemistrySignText(c.current().text), ir.ClassNormal))
		c.pos++
	}
	span := c.tokens[start].span
	if c.pos > start+1 {
		span = source.Merge(span, c.tokens[c.pos-1].span)
	}
	return c.rowWithBase(items, span, span)
}

func (c *chemistryParser) implicitChargeAhead() bool {
	if c.pos >= len(c.tokens) || !c.current().is(tokChar) || !isChemSign(c.current().text) {
		return false
	}
	if c.spaceBefore(c.pos) {
		return false
	}
	next := c.pos + 1
	if next >= len(c.tokens) || c.tokens[next].is(tokSpace) || c.isGroupClose(c.tokens[next]) {
		return true
	}
	t := c.tokens[next]
	return t.is(tokChar) && (t.text == "," || t.text == ";" || t.text == "+" || t.text == "-" || t.text == "=")
}

func (c *chemistryParser) implicitSubscriptAhead() bool {
	if c.pos >= len(c.tokens) || !c.current().is(tokChar) || c.spaceBefore(c.pos) {
		return false
	}
	r, size := utf8.DecodeRuneInString(c.current().text)
	return size == len(c.current().text) && unicode.IsDigit(r)
}

func (c *chemistryParser) appendScript(existing, next astNode) astNode {
	if existing == nil {
		return next
	}
	if next == nil {
		return existing
	}
	span := source.Merge(existing.base().span, next.base().span)
	origin := source.Merge(existing.base().origin, next.base().origin)
	return c.rowWithBase([]astNode{existing, next}, span, origin)
}

func (c *chemistryParser) scripts(
	base astNode,
	topLeft astNode,
	bottomLeft astNode,
	top astNode,
	bottom astNode,
	start source.Range,
) astNode {
	if !c.reserve(start) {
		return c.errorNode(start, "chemistry node limit exceeded", source.CodeLimitExceeded)
	}
	span := base.base().span
	origin := base.base().origin
	for _, child := range []astNode{topLeft, bottomLeft, top, bottom} {
		if child != nil {
			span = source.Merge(span, child.base().span)
			origin = source.Merge(origin, child.base().origin)
		}
	}
	return &astScriptsNode{
		astBase:    newASTBaseAt(astScripts, span, origin),
		main:       base,
		top:        top,
		bottom:     bottom,
		topLeft:    topLeft,
		bottomLeft: bottomLeft,
		limits:     ir.LimitsAuto,
		role:       ir.ScriptRoleOrdinary,
	}
}
