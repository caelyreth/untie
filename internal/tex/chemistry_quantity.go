package tex

import (
	"unicode"
	"unicode/utf8"

	"github.com/caelyreth/untie/ir"
	"github.com/caelyreth/untie/source"
)

func (c *chemistryParser) parsePhysicalQuantity() astNode {
	c.consumeSpaces()
	if c.pos >= len(c.tokens) {
		return nil
	}
	var items []astNode
	number := c.parseQuantityNumber()
	if number == nil {
		t := c.current()
		c.report(t.span, source.SeverityError, source.CodeInvalidValue, "\\pu requires a numeric quantity")
		errorSpan := t.span
		errorText := t.text
		if t.is(tokChar) && unicode.IsLetter([]rune(t.text)[0]) {
			unit := c.parseUnitSymbol()
			errorSpan = unit.base().span
			errorText = c.sourceText(errorSpan)
		} else {
			c.pos++
		}
		if errorText == "" {
			errorText = t.text
		}
		items = append(items, c.errorNode(errorSpan, errorText, source.CodeInvalidValue))
	} else {
		items = append(items, number)
	}
	if number != nil && c.pos < len(c.tokens) &&
		c.current().is(tokChar) && c.current().text == "(" &&
		!c.spaceBefore(c.pos) {
		span := c.consumeUncertaintyGroup()
		c.report(span, source.SeverityError, source.CodeUnsupported, "measurement uncertainty notation is outside the bounded \\pu grammar")
		text := c.sourceText(span)
		if text == "" {
			text = "(...)"
		}
		items = append(items, c.errorNode(span, text, source.CodeUnsupported))
	}

	for c.pos < len(c.tokens) && !c.exhausted {
		if c.current().is(tokSpace) {
			start := c.current().span
			c.consumeSpaces()
			if c.pos < len(c.tokens) {
				items = append(items, c.explicitSpace(start, 3))
			}
			continue
		}
		t := c.current()
		if t.is(tokChar) && t.text == "/" {
			slashSpan := t.span
			c.pos++
			if c.pos < len(c.tokens) && c.current().is(tokChar) && c.current().text == "/" {
				slashSpan = source.Merge(slashSpan, c.current().span)
				c.pos++
			}
			items = append(
				items,
				c.generatedSpace(1),
				c.glyphAt(slashSpan, t.origin, "/", ir.ClassNormal),
				c.generatedSpace(1),
			)
			continue
		}
		if t.is(tokChar) && (t.text == "*" || t.text == ".") {
			c.pos++
			items = append(items, c.generatedSpace(1), c.glyph(t, "·", ir.ClassNormal), c.generatedSpace(1))
			continue
		}
		if t.is(tokChar) && t.text == "(" {
			c.pos++
			items = append(items, c.parseDelimitedBlock(t, chemistryClose{kind: tokChar, text: ")"}, 1, false, "(", ")"))
			continue
		}
		if t.isControl() {
			items = append(items, c.parseControlAtom(1, true))
			continue
		}
		if t.is(tokChar) && (unicode.IsLetter([]rune(t.text)[0]) || t.text == "%" || t.text == "°") {
			unit := c.parseUnitSymbol()
			items = append(items, c.attachUnitScripts(unit))
			continue
		}
		c.pos++
		c.report(t.span, source.SeverityError, source.CodeUnsupported, "unsupported token in \\pu unit")
		items = append(items, c.errorNode(t.span, t.text, source.CodeUnsupported))
	}
	return c.rowWithBase(items, source.Detached(), source.Detached())
}

func (c *chemistryParser) consumeUncertaintyGroup() source.Range {
	open := c.current()
	c.pos++
	depth := 1
	end := open.span
	for c.pos < len(c.tokens) {
		t := c.current()
		c.pos++
		end = source.Merge(end, t.span)
		if t.is(tokChar) && t.text == "(" {
			depth++
		} else if t.is(tokChar) && t.text == ")" {
			depth--
			if depth == 0 {
				break
			}
		}
	}
	return end
}

func (c *chemistryParser) parseQuantityNumber() astNode {
	start := c.pos
	signStart := c.pos
	if c.pos < len(c.tokens) && c.current().is(tokChar) && isChemSign(c.current().text) {
		c.pos++
	}
	digits := 0
	dot := false
	for c.pos < len(c.tokens) && c.current().is(tokChar) {
		r, size := utf8.DecodeRuneInString(c.current().text)
		if size != len(c.current().text) {
			break
		}
		if unicode.IsDigit(r) {
			digits++
			c.pos++
			continue
		}
		if (r == '.' || r == ',') && !dot {
			dot = true
			c.pos++
			continue
		}
		break
	}
	if digits == 0 {
		c.pos = start
		return nil
	}
	end := c.pos
	exponentToken := -1
	if c.pos < len(c.tokens) && c.current().is(tokChar) {
		r, size := utf8.DecodeRuneInString(c.current().text)
		if size == len(c.current().text) && (r == 'e' || r == 'E') {
			probe := c.pos + 1
			if probe < len(c.tokens) && c.tokens[probe].is(tokChar) && isChemSign(c.tokens[probe].text) {
				probe++
			}
			digitsStart := probe
			for probe < len(c.tokens) && c.tokens[probe].is(tokChar) && unicode.IsDigit([]rune(c.tokens[probe].text)[0]) {
				probe++
			}
			if probe > digitsStart {
				exponentToken = c.pos
				c.pos = probe
			}
		}
	}

	var items []astNode
	if signStart < end {
		items = append(items, c.numberFromTokens(c.tokens[signStart:end]))
	}
	if exponentToken >= 0 {
		times := c.glyphAt(c.tokens[exponentToken].span, c.tokens[exponentToken].origin, "×", ir.ClassBinary)
		var ten astNode = c.textAt(c.tokens[exponentToken].span, c.tokens[exponentToken].origin, "10", ir.VariantNormal)
		exponent := c.numberFromTokens(c.tokens[exponentToken+1 : c.pos])
		ten = c.scripts(ten, nil, nil, exponent, nil, c.tokens[exponentToken].span)
		items = append(items, c.generatedSpace(2), times, c.generatedSpace(2), ten)
	}
	if len(items) == 0 {
		c.pos = start
		return nil
	}
	if len(items) == 1 {
		return items[0]
	}
	return c.rowWithBase(items, source.Merge(c.tokens[start].span, c.tokens[c.pos-1].span), c.tokens[start].origin)
}

func (c *chemistryParser) parseUnitSymbol() astNode {
	start := c.pos
	if c.current().is(tokChar) && (c.current().text == "%" || c.current().text == "°") {
		t := c.current()
		c.pos++
		return c.glyph(t, t.text, ir.ClassNormal)
	}
	for c.pos < len(c.tokens) && c.current().is(tokChar) {
		r, size := utf8.DecodeRuneInString(c.current().text)
		if size != len(c.current().text) || !unicode.IsLetter(r) {
			break
		}
		c.pos++
	}
	return c.textFromTokens(c.tokens[start:c.pos], ir.VariantNormal)
}

func (c *chemistryParser) attachUnitScripts(base astNode) astNode {
	if c.pos >= len(c.tokens) || c.spaceBefore(c.pos) {
		return base
	}
	var top astNode
	if c.current().is(tokSuperscript) {
		marker := c.current()
		c.pos++
		top = c.parseScriptContent(marker, 1, true)
	} else if c.current().is(tokChar) && unicode.IsDigit([]rune(c.current().text)[0]) {
		top = c.parseBareScript()
	} else if c.current().is(tokChar) && isChemSign(c.current().text) {
		top = c.parseBareScript()
	}
	if top == nil {
		return base
	}
	return c.scripts(base, nil, nil, top, nil, base.base().span)
}
