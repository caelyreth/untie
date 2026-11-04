package tex

import (
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/caelyreth/untie/ir"
	"github.com/caelyreth/untie/source"
)

func (c *chemistryParser) matchArrow() (astNode, bool) {
	consumed, span, glyphText, ok := c.arrowAt()
	if !ok {
		return nil, false
	}
	start := c.tokens[c.pos].span
	c.pos += consumed
	arrow := c.glyphAt(span, start, glyphText, ir.ClassRelation)

	var top, bottom astNode
	c.consumeSpaces()
	if c.annotationAhead() {
		top, _ = c.parseAnnotation()
		if !c.spaceBefore(c.pos) && c.annotationAhead() {
			bottom, _ = c.parseAnnotation()
		}
	}
	if top == nil && bottom == nil {
		return arrow, true
	}
	if !c.reserve(span) {
		return c.errorNode(span, "chemistry node limit exceeded", source.CodeLimitExceeded), true
	}
	scripts := c.scripts(arrow, nil, nil, top, bottom, start)
	if node, ok := scripts.(*astScriptsNode); ok {
		node.limits = ir.LimitsAlways
		node.role = ir.ScriptRoleLimits
	}
	return scripts, true
}

func (c *chemistryParser) matchUpDownArrow() (astNode, bool) {
	if c.pos >= len(c.tokens) {
		return nil, false
	}
	t := c.current()
	beforeBoundary := c.pos == 0 || c.spaceBefore(c.pos) || c.previousIsDelimiter()
	afterBoundary := c.spaceAfter(c.pos) || c.nextIsDelimiter(c.pos+1)
	if t.is(tokSuperscript) && beforeBoundary && afterBoundary {
		c.pos++
		return c.glyph(t, "↑", ir.ClassRelation), true
	}
	if !t.is(tokChar) || t.text != "v" {
		return nil, false
	}
	if !beforeBoundary || !afterBoundary {
		return nil, false
	}
	c.pos++
	return c.glyph(t, "↓", ir.ClassRelation), true
}

func (c *chemistryParser) previousIsDelimiter() bool {
	if c.pos == 0 {
		return true
	}
	previous := c.tokens[c.pos-1]
	return previous.is(tokChar) && (previous.text == "(" || previous.text == "[")
}

func (c *chemistryParser) nextIsDelimiter(index int) bool {
	index = c.nextNonSpace(index)
	if index >= len(c.tokens) {
		return true
	}
	t := c.tokens[index]
	return c.isGroupClose(t) || t.is(tokChar) && (t.text == "(" || t.text == "[")
}

func (c *chemistryParser) arrowAt() (int, source.Range, string, bool) {
	if c.pos >= len(c.tokens) || !c.tokens[c.pos].is(tokChar) {
		return 0, source.Detached(), "", false
	}
	end := c.pos
	var text strings.Builder
	for end < len(c.tokens) && c.tokens[end].is(tokChar) {
		if end > c.pos && c.spaceBefore(end) {
			break
		}
		text.WriteString(c.tokens[end].text)
		end++
		if text.Len() > 8 {
			break
		}
	}
	input := text.String()
	var glyphText string
	var consumed int
	for _, candidate := range []struct {
		source string
		glyph  string
	}{
		{"<=>>", "⇌"},
		{"<<=>", "⇋"},
		{"<-->", "⇄"},
		{"<=>", "⇌"},
		{"<->", "↔"},
		{"->", "→"},
		{"<-", "←"},
		{"⟶", "⟶"},
		{"→", "→"},
		{"⇌", "⇌"},
		{"⇄", "⇄"},
		{"↔", "↔"},
		{"←", "←"},
	} {
		if strings.HasPrefix(input, candidate.source) {
			glyphText = candidate.glyph
			consumed = utf8.RuneCountInString(candidate.source)
			break
		}
	}
	if consumed == 0 {
		return 0, source.Detached(), "", false
	}
	start := c.tokens[c.pos].span
	endToken := c.tokens[c.pos+consumed-1]
	span := source.Merge(start, endToken.span)
	return consumed, span, glyphText, true
}

func (c *chemistryParser) parseAnnotation() (astNode, source.Range) {
	if !c.annotationAhead() {
		return nil, source.Detached()
	}
	open := c.current()
	c.pos++
	items, closed, closeSpan := c.parseItems(
		chemistryClose{kind: tokChar, text: "]", control: false},
		1,
		false,
	)
	if !closed {
		c.report(open.span, source.SeverityError, source.CodeUnclosedGroup, "unclosed reaction annotation")
	}
	span := source.Merge(open.span, closeSpan)
	return c.rowWithBase(items, span, tokenOrigin(open)), span
}

func (c *chemistryParser) parseBond() astNode {
	command := c.current()
	if command.text != "\\bond" {
		return nil
	}
	c.pos++
	if c.pos >= len(c.tokens) || !c.current().is(tokBeginGroup) {
		c.report(command.span, source.SeverityError, source.CodeMissingArgument, "\\bond requires a braced argument in chemistry markup")
		return c.errorNode(command.span, "\\bond", source.CodeMissingArgument)
	}
	open := c.current()
	c.pos++
	start := c.pos
	depth := 1
	closed := false
	for c.pos < len(c.tokens) {
		t := c.current()
		c.pos++
		if t.is(tokBeginGroup) {
			depth++
		} else if t.is(tokEndGroup) {
			depth--
			if depth == 0 {
				closed = true
				break
			}
		}
	}
	if depth != 0 {
		c.report(open.span, source.SeverityError, source.CodeUnclosedGroup, "unclosed \\bond argument")
	}
	end := c.pos
	if closed {
		end--
	}
	kind := c.rawText(c.tokens[start:end])
	span := source.Merge(command.span, open.span)
	if c.pos > start {
		span = source.Merge(span, c.tokens[c.pos-1].span)
	}
	switch kind {
	case "-", "1":
		return c.glyphAt(span, command.origin, "−", ir.ClassNormal)
	case "=", "2":
		return c.glyphAt(span, command.origin, "=", ir.ClassNormal)
	case "#", "3":
		return c.glyphAt(span, command.origin, "≡", ir.ClassNormal)
	case "->":
		return c.glyphAt(span, command.origin, "→", ir.ClassNormal)
	case "<-":
		return c.glyphAt(span, command.origin, "←", ir.ClassNormal)
	case "...":
		return c.glyphAt(span, command.origin, "⋯", ir.ClassNormal)
	case "....":
		return c.glyphAt(span, command.origin, "⋯⋯", ir.ClassNormal)
	default:
		c.report(span, source.SeverityError, source.CodeUnsupported, "unsupported \\bond kind "+kind)
		return c.errorNode(span, kind, source.CodeUnsupported)
	}
}

func (c *chemistryParser) matchExplicitOperator() bool {
	if c.pos >= len(c.tokens) {
		return false
	}
	if c.current().is(tokParameter) && c.current().text == "#" {
		return true
	}
	if !c.current().is(tokChar) {
		return false
	}
	switch c.current().text {
	case "+", "-", "=", "<", ">", ",", ";":
		return true
	case "±":
		return true
	}
	return false
}

func (c *chemistryParser) parseExplicitOperator() astNode {
	t := c.current()
	text := t.text
	class := ir.ClassBinary
	switch text {
	case "<", ">":
		class = ir.ClassRelation
	case ",", ";":
		class = ir.ClassPunctuation
	case "-":
		text = "−"
		if !c.spaceBefore(c.pos) && !c.spaceAfter(c.pos) && c.nextStartsAtom(c.pos+1) {
			class = ir.ClassNormal
		}
	case "=":
		class = ir.ClassRelation
		if !c.spaceBefore(c.pos) && !c.spaceAfter(c.pos) && c.nextStartsAtom(c.pos+1) {
			class = ir.ClassNormal
		}
	case "#":
		text = "≡"
		class = ir.ClassNormal
	}
	c.pos++
	return c.glyph(t, text, class)
}

func (c *chemistryParser) coefficientAhead() bool {
	i := c.pos
	if i < len(c.tokens) && c.tokens[i].is(tokChar) && isChemSign(c.tokens[i].text) {
		i++
	}
	if i >= len(c.tokens) || !c.tokens[i].is(tokChar) {
		return false
	}
	r, size := utf8.DecodeRuneInString(c.tokens[i].text)
	return size == len(c.tokens[i].text) && unicode.IsDigit(r)
}

func (c *chemistryParser) parseCoefficient() astNode {
	start := c.pos
	var items []astNode
	if c.current().is(tokChar) && isChemSign(c.current().text) {
		items = append(items, c.glyph(c.current(), chemistrySignText(c.current().text), ir.ClassNormal))
		c.pos++
	}
	numberStart := c.pos
	for c.pos < len(c.tokens) && c.tokens[c.pos].is(tokChar) {
		t := c.tokens[c.pos]
		r, size := utf8.DecodeRuneInString(t.text)
		if size != len(t.text) || !unicode.IsDigit(r) && r != '.' && r != ',' {
			break
		}
		c.pos++
	}
	if c.pos == numberStart {
		return c.errorNode(c.tokens[start].span, "invalid coefficient", source.CodeInvalidValue)
	}
	numerator := c.numberFromTokens(c.tokens[numberStart:c.pos])
	if c.pos+1 < len(c.tokens) && c.current().is(tokChar) && c.current().text == "/" &&
		c.tokens[c.pos+1].is(tokChar) && unicode.IsDigit([]rune(c.tokens[c.pos+1].text)[0]) {
		slash := c.current()
		c.pos++
		denomStart := c.pos
		for c.pos < len(c.tokens) && c.tokens[c.pos].is(tokChar) && unicode.IsDigit([]rune(c.tokens[c.pos].text)[0]) {
			c.pos++
		}
		denominator := c.numberFromTokens(c.tokens[denomStart:c.pos])
		if c.reserve(slash.span) {
			items = append(items, &astFractionNode{
				astBase: newASTBaseAt(
					astFraction,
					source.Merge(numerator.base().span, denominator.base().span),
					tokenOrigin(c.tokens[start]),
				),
				numerator:   numerator,
				denominator: denominator,
				bar:         true,
				style:       ir.FractionVertical,
				role:        ir.FractionRoleFraction,
				size:        ir.SizeText,
			})
		}
	} else {
		items = append(items, numerator)
	}
	if len(items) == 1 {
		return items[0]
	}
	span := source.Merge(c.tokens[start].span, c.tokens[c.pos-1].span)
	return c.rowWithBase(items, span, c.tokens[start].origin)
}

func (c *chemistryParser) additionCompoundAhead() bool {
	return c.pos+1 < len(c.tokens) && c.nextStartsAtom(c.pos+1)
}

func (c *chemistryParser) additionCompound(t token) astNode {
	dot := c.glyph(t, "·", ir.ClassNormal)
	space := c.generatedSpace(2)
	return c.rowWithBase([]astNode{space, dot, space}, t.span, tokenOrigin(t))
}

func (c *chemistryParser) parseEmbeddedMathSegment() astNode {
	open := c.current()
	c.pos++
	start := c.pos
	closed := false
	for c.pos < len(c.tokens) {
		t := c.current()
		c.pos++
		if t.is(tokChar) && t.text == "$" {
			closed = true
			break
		}
	}
	end := c.pos
	if closed {
		end--
	}
	span := open.span
	if c.pos > 0 {
		span = source.Merge(span, c.tokens[c.pos-1].span)
	}
	if !closed {
		c.report(span, source.SeverityError, source.CodeUnclosedGroup, "unclosed embedded TeX math")
	}
	if start == end {
		c.report(span, source.SeverityError, source.CodeInvalidValue, "embedded TeX math is empty")
		return c.errorNode(span, "$", source.CodeInvalidValue)
	}
	body := c.p.parseEmbeddedMath(c.tokens[start:end])
	if body == nil {
		c.report(span, source.SeverityError, source.CodeInvalidValue, "embedded TeX math is empty")
		return c.errorNode(span, c.sourceText(span), source.CodeInvalidValue)
	}
	body.base().span = span
	body.base().origin = span
	return body
}
