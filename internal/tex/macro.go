package tex

import (
	"fmt"
	"slices"
	"strings"
	"sync"

	"github.com/caelyreth/untie/source"
)

type macroDef struct {
	args     int
	body     []token
	hostBody string
	compiled bool
}

type macroScope struct {
	defs map[string]*macroDef
}

type expander struct {
	tokens             []token
	tokenSlot          *[]token
	rootScope          macroScope
	scopes             []macroScope
	diagnostic         func(source.Diagnostic)
	limits             source.Limits
	expansionCount     int
	expandedTokenCount int
	disabled           bool
	limitReported      bool
}

const maxPooledTokens = 64 * 1024

var tokenBufferPool = sync.Pool{
	New: func() any {
		return new([]token)
	},
}

func newExpander(src string, limits source.Limits, macros map[string]Macro, diagnostic func(source.Diagnostic)) *expander {
	lex := newLexer(src, limits, false, diagnostic)
	capacity := tokenCapacityHint(src, limits.MaxTokens)
	slot := tokenBufferPool.Get().(*[]token)
	tokens := (*slot)[:0]
	if cap(tokens) < capacity {
		tokens = make([]token, 0, capacity)
	}
	e := &expander{
		tokens:     tokens,
		tokenSlot:  slot,
		diagnostic: diagnostic,
		limits:     limits,
	}
	e.tokens = lex.appendAll(e.tokens)
	slices.Reverse(e.tokens)

	if len(macros) == 0 {
		return e
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
		if macro.Args < 0 || macro.Args > 9 {
			diagnostic(source.Diagnostic{
				Range:    source.Detached(),
				Severity: source.SeverityError,
				Code:     source.CodeInvalidValue,
				Message:  "host macro " + name + " argument count must be between 0 and 9",
			})
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
		if e.rootScope.defs == nil {
			e.rootScope.defs = make(map[string]*macroDef, len(names))
		}
		e.rootScope.defs[name] = &macroDef{
			args:     macro.Args,
			hostBody: macro.Body,
		}
	}
	return e
}

func (e *expander) release() {
	if e == nil || e.tokenSlot == nil {
		return
	}
	if cap(e.tokens) <= maxPooledTokens {
		clear(e.tokens[:cap(e.tokens)])
		*e.tokenSlot = e.tokens[:0]
		tokenBufferPool.Put(e.tokenSlot)
	}
	e.tokens = nil
	e.tokenSlot = nil
}

// subExpander creates an expander over an already-tokenized source range. It
// preserves parser-visible macro scopes and expansion budgets, and the caller
// must absorb the counters after parsing.
func (e *expander) subExpander(tokens []token) *expander {
	sub := &expander{
		tokens:             slices.Clone(tokens),
		rootScope:          e.rootScope,
		scopes:             slices.Clone(e.scopes),
		diagnostic:         e.diagnostic,
		limits:             e.limits,
		expansionCount:     e.expansionCount,
		expandedTokenCount: e.expandedTokenCount,
		disabled:           e.disabled,
		limitReported:      e.limitReported,
	}
	slices.Reverse(sub.tokens)
	return sub
}

func (e *expander) absorb(sub *expander) {
	if sub == nil {
		return
	}
	e.expansionCount = sub.expansionCount
	e.expandedTokenCount = sub.expandedTokenCount
	e.disabled = sub.disabled
	e.limitReported = sub.limitReported
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

// expandAll expands a definition replacement while preserving the parser's
// pending token stream. It is used by \edef and \xdef, whose replacement text
// is frozen at definition time.
func (e *expander) expandAll(tokens []token, endSpan source.Range) ([]token, bool) {
	saved := e.tokens
	defer func() { e.tokens = saved }()

	e.tokens = make([]token, 0, len(tokens)+1)
	e.tokens = append(e.tokens, token{kind: tokEOF, span: endSpan, origin: endSpan})
	e.push(tokens...)

	expanded := make([]token, 0, len(tokens))
	for {
		t := e.next()
		if t.isEOF() {
			return expanded, !e.disabled
		}
		expanded = append(expanded, t)
		if e.disabled {
			return nil, false
		}
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
	if e.scopes == nil {
		capacity := 8
		if e.limits.MaxNesting < capacity {
			capacity = e.limits.MaxNesting
		}
		e.scopes = make([]macroScope, 0, max(capacity, 1))
	}
	e.scopes = append(e.scopes, macroScope{})
}

func (e *expander) endScope() {
	if len(e.scopes) > 0 {
		e.scopes = e.scopes[:len(e.scopes)-1]
	}
}

func (e *expander) lookup(name string) *macroDef {
	for i := len(e.scopes) - 1; i >= 0; i-- {
		defs := e.scopes[i].defs
		if defs == nil {
			continue
		}
		if m, ok := defs[name]; ok {
			return m
		}
	}
	if m, ok := e.rootScope.defs[name]; ok {
		return m
	}
	return nil
}

func (e *expander) define(name string, m *macroDef, global bool) bool {
	if name == "" || m == nil {
		return false
	}
	copy := *m
	copy.body = append([]token(nil), m.body...)
	scope := &e.rootScope
	if !global && len(e.scopes) > 0 {
		scope = &e.scopes[len(e.scopes)-1]
	}
	if scope.defs == nil {
		scope.defs = make(map[string]*macroDef)
	}
	scope.defs[name] = &copy
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
				Range:    normalizeRange(source.Merge(call.span, argSpan)),
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

	if call.depth+1 > e.limits.MaxCallDepth {
		e.diagnostic(source.Diagnostic{
			Range:    normalizeRange(invocation),
			Severity: source.SeverityError,
			Code:     source.CodeLimitExceeded,
			Message:  "TeX macro call depth limit exceeded",
		})
		return false
	}

	size, ok := e.replacementSize(m, args)
	if !ok {
		e.reportLimit(call, "TeX expanded token limit exceeded")
		e.disabled = true
		return false
	}

	replacement := make([]token, 0, size)
	for i := 0; i < len(m.body); i++ {
		t := m.body[i]
		if t.is(tokParameter) {
			if i+1 >= len(m.body) {
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
			continue
		}
		replacement = append(replacement, retarget(t, invocation, call.depth+1))
	}

	e.expandedTokenCount += len(replacement)
	e.push(replacement...)
	return true
}

// replacementSize validates the replacement text while accounting for its size
// against the remaining expansion budget. It must run before replacement is
// allocated so repeated arguments cannot force a large allocation first.
func (e *expander) replacementSize(m *macroDef, args [][]token) (int, bool) {
	remaining := e.limits.MaxExpandedTokens - e.expandedTokenCount
	if remaining < 0 {
		return 0, false
	}
	size := 0
	add := func(n int) bool {
		if n < 0 || n > remaining-size {
			return false
		}
		size += n
		return true
	}

	for i := 0; i < len(m.body); i++ {
		t := m.body[i]
		if !t.is(tokParameter) {
			if !add(1) {
				return 0, false
			}
			continue
		}
		if i+1 >= len(m.body) {
			e.diagnostic(source.Diagnostic{
				Range:    normalizeRange(t.span),
				Severity: source.SeverityError,
				Code:     source.CodeInvalidValue,
				Message:  "incomplete macro parameter at end of replacement text",
			})
			break
		}
		i++
		next := m.body[i]
		switch {
		case next.is(tokParameter):
			if !add(1) {
				return 0, false
			}
		case next.is(tokChar) && len(next.text) == 1 && next.text[0] >= '1' && next.text[0] <= '9':
			argIndex := int(next.text[0] - '1')
			if argIndex >= len(args) {
				e.diagnostic(source.Diagnostic{
					Range:    normalizeRange(next.span),
					Severity: source.SeverityError,
					Code:     source.CodeInvalidValue,
					Message:  fmt.Sprintf("macro argument #%d is not declared", argIndex+1),
				})
				continue
			}
			if !add(len(args[argIndex])) {
				return 0, false
			}
		default:
			e.diagnostic(source.Diagnostic{
				Range:    normalizeRange(next.span),
				Severity: source.SeverityError,
				Code:     source.CodeInvalidValue,
				Message:  "macro parameter must be # followed by an argument number",
			})
		}
	}
	return size, true
}

func (e *expander) reportLimit(t token, message string) {
	if e.limitReported {
		return
	}
	e.limitReported = true
	e.diagnostic(source.Diagnostic{
		Range:    normalizeRange(t.span),
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

// normalizeRange preserves valid source ranges. Zero-width ranges are valid caret
// positions, including at EOF, and must not be widened beyond the source.
func normalizeRange(r source.Range) source.Range {
	if r.IsDetached() {
		return source.Detached()
	}
	return r
}
