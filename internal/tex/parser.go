package tex

import (
	"math"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/caelyreth/untie/ir"
	"github.com/caelyreth/untie/source"
)

type parser struct {
	src         string
	mode        ir.Mode
	exp         *expander
	limits      source.Limits
	diagnostics []source.Diagnostic
	depth       int
	iterations  int
	limitHit    bool
	leftDepth   int
}

type stopMode uint8

type combiningAccentSpec struct {
	mark     string
	position ir.Position
}

var combiningAccents = map[rune]combiningAccentSpec{
	'\u0300': {mark: "ˋ", position: ir.PositionAbove},
	'\u0301': {mark: "ˊ", position: ir.PositionAbove},
	'\u0302': {mark: "ˆ", position: ir.PositionAbove},
	'\u0303': {mark: "˜", position: ir.PositionAbove},
	'\u0304': {mark: "¯", position: ir.PositionAbove},
	'\u0306': {mark: "˘", position: ir.PositionAbove},
	'\u0307': {mark: "˙", position: ir.PositionAbove},
	'\u0308': {mark: "¨", position: ir.PositionAbove},
	'\u030A': {mark: "˚", position: ir.PositionAbove},
	'\u030B': {mark: "˝", position: ir.PositionAbove},
	'\u030C': {mark: "ˇ", position: ir.PositionAbove},
	'\u0327': {mark: "¸", position: ir.PositionBelow},
	'\u0332': {mark: "_", position: ir.PositionBelow},
	'\u20D7': {mark: "⃗", position: ir.PositionAbove},
}

const (
	stopEOF stopMode = iota
	stopGroup
	stopRight
	stopBracket
	stopCell
	stopDollar
	stopParen
	stopEndgroup
)

func newParser(src string, mode ir.Mode, macros map[string]Macro, limits source.Limits) *parser {
	p := &parser{src: src, mode: mode, limits: limits}
	p.exp = newExpander(src, limits, macros, p.report)
	return p
}

func (p *parser) parse() ir.Node {
	root := p.parseSequence(stopEOF)
	lower := newLowerer(p.limits, p.report)
	node := lower.lower(root, lowerContext{})
	if node == nil {
		node = ir.NewRow(source.NewRange(0, len(p.src)))
	}
	node.NodeBase().Span = source.NewRange(0, len(p.src))
	node.NodeBase().Origin = node.NodeBase().Span
	if p.mode == ir.Display {
		node.NodeBase().Size = ir.SizeDisplay
	} else {
		node.NodeBase().Size = ir.SizeText
	}
	return node
}

func (p *parser) parseSequence(stop stopMode) astNode {
	return p.parseSequenceWithInfix(stop, false)
}

func (p *parser) parseSequenceWithInfix(stop stopMode, infixSeen bool) astNode {
	p.depth++
	defer func() { p.depth-- }()

	var items []astNode
	start := source.NewRange(0, 0)
	first := true
	for {
		p.iterations++
		if p.iterations > p.limits.MaxTokens*2 {
			p.limit(source.Detached(), "TeX parser iteration limit exceeded")
			break
		}

		t := p.exp.peek()
		if t.is(tokSpace) {
			p.exp.next()
			continue
		}
		if t.isEOF() {
			if stop != stopEOF {
				p.stopAtEOF(stop, t)
			}
			break
		}
		if p.shouldStop(t, stop) {
			break
		}
		if t.is(tokEndGroup) {
			p.report(source.Diagnostic{
				Range:    normalizeRange(t.span),
				Severity: source.SeverityError,
				Code:     source.CodeUnexpectedToken,
				Message:  "unexpected '}'",
			})
			p.exp.next()
			items = append(items, astErr(t.span, "}", source.CodeUnexpectedToken))
			continue
		}

		if t.isControl() {
			if infix, ok := infixCommands[t.text]; ok {
				p.exp.next()
				if infixSeen {
					p.report(source.Diagnostic{
						Range:    normalizeRange(t.span),
						Severity: source.SeverityError,
						Code:     source.CodeUnexpectedToken,
						Message:  "only one infix fraction operator is allowed per group",
					})
					continue
				}
				items = []astNode{p.applyInfix(items, start, infix, t, stop)}
				break
			}
		}

		if t.is(tokAlignment) {
			p.report(source.Diagnostic{
				Range:    normalizeRange(t.span),
				Severity: source.SeverityError,
				Code:     source.CodeUnexpectedToken,
				Message:  "alignment tab '&' is only valid in a math environment",
			})
			p.exp.next()
			items = append(items, astErr(t.span, "&", source.CodeUnexpectedToken))
			continue
		}
		if t.isControl() && t.text == "\\\\" && stop != stopCell {
			p.report(source.Diagnostic{
				Range:    normalizeRange(t.span),
				Severity: source.SeverityError,
				Code:     source.CodeUnexpectedToken,
				Message:  "line break is only valid in a math environment",
			})
			p.exp.next()
			items = append(items, astErr(t.span, "\\\\", source.CodeUnexpectedToken))
			continue
		}

		node := p.parseAtom(stop)
		if node == nil {
			continue
		}
		if glyph, ok := node.(*astGlyphNode); ok {
			if mark, position, combining, supported := combiningAccent(glyph.text); combining {
				if !supported || len(items) == 0 {
					p.report(source.Diagnostic{
						Range:    normalizeRange(glyph.span),
						Severity: source.SeverityError,
						Code:     source.CodeUnsupported,
						Message:  "unsupported combining character",
					})
					items = append(items, astErr(glyph.span, glyph.text, source.CodeUnsupported))
					continue
				}
				body := items[len(items)-1]
				items[len(items)-1] = &astAccentNode{
					astBase: newASTBaseAt(
						astAccent,
						source.Merge(body.base().span, glyph.span),
						source.Merge(body.base().origin, glyph.origin),
					),
					body:     body,
					mark:     newGlyphAST(glyph.span, mark, ir.ClassDiacritic),
					position: position,
					dotless:  true,
				}
				continue
			}
		}
		if first {
			start = node.base().span
			first = false
		}
		items = append(items, node)
	}

	row := &astRowNode{
		astBase: newASTBase(astRow, source.Detached()),
		items:   items,
	}
	if len(items) > 0 {
		row.span = astSpan(items...)
		row.origin = astOrigin(items...)
	} else if !first {
		row.span = start
		row.origin = start
	}
	return row
}

func combiningAccent(text string) (mark string, position ir.Position, combining, supported bool) {
	r, size := utf8.DecodeRuneInString(text)
	if size != len(text) {
		return "", ir.PositionAbove, false, false
	}
	if entry, ok := combiningAccents[r]; ok {
		return entry.mark, entry.position, true, true
	}
	return "", ir.PositionAbove, r >= 0x0300 && r <= 0x036f, false
}

func (p *parser) shouldStop(t token, stop stopMode) bool {
	if stop == stopCell && t.is(tokAlignment) {
		return true
	}
	if t.is(tokEndGroup) {
		return stop == stopGroup || stop == stopRight || stop == stopBracket || stop == stopCell || stop == stopDollar
	}
	if !t.isControl() {
		if t.is(tokChar) {
			if stop == stopBracket && t.text == "]" {
				return true
			}
			if stop == stopDollar && t.text == "$" {
				return true
			}
		}
		return false
	}
	switch stop {
	case stopEndgroup:
		return t.text == "\\endgroup"
	case stopRight:
		return t.text == "\\right" || t.text == "\\middle"
	case stopParen:
		return t.text == "\\)"
	case stopCell:
		return t.is(tokAlignment) || isRowBreak(t.text) || t.text == "\\end" ||
			t.text == "\\hline" || t.text == "\\hdashline"
	default:
		return false
	}
}

func (p *parser) stopAtEOF(stop stopMode, t token) {
	switch stop {
	case stopGroup, stopBracket, stopDollar, stopParen, stopEndgroup:
		p.report(source.Diagnostic{
			Range:    normalizeRange(t.span),
			Severity: source.SeverityError,
			Code:     source.CodeUnclosedGroup,
			Message:  "unexpected end of input while parsing group",
		})
	case stopRight:
		p.report(source.Diagnostic{
			Range:    normalizeRange(t.span),
			Severity: source.SeverityError,
			Code:     source.CodeUnclosedGroup,
			Message:  "unexpected end of input before '\\right'",
		})
	case stopCell:
		p.report(source.Diagnostic{
			Range:    normalizeRange(t.span),
			Severity: source.SeverityError,
			Code:     source.CodeInvalidEnvironment,
			Message:  "unexpected end of input inside math environment",
		})
	}
}

func (p *parser) applyInfix(items []astNode, start source.Range, infix fractionInfix, command token, stop stopMode) astNode {
	left := &astRowNode{astBase: newASTBase(astRow, start), items: items}
	if len(items) > 0 {
		left.span = astSpan(items...)
		left.origin = astOrigin(items...)
	}
	if infix.dimension {
		thickness := p.parseDimension(command, p.exp.peek().is(tokBeginGroup))
		if thickness.Value != 0 {
			p.report(source.Diagnostic{
				Range:    normalizeRange(command.span),
				Severity: source.SeverityWarning,
				Code:     source.CodeUnsupported,
				Message:  "custom \\above rule thickness is not represented in the neutral IR",
			})
		}
	}
	right := p.parseSequenceWithInfix(stop, true)
	fraction := &astFractionNode{
		astBase:     newASTBaseAt(astFraction, source.Merge(command.span, right.base().span), tokenOrigin(command)),
		numerator:   left,
		denominator: right,
		bar:         infix.bar,
		style:       infix.style,
		role:        infix.role,
		size:        ir.SizeInherit,
		infix:       true,
	}
	if infix.open != "" || infix.close != "" {
		return &astFencedNode{
			astBase:  newASTBase(astFenced, fraction.span),
			open:     newGlyphAST(command.span, infix.open, ir.ClassOpening),
			close:    newGlyphAST(command.span, infix.close, ir.ClassClosing),
			body:     []astNode{fraction},
			balanced: true,
		}
	}
	return fraction
}

func (p *parser) parseAtom(stop stopMode) astNode {
	if p.depth >= p.limits.MaxNesting {
		t := p.exp.peek()
		p.limit(normalizeRange(t.span), "TeX parser nesting limit exceeded")
		return p.recoverAtom(t)
	}

	base := p.parseNucleus(stop)
	if base == nil {
		return nil
	}

	var top astNode
	var bottom astNode
	var limits ir.LimitsPlacement
	var scriptSpan source.Range
	hasScriptSpan := false
	setScriptSpan := func(span source.Range) {
		if !hasScriptSpan {
			scriptSpan = span
			hasScriptSpan = true
		} else {
			scriptSpan = source.Merge(scriptSpan, span)
		}
	}

	for {
		p.exp.consumeSpaces()
		t := p.exp.peek()
		switch {
		case !t.isControl() && t.is(tokSuperscript):
			p.exp.next()
			if top != nil {
				p.report(source.Diagnostic{
					Range:    normalizeRange(t.span),
					Severity: source.SeverityError,
					Code:     source.CodeUnexpectedToken,
					Message:  "double superscript",
				})
			}
			arg, span := p.parseScriptArgument("superscript", t)
			if top == nil {
				top = arg
			} else {
				span := source.Merge(top.base().span, arg.base().span)
				top = &astRowNode{astBase: newASTBaseAt(astRow, span, source.Merge(top.base().origin, arg.base().origin)), items: []astNode{top, arg}}
			}
			setScriptSpan(span)
		case !t.isControl() && t.is(tokSubscript):
			p.exp.next()
			if bottom != nil {
				p.report(source.Diagnostic{
					Range:    normalizeRange(t.span),
					Severity: source.SeverityError,
					Code:     source.CodeUnexpectedToken,
					Message:  "double subscript",
				})
			}
			arg, span := p.parseScriptArgument("subscript", t)
			if bottom == nil {
				bottom = arg
			} else {
				span := source.Merge(bottom.base().span, arg.base().span)
				bottom = &astRowNode{astBase: newASTBaseAt(astRow, span, source.Merge(bottom.base().origin, arg.base().origin)), items: []astNode{bottom, arg}}
			}
			setScriptSpan(span)
		case !t.isControl() && t.is(tokChar) && (t.text == "'" || t.text == "′"):
			p.exp.next()
			prime := newGlyphASTAt(t.span, tokenOrigin(t), "′", ir.ClassNormal)
			if top == nil {
				top = &astRowNode{astBase: newASTBaseAt(astRow, t.span, tokenOrigin(t)), items: []astNode{prime}}
			} else if row, ok := top.(*astRowNode); ok && isPrimeRow(row) {
				row.items = append(row.items, prime)
				row.span = source.Merge(row.span, t.span)
				row.origin = source.Merge(row.origin, tokenOrigin(t))
			} else {
				p.report(source.Diagnostic{
					Range:    normalizeRange(t.span),
					Severity: source.SeverityError,
					Code:     source.CodeUnexpectedToken,
					Message:  "prime conflicts with an existing superscript",
				})
			}
			setScriptSpan(t.span)
		case t.isControl() && (t.text == "\\limits" || t.text == "\\nolimits"):
			p.exp.next()
			operatorBase := isLimitBase(base)
			if !operatorBase {
				p.report(source.Diagnostic{
					Range:    normalizeRange(t.span),
					Severity: source.SeverityError,
					Code:     source.CodeUnexpectedToken,
					Message:  "limit controls must follow a math operator",
				})
				continue
			}
			if t.text == "\\limits" {
				limits = ir.LimitsAlways
			} else {
				limits = ir.LimitsNever
			}
			switch n := base.(type) {
			case *astOperatorNode:
				n.limits = limits
			case *astGlyphNode:
				n.limits = limits
			case *astClassNode:
				n.limits = limits
				n.setLimits = true
			}
		default:
			if top == nil && bottom == nil {
				return base
			}
			span := astSpan(base)
			if hasScriptSpan {
				span = source.Merge(span, scriptSpan)
			}
			scripts := &astScriptsNode{
				astBase: newASTBase(astScripts, span),
				main:    base,
				top:     top,
				bottom:  bottom,
				limits:  scriptLimits(base, limits),
				role:    scriptSemanticRole(base),
			}
			return scripts
		}
	}
}

func isLimitBase(node astNode) bool {
	switch n := node.(type) {
	case *astOperatorNode:
		return true
	case *astGlyphNode:
		return n.class == ir.ClassLarge
	case *astClassNode:
		return n.class == ir.ClassLarge
	default:
		return false
	}
}

func scriptSemanticRole(node astNode) ir.ScriptRole {
	if hasBraceDecoration(node) {
		return ir.ScriptRoleBraceLabel
	}
	if hasLimitScriptBase(node) {
		return ir.ScriptRoleLimits
	}
	return ir.ScriptRoleOrdinary
}

func hasBraceDecoration(node astNode) bool {
	switch n := node.(type) {
	case *astClassNode:
		return hasBraceDecoration(n.body)
	case *astVariantNode:
		return hasBraceDecoration(n.body)
	case *astAccentNode:
		glyph, ok := n.mark.(*astGlyphNode)
		return ok && strings.ContainsAny(glyph.text, "⏞⏟⎴⎵⏜⏝⏠⏡")
	default:
		return false
	}
}

func hasLimitScriptBase(node astNode) bool {
	switch n := node.(type) {
	case *astClassNode:
		return n.class == ir.ClassLarge &&
			(n.limits != ir.LimitsNever || containsIntegral(n.body))
	case *astVariantNode:
		return hasLimitScriptBase(n.body)
	case *astOperatorNode:
		return n.limits != ir.LimitsNever || containsIntegralText(n.text)
	case *astGlyphNode:
		return n.class == ir.ClassLarge &&
			(n.limits != ir.LimitsNever || containsIntegralText(n.text))
	default:
		return false
	}
}

func containsIntegral(node astNode) bool {
	switch n := node.(type) {
	case *astGlyphNode:
		return containsIntegralText(n.text)
	case *astOperatorNode:
		return containsIntegralText(n.text)
	case *astRowNode:
		for _, item := range n.items {
			if containsIntegral(item) {
				return true
			}
		}
	}
	return false
}

func containsIntegralText(text string) bool {
	return strings.ContainsAny(text, "∫∬∭∮∯∰")
}

func scriptLimits(base astNode, explicit ir.LimitsPlacement) ir.LimitsPlacement {
	if explicit != ir.LimitsAuto {
		return explicit
	}
	switch n := base.(type) {
	case *astOperatorNode:
		return n.limits
	case *astGlyphNode:
		return n.limits
	case *astClassNode:
		return n.limits
	default:
		return ir.LimitsAuto
	}
}

func (p *parser) parseNucleus(stop stopMode) astNode {
	t := p.exp.next()
	if t.isEOF() {
		return nil
	}
	switch t.kind {
	case tokBeginGroup:
		return p.parseGroup(t)
	case tokChar:
		if t.text == "~" {
			return &astSpaceNode{
				astBase: newASTBase(astSpace, t.span),
				kind:    ir.SpaceInterword,
			}
		}
		if t.text == "$" {
			p.report(source.Diagnostic{
				Range:    normalizeRange(t.span),
				Severity: source.SeverityError,
				Code:     source.CodeUnsupported,
				Message:  "nested math delimiters are not supported in math mode",
			})
			return astErr(t.span, t.text, source.CodeUnsupported)
		}
		spec := symbolForChar(t.text)
		return astGlyphSpec(t.span, t.origin, spec)
	case tokEndGroup, tokAlignment, tokParameter:
		p.report(source.Diagnostic{
			Range:    normalizeRange(t.span),
			Severity: source.SeverityError,
			Code:     source.CodeUnexpectedToken,
			Message:  "unexpected token '" + t.text + "'",
		})
		return astErr(t.span, t.text, source.CodeUnexpectedToken)
	case tokSuperscript, tokSubscript:
		p.exp.push(t)
		return &astRowNode{astBase: newASTBase(astRow, source.Detached())}
	case tokControlWord, tokControlSymbol:
		return p.parseCommand(t, stop)
	default:
		return &astRowNode{astBase: newASTBase(astRow, source.Detached())}
	}
}

func (p *parser) parseDollarMath(open token) astNode {
	body := p.parseSequence(stopDollar)
	span := source.Merge(open.span, body.base().span)
	if close := p.exp.peek(); close.is(tokChar) && close.text == "$" {
		p.exp.next()
		span = source.Merge(span, close.span)
	} else {
		p.report(source.Diagnostic{
			Range:    normalizeRange(open.span),
			Severity: source.SeverityError,
			Code:     source.CodeUnclosedGroup,
			Message:  "unclosed '$' math delimiter",
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

func (p *parser) parseGroup(open token) astNode {
	p.exp.beginScope()
	defer p.exp.endScope()

	body := p.parseSequence(stopGroup)
	close := p.exp.peek()
	span := open.span
	if close.is(tokEndGroup) {
		p.exp.next()
		span = source.Merge(open.span, close.span)
	} else {
		p.report(source.Diagnostic{
			Range:    normalizeRange(span),
			Severity: source.SeverityError,
			Code:     source.CodeUnclosedGroup,
			Message:  "unclosed group; expected '}'",
		})
		span = source.Merge(span, body.base().span)
	}
	body.base().span = span
	body.base().origin = containerOrigin(open, body, span)
	return body
}

func (p *parser) parseScriptArgument(name string, marker token) (astNode, source.Range) {
	p.exp.consumeSpaces()
	t := p.exp.peek()
	if t.isEOF() || t.is(tokEndGroup) || t.isControl() && (t.text == "\\right" || t.text == "\\middle" || t.text == "\\end") {
		p.report(source.Diagnostic{
			Range:    normalizeRange(marker.span),
			Severity: source.SeverityError,
			Code:     source.CodeMissingArgument,
			Message:  "expected group after '" + name + "'",
		})
		span := normalizeRange(marker.span)
		return newPlaceholderAST(span, "missing "+name), source.Merge(marker.span, t.span)
	}
	if t.isControl() && commandTakesRequiredArgument(t.text) {
		p.report(source.Diagnostic{
			Range:    normalizeRange(t.span),
			Severity: source.SeverityError,
			Code:     source.CodeMissingArgument,
			Message:  "function requires a braced argument in script position",
		})
	}
	node := p.parseNucleus(stopEOF)
	if node == nil {
		span := normalizeRange(marker.span)
		return newPlaceholderAST(span, "missing "+name), span
	}
	return node, source.Merge(marker.span, node.base().span)
}

func commandTakesRequiredArgument(command string) bool {
	switch command {
	case "\\sqrt", "\\frac", "\\dfrac", "\\tfrac", "\\cfrac", "\\binom",
		"\\dbinom", "\\tbinom", "\\operatorname", "\\operatornamewithlimits":
		return true
	default:
		return false
	}
}

func (p *parser) parseRequiredArgument(owner token) astNode {
	p.exp.consumeSpaces()
	t := p.exp.peek()
	if t.isEOF() || t.is(tokEndGroup) || t.is(tokAlignment) || t.isControl() && (t.text == "\\right" || t.text == "\\middle" || t.text == "\\end") {
		p.report(source.Diagnostic{
			Range:    normalizeRange(owner.span),
			Severity: source.SeverityError,
			Code:     source.CodeMissingArgument,
			Message:  "missing required argument to '" + owner.text + "'",
		})
		span := normalizeRange(owner.span)
		return newPlaceholderAST(span, "missing argument to "+owner.text)
	}
	if t.is(tokBeginGroup) {
		p.exp.next()
		return p.parseGroup(t)
	}
	node := p.parseNucleus(stopEOF)
	if node == nil {
		return newPlaceholderAST(normalizeRange(owner.span), "missing argument to "+owner.text)
	}
	return node
}

func (p *parser) parseOptionalArgument(owner token) astNode {
	p.exp.consumeSpaces()
	t := p.exp.peek()
	if !t.is(tokChar) || t.text != "[" {
		return nil
	}
	p.exp.next()
	p.exp.beginScope()
	defer p.exp.endScope()
	body := p.parseSequence(stopBracket)
	close := p.exp.peek()
	span := source.Merge(t.span, body.base().span)
	if close.is(tokChar) && close.text == "]" {
		p.exp.next()
		span = source.Merge(t.span, close.span)
	} else {
		p.report(source.Diagnostic{
			Range:    normalizeRange(owner.span),
			Severity: source.SeverityError,
			Code:     source.CodeUnclosedGroup,
			Message:  "unclosed optional argument to '" + owner.text + "'",
		})
	}
	body.base().span = span
	body.base().origin = containerOrigin(t, body, span)
	return body
}

func (p *parser) recoverAtom(t token) astNode {
	if t.isEOF() {
		return nil
	}
	p.exp.next()
	if t.is(tokBeginGroup) {
		depth := 1
		for depth > 0 {
			next := p.exp.popRaw()
			if next.isEOF() {
				break
			}
			if next.is(tokBeginGroup) {
				depth++
			} else if next.is(tokEndGroup) {
				depth--
			}
		}
	}
	return newPlaceholderAST(normalizeRange(t.span), "nesting limit exceeded")
}

func (p *parser) report(d source.Diagnostic) {
	p.diagnostics = append(p.diagnostics, d)
}

func (p *parser) limit(span source.Range, message string) {
	if p.limitHit {
		return
	}
	p.limitHit = true
	p.report(source.Diagnostic{
		Range:    span,
		Severity: source.SeverityError,
		Code:     source.CodeLimitExceeded,
		Message:  message,
	})
}

func (p *parser) parseDimension(t token, braced bool) ir.Length {
	text := ""
	span := t.span
	if braced {
		text = p.consumeRawGroupText()
	} else {
		for {
			next := p.exp.popRaw()
			if next.is(tokSpace) || next.isEOF() {
				p.exp.push(next)
				break
			}
			if next.isControl() {
				if text != "" && (next.text == "\\relax" || next.text == "\\ ") {
					p.exp.push(next)
					break
				}
				p.exp.push(next)
				break
			}
			text += next.text
			span = source.Merge(span, next.span)
			if hasExplicitLengthUnit(text) {
				if _, ok := parseLengthString(strings.TrimSpace(text)); ok {
					break
				}
			}
		}
	}
	length, ok := parseLengthString(strings.TrimSpace(text))
	if !ok {
		p.report(source.Diagnostic{
			Range:    normalizeRange(span),
			Severity: source.SeverityError,
			Code:     source.CodeInvalidValue,
			Message:  "invalid TeX dimension '" + text + "'",
		})
		return ir.Length{}
	}
	if math.Abs(float64(length.Value)) > float64(p.limits.MaxDimension)*1_000_000 {
		p.report(source.Diagnostic{
			Range:    normalizeRange(span),
			Severity: source.SeverityError,
			Code:     source.CodeLimitExceeded,
			Message:  "TeX dimension exceeds configured limit",
		})
		return ir.Length{}
	}
	return length
}

func hasExplicitLengthUnit(text string) bool {
	for _, unit := range []string{"em", "ex", "mu", "pt", "dip"} {
		if strings.HasSuffix(strings.TrimSpace(text), unit) {
			return true
		}
	}
	return false
}

func parseLengthString(text string) (ir.Length, bool) {
	if len(text) < 2 {
		return ir.Length{}, false
	}
	unit := ""
	valueText := ""
	for _, candidate := range []string{"em", "ex", "mu", "pt", "dip"} {
		if strings.HasSuffix(text, candidate) {
			unit = candidate
			valueText = strings.TrimSpace(strings.TrimSuffix(text, candidate))
			break
		}
	}
	if unit == "" {
		unit = "pt"
		valueText = strings.TrimSpace(text)
	}
	value, err := strconv.ParseFloat(valueText, 64)
	if err != nil || math.IsNaN(value) || math.IsInf(value, 0) {
		return ir.Length{}, false
	}
	if math.Abs(value) > float64(math.MaxInt64)/1_000_000 {
		return ir.Length{}, false
	}
	var length ir.Length
	switch unit {
	case "em":
		length, err = ir.EmLength(value)
	case "ex":
		length, err = ir.ExLength(value)
	case "mu":
		length, err = ir.MuLength(value)
	case "pt":
		length, err = ir.PtLength(value)
	case "dip":
		length, err = ir.DipLength(value)
	default:
		return ir.Length{}, false
	}
	if err != nil {
		return ir.Length{}, false
	}
	return length, true
}

func (p *parser) consumeRawGroupText() string {
	p.exp.consumeSpaces()
	open := p.exp.popRaw()
	if !open.is(tokBeginGroup) {
		p.exp.push(open)
		return ""
	}
	var b strings.Builder
	depth := 1
	for {
		t := p.exp.popRaw()
		if t.isEOF() {
			break
		}
		if t.is(tokBeginGroup) {
			depth++
			if depth > 1 {
				b.WriteString("{")
			}
			continue
		}
		if t.is(tokEndGroup) {
			depth--
			if depth == 0 {
				break
			}
			b.WriteString("}")
			continue
		}
		b.WriteString(t.text)
	}
	return b.String()
}

func newGlyphAST(span source.Range, text string, class ir.MathClass) *astGlyphNode {
	return newGlyphASTAt(span, span, text, class)
}

func newGlyphASTAt(span, origin source.Range, text string, class ir.MathClass) *astGlyphNode {
	return &astGlyphNode{
		astBase: newASTBaseAt(astGlyph, span, origin),
		text:    text,
		class:   class,
		variant: ir.VariantAuto,
	}
}

func astGlyphSpec(span, origin source.Range, spec symbolSpec) *astGlyphNode {
	b := newASTBase(astGlyph, span)
	b.origin = origin
	return &astGlyphNode{
		astBase: b,
		text:    spec.text,
		class:   spec.class,
		variant: spec.variant,
		limits:  spec.limits,
	}
}

func newPlaceholderAST(span source.Range, reason string) *astPlaceholderNode {
	return &astPlaceholderNode{
		astBase: newASTBase(astPlaceholder, normalizeRange(span)),
		reason:  reason,
	}
}

func astErr(span source.Range, text string, code source.Code) *astErrorNode {
	if text == "" {
		text = "error"
	}
	return &astErrorNode{
		astBase: newASTBase(astError, normalizeRange(span)),
		text:    text,
		code:    code,
	}
}

func isPrimeRow(row *astRowNode) bool {
	if len(row.items) == 0 {
		return false
	}
	for _, item := range row.items {
		glyph, ok := item.(*astGlyphNode)
		if !ok || glyph.text != "′" {
			return false
		}
	}
	return true
}
