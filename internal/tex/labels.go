package tex

import (
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/caelyreth/untie/ir"
	"github.com/caelyreth/untie/source"
)

type labelArgument struct {
	text    string
	span    source.Range
	origin  source.Range
	present bool
	closed  bool
}

func (p *parser) parseEquationLabel(command token) {
	argument := p.parseLabelArgument(command)
	if !argument.present || !argument.closed {
		return
	}
	span, origin := equationLabelSpan(command, argument.span)
	name := unescapeLabelName(argument.text)
	if code, message, ok := validLabelName(name, p.limits.MaxTextBytes); !ok {
		p.report(source.Diagnostic{
			Range:    normalizeRange(span),
			Severity: source.SeverityError,
			Code:     code,
			Message:  message,
		})
		return
	}
	for _, existing := range p.labels {
		if existing.name == name {
			p.report(source.Diagnostic{
				Range:    normalizeRange(span),
				Severity: source.SeverityError,
				Code:     source.CodeDuplicateLabel,
				Message:  "duplicate formula label " + quoteLabelForDiagnostic(name),
			})
			return
		}
	}
	p.labels = append(p.labels, astEquationLabel{
		name:   name,
		span:   normalizeRange(span),
		origin: normalizeRange(origin),
	})
}

func (p *parser) parseEquationReference(command token) astNode {
	argument := p.parseLabelArgument(command)
	span, origin := equationLabelSpan(command, argument.span)
	if !argument.present || !argument.closed {
		return newPlaceholderAST(span, labelPlaceholderReason("invalid equation reference", p.limits.MaxTextBytes))
	}
	name := unescapeLabelName(argument.text)
	if code, message, ok := validLabelName(name, p.limits.MaxTextBytes); !ok {
		p.report(source.Diagnostic{
			Range:    normalizeRange(span),
			Severity: source.SeverityError,
			Code:     code,
			Message:  message,
		})
		return newPlaceholderAST(span, labelPlaceholderReason("invalid equation reference", p.limits.MaxTextBytes))
	}
	reference, ok := p.references[name]
	if !ok {
		p.report(source.Diagnostic{
			Range:    normalizeRange(span),
			Severity: source.SeverityError,
			Code:     source.CodeUnresolvedReference,
			Message:  "unresolved equation reference " + quoteLabelForDiagnostic(name),
		})
		return newPlaceholderAST(span, labelPlaceholderReason("unresolved equation reference", p.limits.MaxTextBytes))
	}
	if code, message, ok := validReferenceText(reference, p.limits.MaxTextBytes); !ok {
		p.report(source.Diagnostic{
			Range:    normalizeRange(span),
			Severity: source.SeverityError,
			Code:     code,
			Message:  message + " for equation reference " + quoteLabelForDiagnostic(name),
		})
		return newPlaceholderAST(span, labelPlaceholderReason("invalid equation reference", p.limits.MaxTextBytes))
	}
	return &astTextNode{
		astBase: newASTBaseAt(astText, span, origin),
		text:    "(" + reference + ")",
		variant: ir.VariantNormal,
	}
}

func (p *parser) parseLabelArgument(owner token) labelArgument {
	p.exp.consumeSpaces()
	next := p.exp.peek()
	if missingRequiredArgumentToken(next) {
		p.report(source.Diagnostic{
			Range:    normalizeRange(owner.span),
			Severity: source.SeverityError,
			Code:     source.CodeMissingArgument,
			Message:  owner.text + " requires a label argument",
		})
		return labelArgument{span: normalizeRange(owner.span), origin: tokenOrigin(owner)}
	}
	if next.is(tokBeginGroup) {
		group := p.consumeRawGroupTextValue()
		span := source.Merge(owner.span, group.close)
		if !group.closed {
			span = source.Merge(owner.span, group.content)
			p.report(source.Diagnostic{
				Range:    normalizeRange(span),
				Severity: source.SeverityError,
				Code:     source.CodeUnclosedGroup,
				Message:  owner.text + " label argument is not closed",
			})
		}
		return labelArgument{
			text:    group.text,
			span:    normalizeRange(span),
			origin:  tokenOrigin(owner),
			present: group.present,
			closed:  group.closed,
		}
	}
	token := p.exp.next()
	return labelArgument{
		text:    token.text,
		span:    normalizeRange(source.Merge(owner.span, token.span)),
		origin:  tokenOrigin(owner),
		present: true,
		closed:  true,
	}
}

func equationLabelSpan(command token, argument source.Range) (source.Range, source.Range) {
	span := source.Merge(command.span, argument)
	origin := tokenOrigin(command)
	if origin.IsDetached() {
		origin = span
	}
	if origin != command.span {
		// Macro-expanded labels point to the invocation, never to a span that
		// stitches definition bytes to call-site bytes.
		return origin, origin
	}
	return span, span
}

func validLabelName(name string, maxBytes int) (source.Code, string, bool) {
	switch {
	case strings.TrimSpace(name) == "":
		return source.CodeInvalidValue, "label name must not be empty", false
	case !utf8.ValidString(name):
		return source.CodeInvalidValue, "label name is not valid UTF-8", false
	case hasControlRune(name):
		return source.CodeInvalidValue, "label name contains a control character", false
	case len(name) > maxBytes:
		return source.CodeLimitExceeded, "label name exceeds configured text limit", false
	default:
		return "", "", true
	}
}

func validReferenceText(text string, maxBytes int) (source.Code, string, bool) {
	switch {
	case strings.TrimSpace(text) == "":
		return source.CodeInvalidValue, "equation reference text must not be empty", false
	case !utf8.ValidString(text):
		return source.CodeInvalidValue, "equation reference text is not valid UTF-8", false
	case hasControlRune(text):
		return source.CodeInvalidValue, "equation reference text contains a control character", false
	case len(text)+len("()") > maxBytes:
		return source.CodeLimitExceeded, "equation reference text exceeds configured text limit", false
	default:
		return "", "", true
	}
}

func hasControlRune(text string) bool {
	for _, r := range text {
		if unicode.IsControl(r) {
			return true
		}
	}
	return false
}

func unescapeLabelName(text string) string {
	if !strings.Contains(text, `\`) {
		return text
	}
	var out strings.Builder
	out.Grow(len(text))
	for i := 0; i < len(text); i++ {
		if text[i] == '\\' && i+1 < len(text) && strings.ContainsRune(`#$%&~_^{}\ `, rune(text[i+1])) {
			i++
		}
		out.WriteByte(text[i])
	}
	return out.String()
}

func quoteLabelForDiagnostic(name string) string {
	const maxBytes = 64
	if len(name) <= maxBytes {
		return strconv.Quote(name)
	}
	cut := maxBytes
	for cut > 0 && !utf8.RuneStart(name[cut]) {
		cut--
	}
	return strconv.Quote(name[:cut] + "...")
}

func labelPlaceholderReason(reason string, maxBytes int) string {
	if maxBytes < 1 {
		maxBytes = 1
	}
	if len(reason) <= maxBytes {
		return reason
	}
	reason = reason[:maxBytes]
	for reason != "" && !utf8.ValidString(reason) {
		reason = reason[:len(reason)-1]
	}
	if reason == "" {
		return "?"
	}
	return reason
}
