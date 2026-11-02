package tex

import (
	"strconv"
	"strings"

	"github.com/caelyreth/untie/internal/mathdata"
	"github.com/caelyreth/untie/source"
)

type definitionKind uint8

const (
	defNormal definitionKind = iota
	defNew
	defRenew
	defProvide
)

func definitionKindFor(command string) definitionKind {
	switch command {
	case "\\newcommand":
		return defNew
	case "\\renewcommand":
		return defRenew
	case "\\providecommand":
		return defProvide
	default:
		return defNormal
	}
}

func (p *parser) parseDefinition(command token, kind definitionKind, global bool) {
	expanding := command.text == "\\edef" || command.text == "\\xdef"
	name, nameSpan, ok := p.consumeDefinitionName()
	if !ok {
		return
	}
	args, body, bodySpan, ok := p.consumeDefinitionBody(command)
	if !ok {
		return
	}
	if expanding {
		var expanded bool
		body, expanded = p.exp.expandAll(body, nameSpan)
		if !expanded {
			p.report(source.Diagnostic{
				Range:    normalizeRange(source.Merge(command.span, bodySpan)),
				Severity: source.SeverityError,
				Code:     source.CodeLimitExceeded,
				Message:  "expanding macro definition exceeded configured limits",
			})
			return
		}
	}
	macro := &macroDef{args: args, body: body}
	if !p.exp.define(name, macro, global) {
		p.report(source.Diagnostic{
			Range:    normalizeRange(nameSpan),
			Severity: source.SeverityError,
			Code:     source.CodeInvalidValue,
			Message:  "invalid macro name",
		})
		return
	}
	_ = bodySpan
	_ = kind
}

func (p *parser) parseNewCommand(command token) {
	p.parseNewCommandInScope(command, false)
}

func (p *parser) parseNewCommandInScope(command token, global bool) {
	kind := definitionKindFor(command.text)
	unsupported := false
	p.exp.consumeSpaces()
	if star := p.exp.popRaw(); star.is(tokChar) && star.text == "*" {
		// Starred forms only affect paragraph-level behavior in LaTeX.
	} else {
		p.exp.push(star)
	}

	name, nameSpan, ok := p.consumeNewCommandName(command)
	if !ok {
		return
	}
	args := 0
	if raw, _, present := p.consumeRawOptionalGroup(); present {
		number, err := strconv.Atoi(strings.TrimSpace(raw))
		if err != nil || number < 0 || number > 9 {
			unsupported = true
			p.report(source.Diagnostic{
				Range:    normalizeRange(nameSpan),
				Severity: source.SeverityError,
				Code:     source.CodeInvalidValue,
				Message:  "macro argument count must be an integer from 0 through 9",
			})
		} else {
			args = number
		}
		if _, _, second := p.consumeRawOptionalGroup(); second {
			unsupported = true
			p.report(source.Diagnostic{
				Range:    normalizeRange(nameSpan),
				Severity: source.SeverityError,
				Code:     source.CodeUnsupported,
				Message:  "optional macro arguments with defaults are not supported",
			})
		}
	}

	_, body, bodySpan, ok := p.consumeRawRequiredGroup()
	if !ok {
		p.report(source.Diagnostic{
			Range:    normalizeRange(nameSpan),
			Severity: source.SeverityError,
			Code:     source.CodeMissingArgument,
			Message:  "missing replacement text for " + command.text,
		})
		return
	}
	_ = bodySpan
	if unsupported {
		return
	}

	custom := p.exp.lookup(name) != nil
	builtin := isBuiltinCommand(name)
	exists := custom || builtin
	switch kind {
	case defNew:
		if exists {
			p.report(source.Diagnostic{
				Range:    normalizeRange(nameSpan),
				Severity: source.SeverityError,
				Code:     source.CodeInvalidValue,
				Message:  name + " is already defined; use \\renewcommand to replace it",
			})
			return
		}
	case defRenew:
		if !exists {
			p.report(source.Diagnostic{
				Range:    normalizeRange(nameSpan),
				Severity: source.SeverityError,
				Code:     source.CodeInvalidValue,
				Message:  name + " is not defined; use \\newcommand instead",
			})
			return
		}
	case defProvide:
		if exists {
			return
		}
	}
	p.exp.define(name, &macroDef{args: args, body: body}, global)
}

func (p *parser) parseLet(command token) {
	name, nameSpan, ok := p.consumeDefinitionName()
	if !ok {
		return
	}
	p.exp.consumeSpaces()
	t := p.exp.popRaw()
	if t.is(tokChar) && t.text == "=" {
		p.exp.consumeSpaces()
		t = p.exp.popRaw()
	}
	if t.isEOF() || t.is(tokEndGroup) {
		p.report(source.Diagnostic{
			Range:    normalizeRange(nameSpan),
			Severity: source.SeverityError,
			Code:     source.CodeMissingArgument,
			Message:  "\\let requires a target token",
		})
		return
	}
	if target := p.exp.lookup(t.text); target != nil {
		p.exp.define(name, target, false)
		return
	}
	alias := retarget(t, command.span, 0)
	alias.raw = true
	p.exp.define(name, &macroDef{args: 0, body: []token{alias}}, false)
}

func (p *parser) parseGlobal(command token) {
	p.exp.consumeSpaces()
	t := p.exp.next()
	if !t.isControl() {
		p.report(source.Diagnostic{
			Range:    normalizeRange(t.span),
			Severity: source.SeverityError,
			Code:     source.CodeUnexpectedToken,
			Message:  "\\global must be followed by a macro definition",
		})
		return
	}
	switch t.text {
	case "\\def", "\\gdef", "\\edef", "\\xdef":
		p.parseDefinition(t, defNormal, true)
	case "\\newcommand", "\\renewcommand", "\\providecommand":
		p.parseNewCommandInScope(t, true)
	default:
		p.report(source.Diagnostic{
			Range:    normalizeRange(t.span),
			Severity: source.SeverityError,
			Code:     source.CodeUnexpectedToken,
			Message:  "\\global must be followed by a macro definition",
		})
	}
	_ = command
}

func (p *parser) consumeDefinitionName() (string, source.Range, bool) {
	p.exp.consumeSpaces()
	t := p.exp.popRaw()
	if !t.isControl() || t.isEOF() {
		p.report(source.Diagnostic{
			Range:    normalizeRange(t.span),
			Severity: source.SeverityError,
			Code:     source.CodeInvalidValue,
			Message:  "expected a control sequence for macro name",
		})
		return "", t.span, false
	}
	return t.text, t.span, true
}

func (p *parser) consumeNewCommandName(command token) (string, source.Range, bool) {
	p.exp.consumeSpaces()
	t := p.exp.popRaw()
	if t.is(tokBeginGroup) {
		p.exp.consumeSpaces()
		name := p.exp.popRaw()
		p.exp.consumeSpaces()
		close := p.exp.popRaw()
		if !name.isControl() || !close.is(tokEndGroup) {
			p.report(source.Diagnostic{
				Range:    normalizeRange(source.Merge(t.span, close.span)),
				Severity: source.SeverityError,
				Code:     source.CodeInvalidValue,
				Message:  command.text + " requires a control sequence as its first argument",
			})
			return "", t.span, false
		}
		return name.text, source.Merge(t.span, close.span), true
	}
	if !t.isControl() {
		p.report(source.Diagnostic{
			Range:    normalizeRange(t.span),
			Severity: source.SeverityError,
			Code:     source.CodeInvalidValue,
			Message:  command.text + " requires a control sequence as its first argument",
		})
		return "", t.span, false
	}
	return t.text, t.span, true
}

func (p *parser) consumeDefinitionBody(command token) (int, []token, source.Range, bool) {
	args := 0
	delimited := false
	open := token{}
	for {
		t := p.exp.popRaw()
		if t.isEOF() {
			p.report(source.Diagnostic{
				Range:    normalizeRange(command.span),
				Severity: source.SeverityError,
				Code:     source.CodeMissingArgument,
				Message:  "unexpected end of input in macro definition",
			})
			return 0, nil, t.span, false
		}
		if t.is(tokBeginGroup) {
			open = t
			break
		}
		if t.is(tokParameter) {
			p.exp.consumeSpaces()
			number := p.exp.popRaw()
			if !number.is(tokChar) || len(number.text) != 1 || number.text[0] < '1' || number.text[0] > '9' {
				p.report(source.Diagnostic{
					Range:    normalizeRange(number.span),
					Severity: source.SeverityError,
					Code:     source.CodeInvalidValue,
					Message:  "macro parameter must be # followed by an argument number",
				})
				continue
			}
			want := strconv.Itoa(args + 1)
			if number.text != want {
				p.report(source.Diagnostic{
					Range:    normalizeRange(number.span),
					Severity: source.SeverityError,
					Code:     source.CodeInvalidValue,
					Message:  "macro parameters must be numbered consecutively from #1",
				})
			}
			args++
			if args > 9 {
				p.report(source.Diagnostic{
					Range:    normalizeRange(number.span),
					Severity: source.SeverityError,
					Code:     source.CodeLimitExceeded,
					Message:  "TeX macros support at most nine arguments",
				})
				args = 9
			}
			continue
		}
		delimited = true
	}
	body, span := p.consumeRawGroupAfter(open)
	if delimited {
		p.report(source.Diagnostic{
			Range:    normalizeRange(source.Merge(command.span, span)),
			Severity: source.SeverityError,
			Code:     source.CodeUnsupported,
			Message:  "delimited macro parameters are not supported",
		})
		return args, body, span, false
	}
	return args, body, span, true
}

func (p *parser) consumeRawRequiredGroup() (source.Range, []token, source.Range, bool) {
	p.exp.consumeSpaces()
	open := p.exp.popRaw()
	if !open.is(tokBeginGroup) {
		p.exp.push(open)
		return open.span, nil, open.span, false
	}
	body, span := p.consumeRawGroupAfter(open)
	return open.span, body, span, true
}

func (p *parser) consumeRawGroupAfter(open token) ([]token, source.Range) {
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
				Message:  "unexpected end of input in macro replacement text",
			})
			return body, source.Merge(span, t.span)
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
				return body, span
			}
			body = append(body, t)
			continue
		}
		body = append(body, t)
	}
}

func (p *parser) consumeRawOptionalGroup() (string, source.Range, bool) {
	p.exp.consumeSpaces()
	open := p.exp.popRaw()
	if !open.is(tokChar) || open.text != "[" {
		p.exp.push(open)
		return "", source.Detached(), false
	}
	var b strings.Builder
	depth := 0
	span := open.span
	for {
		t := p.exp.popRaw()
		if t.isEOF() {
			p.report(source.Diagnostic{
				Range:    normalizeRange(open.span),
				Severity: source.SeverityError,
				Code:     source.CodeUnclosedGroup,
				Message:  "unexpected end of input in optional macro declaration argument",
			})
			return b.String(), span, true
		}
		span = source.Merge(span, t.span)
		if t.is(tokBeginGroup) {
			depth++
			b.WriteString(t.text)
			continue
		}
		if t.is(tokEndGroup) {
			depth--
			b.WriteString(t.text)
			continue
		}
		if depth == 0 && t.is(tokChar) && t.text == "]" {
			return b.String(), span, true
		}
		b.WriteString(t.text)
	}
}

func isBuiltinCommand(name string) bool {
	if _, ok := symbolCommands[name]; ok {
		return true
	}
	if _, ok := delimiterCommands[name]; ok {
		return true
	}
	if _, ok := accentCommands[name]; ok {
		return true
	}
	if _, ok := spacingCommands[name]; ok {
		return true
	}
	if _, ok := namedOperators[name]; ok {
		return true
	}
	if _, ok := fontCommands[name]; ok {
		return true
	}
	if _, ok := infixCommands[name]; ok {
		return true
	}
	if symbol, ok := mathdata.LookupTeXSymbol(name); ok && symbol.Kind == mathdata.TeXSymbolPublic {
		return true
	}
	if _, ok := mathdata.LookupTeXOperator(name); ok {
		return true
	}
	switch name {
	case "\\frac", "\\dfrac", "\\tfrac", "\\cfrac", "\\binom", "\\dbinom", "\\tbinom",
		"\\sqrt", "\\left", "\\right", "\\middle", "\\overline", "\\underline", "\\underbar",
		"\\cancel", "\\bcancel", "\\xcancel", "\\text", "\\textrm", "\\textnormal",
		"\\textsf", "\\texttt", "\\textbf", "\\textmd", "\\textit", "\\textup", "\\emph",
		"\\operatorname", "\\displaystyle", "\\textstyle", "\\scriptstyle",
		"\\scriptscriptstyle", "\\rm", "\\it", "\\bf", "\\sf", "\\tt", "\\cal",
		"\\kern", "\\mkern", "\\hspace", "\\hskip", "\\mspace", "\\nobreak", "\\allowbreak",
		"\\relax", "\\def", "\\gdef", "\\edef", "\\xdef", "\\newcommand", "\\renewcommand",
		"\\providecommand", "\\let", "\\global", "\\begin", "\\end", "\\limits", "\\nolimits",
		"\\big", "\\Big", "\\bigg", "\\Bigg", "\\bigl", "\\Bigl", "\\biggl", "\\Biggl",
		"\\bigr", "\\Bigr", "\\biggr", "\\Biggr", "\\bigm", "\\Bigm", "\\biggm", "\\Biggm":
		return true
	default:
		return false
	}
}
