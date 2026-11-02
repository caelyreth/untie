package tex

import (
	"fmt"
	"slices"
	"strings"

	"github.com/caelyreth/untie/source"
)

type macroDef struct {
	args     int
	body     []token
	hostBody string
	compiled bool
}

type expander struct {
	tokens             []token
	scopes             []map[string]*macroDef
	diagnostic         func(source.Diagnostic)
	limits             source.Limits
	expansionCount     int
	expandedTokenCount int
	disabled           bool
	limitReported      bool
}

func newExpander(src string, limits source.Limits, macros map[string]Macro, diagnostic func(source.Diagnostic)) *expander {
	lex := newLexer(src, limits, false, diagnostic)
	tokens := lex.all()
	e := &expander{
		tokens:     make([]token, 0, len(tokens)),
		scopes:     []map[string]*macroDef{{}},
		diagnostic: diagnostic,
		limits:     limits,
	}
	for i := len(tokens) - 1; i >= 0; i-- {
		e.tokens = append(e.tokens, tokens[i])
	}

	names := make([]string, 0, len(macros))
	for rawName := range macros {
		names = append(names, rawName)
	}
	slices.Sort(names)
	seen := make(map[string]bool, len(names))
	for _, rawName := range names {
		macro := macros[rawName]
		name := normalizeMacroName(rawName)
		if name == "" {
			continue
		}
		if seen[name] {
			diagnostic(source.Diagnostic{
				Range:    source.Detached(),
				Severity: source.SeverityError,
				Code:     source.CodeInvalidValue,
				Message:  "duplicate host macro name after normalization: " + name,
			})
			continue
		}
		seen[name] = true
		args := min(max(macro.Args, 0), 9)
		e.scopes[0][name] = &macroDef{
			args:     args,
			hostBody: macro.Body,
		}
	}
	return e
}

func normalizeMacroName(name string) string {
	name = strings.TrimSpace(name)
	if name == "" || strings.HasPrefix(name, "\\") {
		return name
	}
	if len(name) == 1 && strings.ContainsRune("{}$&#^_", rune(name[0])) {
		return name
	}
	return "\\" + name
}

func (e *expander) next() token {
	for {
		t := e.popRaw()
		if t.isEOF() {
			return t
		}
		if t.raw || e.disabled {
			return t
		}
		m := e.lookup(t.text)
		if m == nil {
			return t
		}
		if !e.expand(t, m) {
			t.raw = true
			return t
		}
	}
}

func (e *expander) peek() token {
	if len(e.tokens) == 0 {
		return token{kind: tokEOF, span: source.Detached(), origin: source.Detached()}
	}
	top := e.tokens[len(e.tokens)-1]
	if top.isEOF() || top.raw || e.disabled {
		return top
	}
	t := e.next()
	e.tokens = append(e.tokens, t)
	return t
}

func (e *expander) popRaw() token {
	if len(e.tokens) == 0 {
		return token{kind: tokEOF, span: source.Detached(), origin: source.Detached()}
	}
	last := len(e.tokens) - 1
	t := e.tokens[last]
	e.tokens = e.tokens[:last]
	return t
}

func (e *expander) push(tokens ...token) {
	for i := len(tokens) - 1; i >= 0; i-- {
		e.tokens = append(e.tokens, tokens[i])
	}
}

func (e *expander) consumeSpaces() {
	for {
		t := e.popRaw()
		if !t.is(tokSpace) {
			e.push(t)
			return
		}
	}
}

func (e *expander) consumeArgument() ([]token, source.Range, bool) {
	e.consumeSpaces()
	first := e.popRaw()
	if first.isEOF() {
		e.push(first)
		return nil, first.span, false
	}
	if first.is(tokBeginGroup) {
		var out []token
		depth := 1
		for {
			t := e.popRaw()
			if t.isEOF() {
				e.diagnostic(source.Diagnostic{
					Range:    source.Merge(first.span, t.span),
					Severity: source.SeverityError,
					Code:     source.CodeUnclosedGroup,
					Message:  "unexpected end of input in macro argument, expected '}'",
				})
				return out, source.Merge(first.span, t.span), false
			}
			switch t.kind {
			case tokBeginGroup:
				depth++
				out = append(out, t)
			case tokEndGroup:
				depth--
				if depth == 0 {
					return out, source.Merge(first.span, t.span), true
				}
				out = append(out, t)
			default:
				out = append(out, t)
			}
		}
	}
	return []token{first}, first.span, true
}

func (e *expander) beginScope() {
	e.scopes = append(e.scopes, map[string]*macroDef{})
}

func (e *expander) endScope() {
	if len(e.scopes) > 1 {
		e.scopes = e.scopes[:len(e.scopes)-1]
	}
}

func (e *expander) lookup(name string) *macroDef {
	for i := len(e.scopes) - 1; i >= 0; i-- {
		if m, ok := e.scopes[i][name]; ok {
			return m
		}
	}
	return nil
}

func (e *expander) define(name string, m *macroDef, global bool) bool {
	if name == "" || m == nil {
		return false
	}
	copy := *m
	copy.body = append([]token(nil), m.body...)
	if global {
		e.scopes[0][name] = &copy
		return true
	}
	e.scopes[len(e.scopes)-1][name] = &copy
	return true
}

func (e *expander) expand(call token, m *macroDef) bool {
	if e.expansionCount >= e.limits.MaxMacroExpansions {
		e.reportLimit(call, "TeX macro expansion limit exceeded")
		e.disabled = true
		return false
	}
	e.expansionCount++

	args := make([][]token, m.args)
	invocation := call.span
	for i := range args {
		var argSpan source.Range
		var ok bool
		args[i], argSpan, ok = e.consumeArgument()
		invocation = source.Merge(invocation, argSpan)
		if !ok {
			e.diagnostic(source.Diagnostic{
				Range:    nonEmptyRange(source.Merge(call.span, argSpan)),
				Severity: source.SeverityError,
				Code:     source.CodeMissingArgument,
				Message:  fmt.Sprintf("missing argument %d to macro %s", i+1, call.text),
			})
		}
	}

	if m.hostBody != "" && !m.compiled {
		hostLex := newLexer(m.hostBody, e.limits, true, e.diagnostic)
		hostTokens := hostLex.all()
		if len(hostTokens) > 0 && hostTokens[len(hostTokens)-1].isEOF() {
			hostTokens = hostTokens[:len(hostTokens)-1]
		}
		m.body = hostTokens
		m.compiled = true
	}
	if !m.compiled {
		m.compiled = true
	}

	replacement := make([]token, 0, len(m.body))
	for i := 0; i < len(m.body); i++ {
		t := m.body[i]
		if t.is(tokParameter) {
			if i+1 >= len(m.body) {
				e.diagnostic(source.Diagnostic{
					Range:    nonEmptyRange(t.span),
					Severity: source.SeverityError,
					Code:     source.CodeInvalidValue,
					Message:  "incomplete macro parameter at end of replacement text",
				})
				break
			}
			i++
			next := m.body[i]
			if next.is(tokParameter) {
				t.text = "#"
				replacement = append(replacement, retarget(t, invocation, call.depth+1))
				continue
			}
			if next.is(tokChar) && len(next.text) == 1 && next.text[0] >= '1' && next.text[0] <= '9' {
				argIndex := int(next.text[0] - '1')
				if argIndex >= len(args) {
					e.diagnostic(source.Diagnostic{
						Range:    nonEmptyRange(next.span),
						Severity: source.SeverityError,
						Code:     source.CodeInvalidValue,
						Message:  fmt.Sprintf("macro argument #%d is not declared", argIndex+1),
					})
					continue
				}
				for _, arg := range args[argIndex] {
					if arg.depth < call.depth {
						arg.depth = call.depth
					}
					replacement = append(replacement, arg)
				}
				continue
			}
			e.diagnostic(source.Diagnostic{
				Range:    nonEmptyRange(next.span),
				Severity: source.SeverityError,
				Code:     source.CodeInvalidValue,
				Message:  "macro parameter must be # followed by an argument number",
			})
			continue
		}
		replacement = append(replacement, retarget(t, invocation, call.depth+1))
	}

	if call.depth+1 > e.limits.MaxCallDepth {
		e.diagnostic(source.Diagnostic{
			Range:    nonEmptyRange(invocation),
			Severity: source.SeverityError,
			Code:     source.CodeLimitExceeded,
			Message:  "TeX macro call depth limit exceeded",
		})
		return false
	}
	e.expandedTokenCount += len(replacement)
	if e.expandedTokenCount > e.limits.MaxExpandedTokens {
		e.reportLimit(call, "TeX expanded token limit exceeded")
		e.disabled = true
		return false
	}

	e.push(replacement...)
	return true
}

func (e *expander) reportLimit(t token, message string) {
	if e.limitReported {
		return
	}
	e.limitReported = true
	e.diagnostic(source.Diagnostic{
		Range:    nonEmptyRange(t.span),
		Severity: source.SeverityError,
		Code:     source.CodeLimitExceeded,
		Message:  message,
	})
}

func retarget(t token, invocation source.Range, depth int) token {
	if t.span.IsDetached() {
		t.span = invocation
	}
	t.origin = invocation
	t.depth = depth
	t.raw = false
	return t
}

func nonEmptyRange(r source.Range) source.Range {
	if r.IsDetached() {
		return source.Detached()
	}
	if r.End == r.Start {
		r.End++
	}
	return r
}
