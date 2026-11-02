package tex

import (
	"unicode"
	"unicode/utf8"

	"github.com/caelyreth/untie/source"
)

type tokenKind uint8

const (
	tokEOF tokenKind = iota
	tokSpace
	tokChar
	tokControlWord
	tokControlSymbol
	tokBeginGroup
	tokEndGroup
	tokSuperscript
	tokSubscript
	tokAlignment
	tokParameter
	tokError
)

type token struct {
	kind   tokenKind
	text   string
	span   source.Range
	origin source.Range
	depth  int
	raw    bool
}

func (t token) is(kind tokenKind) bool {
	return t.kind == kind
}

func (t token) isControl() bool {
	return t.kind == tokControlWord || t.kind == tokControlSymbol
}

func (t token) isEOF() bool {
	return t.kind == tokEOF
}

type lexer struct {
	src        string
	pos        int
	detached   bool
	count      int
	limits     source.Limits
	diagnostic func(source.Diagnostic)
	hitLimit   bool
}

func newLexer(src string, limits source.Limits, detached bool, diagnostic func(source.Diagnostic)) *lexer {
	return &lexer{
		src:        src,
		detached:   detached,
		limits:     limits,
		diagnostic: diagnostic,
	}
}

func (l *lexer) all() []token {
	capacity := min(len(l.src)+1, l.limits.MaxTokens+1)
	out := make([]token, 0, capacity)
	for {
		t := l.next()
		out = append(out, t)
		if t.isEOF() {
			return out
		}
	}
}

func (l *lexer) next() token {
	for l.pos < len(l.src) {
		if l.count >= l.limits.MaxTokens {
			span := l.rangeFor(l.pos, len(l.src))
			if !l.hitLimit {
				l.hitLimit = true
				l.diagnostic(source.Diagnostic{
					Range:    span,
					Severity: source.SeverityError,
					Code:     source.CodeLimitExceeded,
					Message:  "TeX token limit exceeded",
				})
			}
			return l.makeToken(tokEOF, "", span)
		}

		start := l.pos
		b := l.src[l.pos]
		if isTeXSpace(b) {
			for l.pos < len(l.src) && isTeXSpace(l.src[l.pos]) {
				l.pos++
			}
			return l.counted(tokSpace, " ", start, l.pos)
		}
		if b == '%' {
			l.pos++
			for l.pos < len(l.src) && l.src[l.pos] != '\n' {
				l.pos++
			}
			if l.pos < len(l.src) {
				l.pos++
			}
			continue
		}
		if b == '\\' {
			return l.lexControl()
		}

		r, size := utf8.DecodeRuneInString(l.src[l.pos:])
		if r == utf8.RuneError && size == 1 {
			l.pos++
			span := l.rangeFor(start, l.pos)
			l.diagnostic(source.Diagnostic{
				Range:    span,
				Severity: source.SeverityError,
				Code:     source.CodeSyntax,
				Message:  "invalid UTF-8 byte in TeX source",
			})
			return l.counted(tokError, "\uFFFD", start, l.pos)
		}
		l.pos += size
		switch r {
		case '{':
			return l.counted(tokBeginGroup, "{", start, l.pos)
		case '}':
			return l.counted(tokEndGroup, "}", start, l.pos)
		case '^':
			return l.counted(tokSuperscript, "^", start, l.pos)
		case '_':
			return l.counted(tokSubscript, "_", start, l.pos)
		case '&':
			return l.counted(tokAlignment, "&", start, l.pos)
		case '#':
			return l.counted(tokParameter, "#", start, l.pos)
		default:
			if unicode.IsControl(r) {
				span := l.rangeFor(start, l.pos)
				l.diagnostic(source.Diagnostic{
					Range:    span,
					Severity: source.SeverityError,
					Code:     source.CodeSyntax,
					Message:  "control character is not allowed in TeX source",
				})
				return l.counted(tokError, "\uFFFD", start, l.pos)
			}
			return l.counted(tokChar, l.src[start:l.pos], start, l.pos)
		}
	}

	return l.makeToken(tokEOF, "", l.rangeFor(l.pos, l.pos))
}

func (l *lexer) lexControl() token {
	start := l.pos
	l.pos++ // backslash
	if l.pos >= len(l.src) {
		span := l.rangeFor(start, l.pos)
		l.diagnostic(source.Diagnostic{
			Range:    span,
			Severity: source.SeverityError,
			Code:     source.CodeSyntax,
			Message:  "bare backslash at end of input",
		})
		return l.counted(tokError, "\\", start, l.pos)
	}

	b := l.src[l.pos]
	if isASCIILetter(b) || b == '@' {
		for l.pos < len(l.src) {
			c := l.src[l.pos]
			if !isASCIILetter(c) && c != '@' {
				break
			}
			l.pos++
		}
		span := l.rangeFor(start, l.pos)
		t := l.makeToken(tokControlWord, l.src[start:l.pos], span)
		// TeX control words consume following whitespace.
		for l.pos < len(l.src) && isTeXSpace(l.src[l.pos]) {
			l.pos++
		}
		return t
	}

	r, size := utf8.DecodeRuneInString(l.src[l.pos:])
	if r == utf8.RuneError && size == 1 {
		l.pos++
		span := l.rangeFor(start, l.pos)
		l.diagnostic(source.Diagnostic{
			Range:    span,
			Severity: source.SeverityError,
			Code:     source.CodeSyntax,
			Message:  "invalid UTF-8 after control sequence introducer",
		})
		return l.counted(tokError, "\\\uFFFD", start, l.pos)
	}
	l.pos += size
	if isTeXSpaceByteString(l.src[l.pos-size : l.pos]) {
		return l.counted(tokControlSymbol, "\\ ", start, l.pos)
	}
	return l.counted(tokControlSymbol, l.src[start:l.pos], start, l.pos)
}

func (l *lexer) counted(kind tokenKind, text string, start, end int) token {
	l.count++
	return l.makeToken(kind, text, l.rangeFor(start, end))
}

func (l *lexer) makeToken(kind tokenKind, text string, span source.Range) token {
	if l.detached {
		return token{kind: kind, text: text, span: source.Detached(), origin: source.Detached()}
	}
	return token{kind: kind, text: text, span: span, origin: span}
}

func (l *lexer) rangeFor(start, end int) source.Range {
	if l.detached {
		return source.Detached()
	}
	return source.NewRange(start, end)
}

func isASCIILetter(b byte) bool {
	return b >= 'a' && b <= 'z' || b >= 'A' && b <= 'Z'
}

func isTeXSpace(b byte) bool {
	switch b {
	case ' ', '\t', '\r', '\n', '\f':
		return true
	default:
		return false
	}
}

func isTeXSpaceByteString(s string) bool {
	return len(s) == 1 && isTeXSpace(s[0])
}
