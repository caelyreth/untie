package tex

import (
	"strings"

	"github.com/caelyreth/untie/source"
)

type urlArgument struct {
	text    string
	span    source.Range
	origin  source.Range
	present bool
	closed  bool
}

type urlGroup struct {
	text    string
	content source.Range
	open    source.Range
	present bool
	closed  bool
}

func (p *parser) parseHref(command token) astNode {
	destination := p.parseURLArgument(command)
	if !destination.present || !destination.closed {
		return newPlaceholderAST(urlFallbackSpan(destination, command), "invalid link destination")
	}

	p.exp.consumeSpaces()
	if missingRequiredArgumentToken(p.exp.peek()) {
		p.report(source.Diagnostic{
			Range:    normalizeRange(command.span),
			Severity: source.SeverityError,
			Code:     source.CodeMissingArgument,
			Message:  "\\href requires a body argument",
		})
		return newPlaceholderAST(normalizeRange(command.span), "missing href body")
	}

	diagnosticsBefore := len(p.diagnostics)
	body := p.parseRequiredArgument(command)
	if hasMalformedHrefBodyDiagnostic(p.diagnostics[diagnosticsBefore:]) {
		return body
	}
	p.report(source.Diagnostic{
		Range:    normalizeRange(command.span),
		Severity: source.SeverityWarning,
		Code:     source.CodeUnsupported,
		Message:  "hyperlinks are outside the formula IR; link destination was discarded",
	})
	return body
}

func (p *parser) parseURL(command token) astNode {
	argument := p.parseURLArgument(command)
	if !argument.present {
		return newPlaceholderAST(urlFallbackSpan(argument, command), "missing URL")
	}
	if argument.closed {
		p.report(source.Diagnostic{
			Range:    normalizeRange(command.span),
			Severity: source.SeverityWarning,
			Code:     source.CodeUnsupported,
			Message:  "hyperlinks are outside the formula IR; link destination was discarded",
		})
	}
	return urlTextNode(argument)
}

func (p *parser) parseURLArgument(owner token) urlArgument {
	p.exp.consumeSpaces()
	next := p.exp.peek()
	if missingRequiredArgumentToken(next) {
		message := "\\url requires a URL argument"
		if owner.text == "\\href" {
			message = "\\href requires a destination argument"
		}
		p.report(source.Diagnostic{
			Range:    normalizeRange(owner.span),
			Severity: source.SeverityError,
			Code:     source.CodeMissingArgument,
			Message:  message,
		})
		return urlArgument{span: normalizeRange(owner.span), origin: tokenOrigin(owner)}
	}
	if next.is(tokBeginGroup) {
		group := p.consumeURLGroup()
		if !group.closed {
			p.report(source.Diagnostic{
				Range:    normalizeRange(group.open),
				Severity: source.SeverityError,
				Code:     source.CodeUnclosedGroup,
				Message:  "unclosed URL group",
			})
		}
		span, origin := p.urlExpandedSpan(owner, group.content)
		return urlArgument{
			text:    group.text,
			span:    span,
			origin:  origin,
			present: group.present,
			closed:  group.closed,
		}
	}

	token := p.exp.next()
	span := validURLTokenSpan(token, owner)
	return urlArgument{
		text:    unescapeURLText(token.text),
		span:    span,
		origin:  tokenOrigin(owner),
		present: true,
		closed:  true,
	}
}

func (p *parser) consumeURLGroup() urlGroup {
	open := p.exp.popRaw()
	if !open.is(tokBeginGroup) {
		p.exp.push(open)
		return urlGroup{}
	}
	var text strings.Builder
	depth := 1
	start := open.span.End
	end := start
	for {
		token := p.exp.popRaw()
		if token.isEOF() {
			break
		}
		end = token.span.End
		switch token.kind {
		case tokBeginGroup:
			depth++
			if depth > 1 {
				text.WriteString("{")
			}
		case tokEndGroup:
			depth--
			if depth == 0 {
				end = token.span.Start
				return urlGroup{
					text:    unescapeURLText(text.String()),
					content: validRange(start, end),
					open:    open.span,
					present: true,
					closed:  true,
				}
			}
			text.WriteString("}")
		default:
			text.WriteString(token.text)
		}
	}
	return urlGroup{
		text:    unescapeURLText(text.String()),
		content: validRange(start, end),
		open:    open.span,
		present: true,
	}
}

func (p *parser) urlExpandedSpan(owner token, content source.Range) (source.Range, source.Range) {
	origin := tokenOrigin(owner)
	if owner.depth == 0 && owner.origin == owner.span && content.IsValid() {
		return content, origin
	}
	if origin.IsDetached() {
		origin = normalizeRange(owner.span)
	}
	return origin, origin
}

func urlTextNode(argument urlArgument) astNode {
	if argument.text == "" {
		return &astRowNode{
			astBase: newASTBaseAt(astRow, argument.span, argument.origin),
		}
	}
	return astLiteralText(token{span: argument.span, origin: argument.origin}, argument.text)
}

func urlFallbackSpan(argument urlArgument, command token) source.Range {
	if argument.span.IsValid() {
		return argument.span
	}
	if origin := tokenOrigin(command); origin.IsValid() {
		return origin
	}
	return normalizeRange(command.span)
}

func validURLTokenSpan(token token, owner token) source.Range {
	if token.span.IsValid() {
		return token.span
	}
	if origin := tokenOrigin(owner); origin.IsValid() {
		return origin
	}
	return normalizeRange(owner.span)
}

func missingRequiredArgumentToken(token token) bool {
	return token.isEOF() || token.is(tokEndGroup) || token.is(tokAlignment) ||
		token.isControl() && (token.text == "\\right" || token.text == "\\middle" || token.text == "\\end")
}

func hasMalformedHrefBodyDiagnostic(diagnostics []source.Diagnostic) bool {
	for _, diagnostic := range diagnostics {
		if diagnostic.Severity != source.SeverityError {
			continue
		}
		if diagnostic.Code == source.CodeMissingArgument || diagnostic.Code == source.CodeUnclosedGroup {
			return true
		}
	}
	return false
}

func validRange(start, end int) source.Range {
	if start < 0 || end < start {
		return source.Detached()
	}
	return source.NewRange(start, end)
}

func unescapeURLText(text string) string {
	if !strings.Contains(text, "\\") {
		return text
	}
	var out strings.Builder
	out.Grow(len(text))
	for i := 0; i < len(text); i++ {
		if text[i] == '\\' && i+1 < len(text) && strings.ContainsRune("#$%&~_^{}", rune(text[i+1])) {
			i++
		}
		out.WriteByte(text[i])
	}
	return out.String()
}
