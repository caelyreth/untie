package typst

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/caelyreth/untie/ir"
	"github.com/caelyreth/untie/source"
)

const noAttach tokenKind = 0xff

type stopSet uint8

const (
	stopNone  stopSet = 0
	stopComma stopSet = 1 << iota
	stopSemicolon
	stopRParen
	stopRBracket
	stopRBrace
	stopDollar
)

type parser struct {
	src    string
	mode   ir.Mode
	limits source.Limits
	tokens []token
	pos    int
	diags  []source.Diagnostic

	nodes int
	depth int
}

func parseSource(src string, mode ir.Mode, limits source.Limits) (*astNode, []source.Diagnostic) {
	limits = limits.WithDefaults()
	if len(src) > limits.MaxSourceBytes {
		span := source.NewRange(0, len(src))
		row := newAST(astRow, span)
		return row, []source.Diagnostic{{
			Range:    span,
			Severity: source.SeverityError,
			Code:     source.CodeLimitExceeded,
			Message:  "source exceeds configured byte limit",
		}}
	}
	tokens, lexDiags := lex(src, limits)
	p := parser{
		src:    src,
		mode:   mode,
		limits: limits,
		tokens: tokens,
		diags:  lexDiags,
	}
	root := p.parseRow(stopNone)
	if p.current().kind != tokEOF {
		p.errorAt(p.current().span, source.CodeSyntax, "unexpected %s", p.current().describe())
	}
	return root, source.NormalizeDiagnostics(p.diags)
}

func (p *parser) current() token {
	if p.pos >= len(p.tokens) {
		return token{kind: tokEOF, span: source.NewRange(len(p.src), len(p.src))}
	}
	return p.tokens[p.pos]
}

func (p *parser) peek(offset int) token {
	i := p.pos + offset
	if i < 0 || i >= len(p.tokens) {
		return token{kind: tokEOF, span: source.NewRange(len(p.src), len(p.src))}
	}
	return p.tokens[i]
}

func (p *parser) advance() token {
	tok := p.current()
	if tok.kind != tokEOF {
		p.pos++
	}
	return tok
}

func (p *parser) match(kind tokenKind) bool {
	if p.current().kind != kind {
		return false
	}
	p.advance()
	return true
}

func (p *parser) enter(span source.Range) bool {
	if p.depth >= p.limits.MaxNesting {
		p.diags = append(p.diags, source.Diagnostic{
			Range:    span,
			Severity: source.SeverityError,
			Code:     source.CodeLimitExceeded,
			Message:  "parser nesting limit exceeded",
		})
		return false
	}
	p.depth++
	return true
}

func (p *parser) leave() {
	p.depth--
}

func (p *parser) node(span source.Range, kind astKind) *astNode {
	p.nodes++
	if p.nodes > p.limits.MaxNodes {
		if p.nodes == p.limits.MaxNodes+1 {
			p.diags = append(p.diags, source.Diagnostic{
				Range:    span,
				Severity: source.SeverityError,
				Code:     source.CodeLimitExceeded,
				Message:  "node limit exceeded",
			})
		}
		span = source.NewRange(span.Start, min(span.Start+1, span.End))
		kind = astError
	}
	return newAST(kind, span)
}

func (p *parser) errorAt(span source.Range, code source.Code, format string, args ...any) {
	p.diags = append(p.diags, source.Diagnostic{
		Range:    span,
		Severity: source.SeverityError,
		Code:     code,
		Message:  fmt.Sprintf(format, args...),
	})
}

func (p *parser) atStop(stops stopSet) bool {
	tok := p.current()
	switch tok.kind {
	case tokEOF:
		return true
	case tokComma:
		return stops&stopComma != 0
	case tokSemicolon:
		return stops&stopSemicolon != 0
	case tokRParen:
		return stops&stopRParen != 0
	case tokRBracket:
		return stops&stopRBracket != 0
	case tokRBrace:
		return stops&stopRBrace != 0
	case tokDollar:
		return stops&stopDollar != 0
	default:
		return false
	}
}

func (p *parser) parseRow(stops stopSet) *astNode {
	start := p.current().span.Start
	if !p.enter(source.NewRange(start, start)) {
		return p.node(source.NewRange(start, start), astError)
	}
	defer p.leave()

	items := make([]*astNode, 0, 4)
	for !p.atStop(stops) {
		tok := p.current()
		if tok.kind == tokLinebreak {
			p.advance()
			items = append(items, p.node(tok.span, astLinebreak))
			continue
		}
		if tok.kind == tokAmp {
			p.advance()
			items = append(items, p.node(tok.span, astAlign))
			continue
		}
		before := p.pos
		expr := p.parseExpr(0, noAttach, stopNone)
		if expr == nil {
			if p.pos == before {
				p.advance()
			}
			bad := p.node(tok.span, astError)
			bad.raw = tok.text
			p.errorAt(tok.span, source.CodeUnexpectedToken, "unexpected %s", tok.describe())
			items = append(items, bad)
			continue
		}
		items = append(items, expr)
	}
	end := start
	if len(items) > 0 {
		end = items[len(items)-1].span.End
	}
	row := p.node(source.NewRange(start, end), astRow)
	row.items = items
	return row
}

// parseExpr is a Pratt parser for Typst math. Attachment precedence is two and
// fractions are one, matching Typst 0.15. stopAttach keeps the opposite side of
// an attachment chain at the outer level, so x^a_b yields one attachment node
// with both a top and bottom script.
func (p *parser) parseExpr(minPrec int, stopAttach tokenKind, stops stopSet) *astNode {
	if !p.enter(p.current().span) {
		return nil
	}
	defer p.leave()

	left := p.parsePrimary()
	if left == nil {
		return nil
	}

	for !p.atStop(stops) {
		tok := p.current()
		if tok.kind == stopAttach {
			return left
		}
		switch tok.kind {
		case tokSlash:
			const prec = 1
			if prec < minPrec {
				return left
			}
			p.advance()
			right := p.parseExpr(prec+1, noAttach, stops)
			if right == nil {
				p.errorAt(tok.span, source.CodeMissingArgument, "missing denominator")
				right = p.node(source.NewRange(tok.span.End, tok.span.End), astError)
				right.raw = "missing denominator"
			}
			right = unparenthesize(right)
			fr := p.node(source.Merge(left.span, right.span), astFraction)
			fr.items = []*astNode{left, right}
			left = fr

		case tokCaret, tokUnderscore:
			const prec = 2
			if prec < minPrec {
				return left
			}
			opposite := tokUnderscore
			if tok.kind == tokUnderscore {
				opposite = tokCaret
			}
			p.advance()
			right := p.parseExpr(prec, opposite, stops)
			if right == nil {
				p.errorAt(tok.span, source.CodeMissingArgument, "expected expression after %s", tok.text)
				right = p.node(source.NewRange(tok.span.End, tok.span.End), astError)
				right.raw = "missing attachment"
			}
			right = unparenthesize(right)
			if tok.kind == tokCaret {
				left = p.addAttachment(left, attachmentTop, right)
			} else {
				left = p.addAttachment(left, attachmentBottom, right)
			}

		case tokPrime:
			const prec = 2
			if prec < minPrec || tok.spaceBefore || stopAttach == tokPrime {
				return left
			}
			p.advance()
			prime := p.node(tok.span, astGlyph)
			prime.text = strings.Repeat("′", len(tok.text))
			prime.raw = "primes"
			left = p.addAttachment(left, attachmentTopRight, prime)

		case tokBang:
			const prec = 3
			if prec < minPrec || tok.spaceBefore {
				return left
			}
			p.advance()
			bang := p.node(tok.span, astGlyph)
			bang.text = "!"
			row := p.node(source.Merge(left.span, bang.span), astRow)
			row.items = []*astNode{left, bang}
			left = row

		default:
			return left
		}
	}
	return left
}

type attachmentSlot uint8

const (
	attachmentTop attachmentSlot = iota
	attachmentBottom
	attachmentTopLeft
	attachmentBottomLeft
	attachmentTopRight
	attachmentBottomRight
)

func (p *parser) addAttachment(base *astNode, slot attachmentSlot, child *astNode) *astNode {
	if base.kind == astAttach && !attachmentOccupied(base, slot) {
		switch slot {
		case attachmentTop:
			base.top = child
		case attachmentBottom:
			base.bottom = child
		case attachmentTopLeft:
			base.topLeft = child
		case attachmentBottomLeft:
			base.bottomLeft = child
		case attachmentTopRight:
			base.topRight = child
		case attachmentBottomRight:
			base.bottomRight = child
		}
		base.span = source.Merge(base.span, child.span)
		return base
	}
	att := p.node(source.Merge(base.span, child.span), astAttach)
	att.base = base
	switch slot {
	case attachmentTop:
		att.top = child
	case attachmentBottom:
		att.bottom = child
	case attachmentTopLeft:
		att.topLeft = child
	case attachmentBottomLeft:
		att.bottomLeft = child
	case attachmentTopRight:
		att.topRight = child
	case attachmentBottomRight:
		att.bottomRight = child
	}
	return att
}

func attachmentOccupied(n *astNode, slot attachmentSlot) bool {
	switch slot {
	case attachmentTop:
		return n.top != nil
	case attachmentBottom:
		return n.bottom != nil
	case attachmentTopLeft:
		return n.topLeft != nil
	case attachmentBottomLeft:
		return n.bottomLeft != nil
	case attachmentTopRight:
		return n.topRight != nil
	case attachmentBottomRight:
		return n.bottomRight != nil
	default:
		return true
	}
}

func unparenthesize(n *astNode) *astNode {
	if n != nil && n.kind == astDelimited && n.open == "(" && n.close == ")" && len(n.items) == 1 {
		return n.items[0]
	}
	return n
}

func (p *parser) parsePrimary() *astNode {
	tok := p.current()
	switch tok.kind {
	case tokEOF, tokRParen, tokRBracket, tokRBrace:
		return nil
	case tokLinebreak, tokAmp:
		return nil
	case tokIdent:
		callee := p.parseIdentOrField()
		if p.current().kind == tokLParen && !p.current().spaceBefore {
			return p.parseCall(callee)
		}
		return callee
	case tokNumber:
		p.advance()
		lit := p.node(tok.span, astNumber)
		lit.text = tok.text
		if v, ok := parseNumericLiteral(tok.text); ok {
			lit.value = v
		}
		return lit
	case tokString:
		p.advance()
		str := p.node(tok.span, astString)
		str.text = tok.literal
		str.raw = tok.text
		return str
	case tokShorthand:
		p.advance()
		short := p.node(tok.span, astShorthand)
		short.text = tok.literal
		return short
	case tokHash:
		return p.parseCode()
	case tokLParen, tokLBracket, tokLBrace:
		return p.parseDelimited()
	case tokDollar:
		p.advance()
		body := p.parseRow(stopDollar)
		end := p.current().span.End
		if !p.match(tokDollar) {
			p.errorAt(tok.span, source.CodeUnclosedGroup, "unclosed equation")
		}
		eq := p.node(source.NewRange(tok.span.Start, end), astEquation)
		eq.items = []*astNode{body}
		return eq
	}

	text := tok.text
	if tok.literal != "" {
		text = tok.literal
	}
	if text == "√" || text == "∛" || text == "∜" {
		p.advance()
		root := p.node(tok.span, astRoot)
		radicand := p.parseExpr(2, noAttach, stopNone)
		if radicand == nil {
			p.errorAt(tok.span, source.CodeMissingArgument, "missing radicand")
			radicand = p.node(source.NewRange(tok.span.End, tok.span.End), astError)
			radicand.raw = "missing radicand"
		}
		radicand = unparenthesize(radicand)
		root.items = []*astNode{radicand}
		if text != "√" {
			index := p.node(tok.span, astGlyph)
			index.text = strings.TrimPrefix(text, "∛")
			if text == "∛" {
				index.text = "3"
			} else {
				index.text = "4"
			}
			root.items = append(root.items, index)
		}
		root.span = source.Merge(root.span, radicand.span)
		return root
	}
	if text != "" {
		p.advance()
		glyph := p.node(tok.span, astGlyph)
		glyph.text = text
		return glyph
	}
	return nil
}

func (p *parser) parseIdentOrField() *astNode {
	first := p.advance()
	node := p.node(first.span, astIdentifier)
	node.text = first.text
	for p.current().kind == tokGlyph && p.current().text == "." {
		dot := p.current()
		next := p.peek(1)
		if next.kind != tokIdent || next.spaceBefore {
			break
		}
		p.advance()
		part := p.advance()
		node.text += "." + part.text
		node.span = source.Merge(node.span, part.span)
		_ = dot
	}
	if strings.Contains(node.text, ".") {
		node.kind = astField
	}
	return node
}

func (p *parser) parseCall(callee *astNode) *astNode {
	open := p.advance() // left paren
	call := p.node(open.span, astCall)
	call.base = callee

	seenNamed := make(map[string]source.Range)
	for p.current().kind != tokEOF && p.current().kind != tokRParen {
		before := p.pos
		arg := astArg{span: p.current().span}
		named, nameLen := p.namedArgName()
		switch {
		case p.current().kind == tokSpread:
			spread := p.advance()
			arg.kind = argSpread
			arg.span = spread.span
			arg.node = p.parseRow(stopComma | stopSemicolon | stopRParen)
		case named != "":
			name := p.advance()
			for i := 1; i < nameLen; i++ {
				p.advance()
			}
			p.advance() // colon
			arg.kind = argNamed
			arg.name = named
			arg.span = name.span
			if prior, duplicate := seenNamed[named]; duplicate {
				p.errorAt(name.span, source.CodeSyntax, "duplicate argument: %s", named)
				_ = prior
			} else {
				seenNamed[named] = name.span
			}
			arg.node = p.parseRow(stopComma | stopSemicolon | stopRParen)
			if len(arg.node.items) == 0 {
				p.errorAt(name.span, source.CodeMissingArgument, "expected expression after %s:", named)
			}
		default:
			arg.node = p.parseRow(stopComma | stopSemicolon | stopRParen)
		}
		if p.pos == before {
			for !p.atStop(stopComma | stopSemicolon | stopRParen) {
				p.advance()
			}
		}

		if arg.node != nil && len(arg.node.items) > 0 {
			arg.span = source.Merge(arg.span, arg.node.span)
		}
		if p.current().kind == tokSemicolon {
			p.advance()
			arg.semicolonAfter = true
			arg.span = source.Merge(arg.span, p.peek(-1).span)
		} else if p.current().kind == tokComma {
			p.advance()
		}
		call.args = append(call.args, arg)
		if p.current().kind == tokRParen || p.current().kind == tokEOF {
			break
		}
	}

	end := p.current().span.End
	if p.current().kind == tokRParen {
		p.advance()
	} else {
		p.errorAt(open.span, source.CodeUnclosedGroup, "unclosed function arguments")
	}
	call.span = source.NewRange(callee.span.Start, end)

	// Typst allows a trailing content block: text(fill: red)[body].
	for p.current().kind == tokLBracket && !p.current().spaceBefore {
		content := p.parseDelimited()
		if content != nil {
			content.open = ""
			content.close = ""
			content.raw = "content"
			call.args = append(call.args, astArg{
				kind: argPositional,
				span: content.span,
				node: content,
			})
			call.span = source.Merge(call.span, content.span)
		}
	}
	return call
}

// namedArgName recognizes code-style named arguments, whose identifiers may
// contain hyphens (row-gap, column-gap), while math identifiers may not. It
// returns the number of tokens before the colon.
func (p *parser) namedArgName() (string, int) {
	if p.current().kind != tokIdent {
		return "", 0
	}
	var b strings.Builder
	b.WriteString(p.current().text)
	i := 1
	for {
		tok := p.peek(i)
		if tok.kind == tokColon {
			if tok.spaceBefore {
				return "", 0
			}
			return b.String(), i
		}
		if tok.kind != tokShorthand || tok.literal != "−" {
			return "", 0
		}
		next := p.peek(i + 1)
		if next.kind != tokIdent || next.spaceBefore {
			return "", 0
		}
		b.WriteByte('-')
		b.WriteString(next.text)
		i += 2
	}
}

func (p *parser) parseDelimited() *astNode {
	open := p.advance()
	var closeKind tokenKind
	var openText, closeText string
	switch open.kind {
	case tokLParen:
		closeKind, openText, closeText = tokRParen, "(", ")"
	case tokLBracket:
		closeKind, openText, closeText = tokRBracket, "[", "]"
	case tokLBrace:
		closeKind, openText, closeText = tokRBrace, "{", "}"
	default:
		return nil
	}
	var stops stopSet
	switch closeKind {
	case tokRParen:
		stops = stopRParen
	case tokRBracket:
		stops = stopRBracket
	case tokRBrace:
		stops = stopRBrace
	}
	body := p.parseRow(stops)
	end := p.current().span.End
	if p.current().kind == closeKind {
		p.advance()
	} else {
		p.errorAt(open.span, source.CodeUnclosedGroup, "unclosed delimiter")
		end = p.current().span.Start
	}
	delim := p.node(source.NewRange(open.span.Start, end), astDelimited)
	delim.open = openText
	delim.close = closeText
	delim.items = []*astNode{body}
	return delim
}

func (p *parser) parseCode() *astNode {
	hash := p.advance()
	inner := p.parseCodeAtom()
	if inner == nil {
		p.errorAt(hash.span, source.CodeMissingArgument, "expected expression after #")
		inner = p.node(source.NewRange(hash.span.End, hash.span.End), astError)
		inner.raw = "#"
	}
	code := p.node(source.Merge(hash.span, inner.span), astCode)
	code.items = []*astNode{inner}
	return code
}

func (p *parser) parseCodeAtom() *astNode {
	tok := p.current()
	switch tok.kind {
	case tokIdent:
		ident := p.parseIdentOrField()
		switch ident.text {
		case "let", "set", "show", "for", "while", "import", "include", "if", "else", "context", "return", "break", "continue":
			p.errorAt(ident.span, source.CodeUnsupported, "%s expressions are not supported in Typst math", ident.text)
			bad := p.node(ident.span, astError)
			bad.raw = ident.text
			return bad
		case "none":
			lit := p.node(ident.span, astLiteral)
			lit.value = noneValue()
			return lit
		case "auto":
			lit := p.node(ident.span, astLiteral)
			lit.value = autoValue()
			return lit
		case "true", "false":
			lit := p.node(ident.span, astLiteral)
			lit.value = boolValue(ident.text == "true")
			return lit
		}
		if p.current().kind == tokLParen && !p.current().spaceBefore {
			return p.parseCall(ident)
		}
		return ident
	case tokNumber:
		p.advance()
		lit := p.node(tok.span, astLiteral)
		if p.current().kind == tokIdent && !p.current().spaceBefore {
			unit := p.current()
			if length, ok := lengthFromParts(tok.text, unit.text); ok {
				p.advance()
				lit.span = source.Merge(lit.span, unit.span)
				lit.text = tok.text + unit.text
				lit.value = lengthValue(length)
				return lit
			}
		}
		if p.current().kind == tokGlyph && p.current().text == "%" && !p.current().spaceBefore {
			unit := p.advance()
			lit.span = source.Merge(lit.span, unit.span)
			lit.text = tok.text + "%"
			value, err := strconv.ParseFloat(tok.text, 64)
			if err != nil {
				p.errorAt(tok.span, source.CodeInvalidValue, "invalid percentage")
			}
			length, err := ir.PercentLength(value)
			if err != nil {
				p.errorAt(tok.span, source.CodeInvalidValue, "invalid percentage")
				length = ir.Length{}
			}
			lit.value = lengthValue(length)
			return lit
		}
		if v, ok := parseNumericLiteral(tok.text); ok {
			lit.value = v
			lit.text = tok.text
			return lit
		}
		p.errorAt(tok.span, source.CodeInvalidValue, "invalid number %q", tok.text)
		lit.kind = astError
		lit.raw = tok.text
		return lit
	case tokString:
		p.advance()
		lit := p.node(tok.span, astString)
		lit.text = tok.literal
		lit.raw = tok.text
		lit.value = stringValue(tok.literal)
		return lit
	case tokLParen:
		return p.parseCodeParen()
	case tokLBracket:
		content := p.parseDelimited()
		if content != nil {
			content.open = ""
			content.close = ""
			content.raw = "content"
		}
		return content
	case tokLBrace:
		return p.unsupportedCodeBlock()
	case tokGlyph:
		if tok.text == "-" {
			p.advance()
			operand := p.parseCodeAtom()
			if operand == nil {
				p.errorAt(tok.span, source.CodeMissingArgument, "expected expression after -")
				return nil
			}
			unary := p.node(source.Merge(tok.span, operand.span), astUnary)
			unary.text = "-"
			unary.items = []*astNode{operand}
			return unary
		}
	case tokShorthand:
		if tok.literal == "−" {
			p.advance()
			operand := p.parseCodeAtom()
			if operand == nil {
				p.errorAt(tok.span, source.CodeMissingArgument, "expected expression after -")
				return nil
			}
			unary := p.node(source.Merge(tok.span, operand.span), astUnary)
			unary.text = "-"
			unary.items = []*astNode{operand}
			return unary
		}
	case tokDollar:
		return p.parsePrimary()
	case tokHash:
		return p.parseCode()
	}
	return p.parsePrimary()
}

func (p *parser) unsupportedCodeBlock() *astNode {
	start := p.current()
	depth := 0
	end := start.span.End
	for p.current().kind != tokEOF {
		tok := p.advance()
		end = tok.span.End
		switch tok.kind {
		case tokLBrace:
			depth++
		case tokRBrace:
			depth--
			if depth == 0 {
				p.errorAt(source.NewRange(start.span.Start, end), source.CodeUnsupported, "code blocks are not supported in Typst math")
				bad := p.node(source.NewRange(start.span.Start, end), astError)
				bad.raw = p.src[start.span.Start:end]
				return bad
			}
		}
	}
	p.errorAt(source.NewRange(start.span.Start, end), source.CodeUnclosedGroup, "unclosed code block")
	bad := p.node(source.NewRange(start.span.Start, end), astError)
	bad.raw = p.src[start.span.Start:end]
	return bad
}

func (p *parser) parseCodeParen() *astNode {
	open := p.advance()
	if p.current().kind == tokRParen {
		end := p.advance().span.End
		arr := p.node(source.NewRange(open.span.Start, end), astArray)
		arr.value = arrayValue(nil)
		return arr
	}

	type item struct {
		name  string
		node  *astNode
		named bool
	}
	items := make([]item, 0, 4)
	dictionary := false
	hadComma := false
	for p.current().kind != tokEOF && p.current().kind != tokRParen {
		before := p.pos
		current := item{}
		if named, nameLen := p.namedArgName(); named != "" {
			p.advance()
			for i := 1; i < nameLen; i++ {
				p.advance()
			}
			p.advance()
			current.name = named
			current.named = true
			dictionary = true
			current.node = p.parseCodeItem()
		} else {
			current.node = p.parseCodeItem()
		}
		if p.pos == before {
			for p.current().kind != tokEOF && p.current().kind != tokComma && p.current().kind != tokRParen {
				p.advance()
			}
		}
		items = append(items, current)
		if p.current().kind == tokComma {
			hadComma = true
			p.advance()
			continue
		}
		break
	}
	end := p.current().span.End
	if p.current().kind == tokRParen {
		p.advance()
	} else {
		p.errorAt(open.span, source.CodeUnclosedGroup, "unclosed argument list")
	}
	if dictionary {
		dict := p.node(source.NewRange(open.span.Start, end), astDict)
		for _, item := range items {
			if !item.named {
				p.errorAt(item.node.span, source.CodeSyntax, "expected named pair in dictionary")
				continue
			}
			dict.args = append(dict.args, astArg{kind: argNamed, name: item.name, span: item.node.span, node: item.node})
		}
		return dict
	}
	if len(items) == 1 && !hadComma {
		code := p.node(source.NewRange(open.span.Start, end), astCode)
		code.items = []*astNode{items[0].node}
		return code
	}
	arr := p.node(source.NewRange(open.span.Start, end), astArray)
	arr.items = make([]*astNode, 0, len(items))
	for _, item := range items {
		arr.items = append(arr.items, item.node)
	}
	return arr
}

// parseCodeItem prefers a code value when the item is exactly one value before
// the next comma or closing paren. Otherwise it reparses the item as math
// content, allowing arrays of both values and content expressions.
func (p *parser) parseCodeItem() *astNode {
	pos := p.pos
	diags := len(p.diags)
	nodes := p.nodes
	depth := p.depth

	n := p.parseCodeAtom()
	if n != nil && (p.current().kind == tokComma || p.current().kind == tokRParen || p.current().kind == tokEOF) {
		return n
	}
	p.pos = pos
	p.diags = p.diags[:diags]
	p.nodes = nodes
	p.depth = depth
	return p.parseRow(stopComma | stopRParen)
}
