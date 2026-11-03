package tex

import (
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/caelyreth/untie/ir"
	"github.com/caelyreth/untie/source"
)

func (c *chemistryParser) textFromTokens(tokens []token, variant ir.Variant) *astTextNode {
	if len(tokens) == 0 {
		return nil
	}
	var text strings.Builder
	var clusters []ir.TextClusterSource
	span := source.Detached()
	origin := source.Detached()
	orderOK := true
	lastEnd := 0
	for _, token := range tokens {
		if token.is(tokSpace) {
			text.WriteByte(' ')
			span = source.Merge(span, token.span)
			origin = source.Merge(origin, token.origin)
			continue
		}
		value := token.text
		if value == "\\{" || value == "\\}" {
			value = strings.TrimPrefix(value, "\\")
		}
		if token.is(tokBeginGroup) || token.is(tokEndGroup) {
			value = token.text
		}
		start := text.Len()
		text.WriteString(value)
		clusterSpan := normalizeRange(token.span)
		if !clusterSpan.IsDetached() && clusterSpan.Start >= lastEnd {
			clusters = append(clusters, ir.TextClusterSource{
				TextStart: start,
				TextEnd:   text.Len(),
				Source:    clusterSpan,
			})
			lastEnd = clusterSpan.End
		} else {
			orderOK = false
		}
		span = source.Merge(span, token.span)
		origin = source.Merge(origin, token.origin)
	}
	node := &astTextNode{
		astBase:  newASTBaseAt(astText, span, origin),
		text:     text.String(),
		variant:  variant,
		clusters: clusters,
	}
	if !orderOK {
		node.clusters = nil
	}
	c.reserve(span)
	return node
}

func (c *chemistryParser) numberFromTokens(tokens []token) astNode {
	var items []astNode
	for _, token := range tokens {
		if token.is(tokSpace) {
			continue
		}
		items = append(items, c.glyph(token, token.text, ir.ClassNormal))
	}
	if len(items) == 0 {
		return nil
	}
	if len(items) == 1 {
		return items[0]
	}
	return c.rowWithBase(items, astSpan(items...), astOrigin(items...))
}

func (c *chemistryParser) textAt(span, origin source.Range, text string, variant ir.Variant) *astTextNode {
	c.reserve(span)
	return &astTextNode{
		astBase: newASTBaseAt(astText, span, origin),
		text:    text,
		variant: variant,
	}
}

func (c *chemistryParser) glyph(t token, text string, class ir.MathClass) *astGlyphNode {
	c.reserve(t.span)
	return c.p.arena.allocGlyph(t.span, tokenOrigin(t), text, class, ir.VariantAuto, ir.LimitsAuto)
}

func (c *chemistryParser) glyphAt(span, origin source.Range, text string, class ir.MathClass) *astGlyphNode {
	c.reserve(span)
	return c.p.arena.allocGlyph(span, origin, text, class, ir.VariantAuto, ir.LimitsAuto)
}

func (c *chemistryParser) astGlyphSpec(t token, spec symbolSpec) *astGlyphNode {
	c.reserve(t.span)
	return c.p.astGlyphSpec(t.span, tokenOrigin(t), spec)
}

func (c *chemistryParser) errorNode(span source.Range, text string, code source.Code) astNode {
	c.reserve(span)
	return astErr(span, text, code)
}

func (c *chemistryParser) row(items []astNode, span source.Range) astNode {
	if len(items) == 0 {
		return nil
	}
	if !span.IsDetached() {
		return c.rowWithBase(items, span, span)
	}
	return c.rowWithBase(items, astSpan(items...), astOrigin(items...))
}

func (c *chemistryParser) rowWithBase(items []astNode, span, origin source.Range) *astRowNode {
	c.reserve(span)
	return &astRowNode{
		astBase: newASTBaseAt(astRow, span, origin),
		items:   items,
	}
}

func (c *chemistryParser) explicitSpace(span source.Range, mus int64) *astSpaceNode {
	c.reserve(span)
	amount, _ := ir.MuLength(float64(mus))
	return &astSpaceNode{
		astBase: newASTBaseAt(astSpace, span, span),
		kind:    ir.SpaceExplicit,
		amount:  amount,
	}
}

func (c *chemistryParser) generatedSpace(mus int64) *astSpaceNode {
	return c.explicitSpace(source.Detached(), mus)
}

func (c *chemistryParser) reserve(span source.Range) bool {
	if c.exhausted {
		return false
	}
	if c.nodes >= c.p.limits.MaxNodes {
		c.p.limit(span, "chemistry node limit exceeded")
		c.exhausted = true
		return false
	}
	c.nodes++
	return true
}

func (c *chemistryParser) report(span source.Range, severity source.Severity, code source.Code, message string) {
	c.p.report(source.Diagnostic{
		Range:    normalizeRange(span),
		Severity: severity,
		Code:     code,
		Message:  message,
	})
}

func (c *chemistryParser) current() token {
	if c.pos >= len(c.tokens) {
		return token{kind: tokEOF}
	}
	return c.tokens[c.pos]
}

func (c *chemistryParser) currentSpan() source.Range {
	if c.pos >= len(c.tokens) {
		if len(c.tokens) == 0 {
			return source.Detached()
		}
		return c.tokens[len(c.tokens)-1].span
	}
	return c.tokens[c.pos].span
}

func (c *chemistryParser) remainingSpan() source.Range {
	if c.pos >= len(c.tokens) {
		return c.currentSpan()
	}
	return source.Merge(c.tokens[c.pos].span, c.tokens[len(c.tokens)-1].span)
}

func (c *chemistryParser) consumeSpaces() {
	for c.pos < len(c.tokens) && c.tokens[c.pos].is(tokSpace) {
		c.pos++
	}
}

func (c *chemistryParser) closes(close chemistryClose) bool {
	if close.kind == tokEOF || c.pos >= len(c.tokens) {
		return false
	}
	t := c.tokens[c.pos]
	if t.kind != close.kind || t.text != close.text {
		return false
	}
	return !close.control || t.isControl()
}

func (c *chemistryParser) isGroupClose(t token) bool {
	return t.is(tokEndGroup) ||
		t.is(tokChar) && (t.text == ")" || t.text == "]") ||
		t.isControl() && t.text == "\\}"
}

func (c *chemistryParser) closesAnyGroup() bool {
	return c.pos < len(c.tokens) && c.isGroupClose(c.tokens[c.pos])
}

func (c *chemistryParser) arrowAhead() bool {
	_, _, _, ok := c.arrowAt()
	return ok
}

func (c *chemistryParser) annotationAhead() bool {
	return c.pos < len(c.tokens) && c.tokens[c.pos].is(tokChar) && c.tokens[c.pos].text == "["
}

func (c *chemistryParser) nextStartsAtom(index int) bool {
	index = c.nextNonSpace(index)
	if index >= len(c.tokens) {
		return false
	}
	t := c.tokens[index]
	if t.is(tokBeginGroup) {
		return true
	}
	if t.isControl() {
		if t.text == "\\{" || t.text == "\\ce" || t.text == "\\pu" || t.text == "\\bond" {
			return true
		}
		_, ok := symbolCommands[t.text]
		return ok
	}
	if !t.is(tokChar) {
		return false
	}
	r, _ := utf8.DecodeRuneInString(t.text)
	return unicode.IsLetter(r) || unicode.IsDigit(r) || t.text == "(" || t.text == "["
}

func (c *chemistryParser) nextNonSpace(index int) int {
	for index < len(c.tokens) && c.tokens[index].is(tokSpace) {
		index++
	}
	return index
}

func (c *chemistryParser) spaceBefore(index int) bool {
	if index <= 0 || index > len(c.tokens) {
		return false
	}
	if c.tokens[index-1].is(tokSpace) {
		return true
	}
	previous := c.tokens[index-1]
	current := c.tokens[index]
	return !previous.span.IsDetached() && !current.span.IsDetached() &&
		previous.span.End < current.span.Start
}

func (c *chemistryParser) spaceAfter(index int) bool {
	if index+1 >= len(c.tokens) {
		return false
	}
	if c.tokens[index+1].is(tokSpace) {
		return true
	}
	current := c.tokens[index]
	next := c.tokens[index+1]
	return !current.span.IsDetached() && !next.span.IsDetached() &&
		current.span.End < next.span.Start
}

func (c *chemistryParser) rawText(tokens []token) string {
	var out strings.Builder
	for _, token := range tokens {
		out.WriteString(token.text)
	}
	return out.String()
}

func (c *chemistryParser) sourceText(span source.Range) string {
	if span.IsDetached() || !span.IsValid() || span.Start < 0 || span.End > len(c.p.src) || span.Start > span.End {
		return ""
	}
	return c.p.src[span.Start:span.End]
}

func isChemSign(text string) bool {
	return text == "+" || text == "-" || text == "±"
}

func chemistrySignText(text string) string {
	if text == "-" {
		return "−"
	}
	return text
}
