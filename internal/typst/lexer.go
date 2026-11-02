package typst

import (
	"fmt"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/caelyreth/untie/source"
)

type tokenKind uint8

const (
	tokEOF tokenKind = iota
	tokIdent
	tokNumber
	tokInvalidNumber
	tokString
	tokShorthand
	tokGlyph
	tokHash
	tokCaret
	tokUnderscore
	tokSlash
	tokPrime
	tokAmp
	tokComma
	tokSemicolon
	tokColon
	tokLParen
	tokRParen
	tokLBracket
	tokRBracket
	tokLBrace
	tokRBrace
	tokPipe
	tokDollar
	tokLinebreak
	tokSpread
	tokBang
)

type token struct {
	kind        tokenKind
	span        source.Range
	text        string
	literal     string
	spaceBefore bool
}

func (t token) describe() string {
	switch t.kind {
	case tokEOF:
		return "end of input"
	case tokIdent:
		return fmt.Sprintf("identifier %q", t.text)
	case tokNumber:
		return fmt.Sprintf("number %q", t.text)
	case tokInvalidNumber:
		return fmt.Sprintf("invalid number %q", t.text)
	case tokString:
		return "string"
	default:
		if t.text != "" {
			return fmt.Sprintf("%q", t.text)
		}
		return "token"
	}
}

type lexer struct {
	src    string
	pos    int
	limits source.Limits
	count  int
	diags  []source.Diagnostic
}

// mathShorthands is ordered longest-first so prefix forms such as -> and --> are
// not split into unrelated tokens.
var mathShorthands = []struct {
	source string
	text   string
}{
	{"...", "…"},
	{"<=>", "⇔"},
	{"<==>", "⟺"},
	{"==>", "⟹"},
	{"<==", "⟸"},
	{"-->", "⟶"},
	{"<--", "⟵"},
	{"<->", "↔"},
	{"<-->", "⟷"},
	{"->>", "↠"},
	{"|->", "↦"},
	{">->", "↣"},
	{"<-<", "↢"},
	{"<<-", "↞"},
	{"~>", "⇝"},
	{"~~>", "⟿"},
	{"<~", "⇜"},
	{"<~~", "⬳"},
	{"=>", "⇒"},
	{"|=>", "⤇"},
	{"::=", "⩴"},
	{"<<<", "⋘"},
	{">>>", "⋙"},
	{"!=", "≠"},
	{":=", "≔"},
	{"=:", "≕"},
	{"<<", "≪"},
	{">>", "≫"},
	{"<=", "≤"},
	{">=", "≥"},
	{"->", "→"},
	{"<-", "←"},
	{"||", "‖"},
	{"[|", "⟦"},
	{"|]", "⟧"},
	{"-", "−"},
	{"*", "∗"},
	{"~", "∼"},
}

func lex(src string, limits source.Limits) ([]token, []source.Diagnostic) {
	l := lexer{src: src, limits: limits}
	tokens := make([]token, 0, min(len(src)/2+1, limits.MaxTokens))
	for {
		tok := l.next()
		tokens = append(tokens, tok)
		if tok.kind == tokEOF {
			break
		}
	}
	return tokens, l.diags
}

func (l *lexer) next() token {
	space := false
	for l.pos < len(l.src) {
		if isSpace(l.src[l.pos]) {
			space = true
			l.pos++
			continue
		}
		if strings.HasPrefix(l.src[l.pos:], "//") {
			space = true
			for l.pos < len(l.src) && l.src[l.pos] != '\n' {
				l.pos++
			}
			continue
		}
		if strings.HasPrefix(l.src[l.pos:], "/*") {
			space = true
			l.skipBlockComment()
			continue
		}
		break
	}

	start := l.pos
	if start >= len(l.src) {
		return token{kind: tokEOF, span: source.NewRange(start, start), spaceBefore: space}
	}
	if l.count++; l.count > l.limits.MaxTokens {
		l.diags = append(l.diags, source.Diagnostic{
			Range:    source.NewRange(start, min(start+1, len(l.src))),
			Severity: source.SeverityError,
			Code:     source.CodeLimitExceeded,
			Message:  "token limit exceeded",
		})
		l.pos = len(l.src)
		return token{kind: tokEOF, span: source.NewRange(start, start), spaceBefore: space}
	}

	// A two-dot spread is recognized before the three-dot shorthand.
	if strings.HasPrefix(l.src[start:], "..") && !strings.HasPrefix(l.src[start:], "...") {
		l.pos += 2
		return l.tok(tokSpread, start, space)
	}
	for _, shorthand := range mathShorthands {
		if strings.HasPrefix(l.src[start:], shorthand.source) {
			l.pos += len(shorthand.source)
			tok := l.tok(tokShorthand, start, space)
			tok.literal = shorthand.text
			return tok
		}
	}

	c := l.src[start]
	switch c {
	case '#':
		l.pos++
		return l.tok(tokHash, start, space)
	case '^':
		l.pos++
		return l.tok(tokCaret, start, space)
	case '_':
		l.pos++
		return l.tok(tokUnderscore, start, space)
	case '/':
		l.pos++
		return l.tok(tokSlash, start, space)
	case '\'':
		for l.pos < len(l.src) && l.src[l.pos] == '\'' {
			l.pos++
		}
		return l.tok(tokPrime, start, space)
	case '&':
		l.pos++
		return l.tok(tokAmp, start, space)
	case ',':
		l.pos++
		return l.tok(tokComma, start, space)
	case ';':
		l.pos++
		return l.tok(tokSemicolon, start, space)
	case ':':
		l.pos++
		return l.tok(tokColon, start, space)
	case '(':
		l.pos++
		return l.tok(tokLParen, start, space)
	case ')':
		l.pos++
		return l.tok(tokRParen, start, space)
	case '[':
		l.pos++
		return l.tok(tokLBracket, start, space)
	case ']':
		l.pos++
		return l.tok(tokRBracket, start, space)
	case '{':
		l.pos++
		return l.tok(tokLBrace, start, space)
	case '}':
		l.pos++
		return l.tok(tokRBrace, start, space)
	case '|':
		l.pos++
		return l.tok(tokPipe, start, space)
	case '$':
		l.pos++
		return l.tok(tokDollar, start, space)
	case '!':
		l.pos++
		return l.tok(tokBang, start, space)
	case '"':
		return l.stringToken(start, space)
	case '\\':
		return l.backslashToken(start, space)
	}

	r, size := utf8.DecodeRuneInString(l.src[start:])
	if r == utf8.RuneError && size == 1 {
		l.pos++
		l.invalidUTF8(start)
		return token{kind: tokGlyph, span: source.NewRange(start, l.pos), text: string(utf8.RuneError), spaceBefore: space}
	}
	if r >= '0' && r <= '9' {
		return l.numberToken(start, space)
	}
	if isIdentStart(r) {
		l.pos += size
		for l.pos < len(l.src) {
			r, size = utf8.DecodeRuneInString(l.src[l.pos:])
			if !isIdentContinue(r) {
				break
			}
			l.pos += size
		}
		return l.tok(tokIdent, start, space)
	}

	l.pos += size
	return token{kind: tokGlyph, span: source.NewRange(start, l.pos), text: string(r), spaceBefore: space}
}

func (l *lexer) tok(kind tokenKind, start int, space bool) token {
	return token{
		kind:        kind,
		span:        source.NewRange(start, l.pos),
		text:        l.src[start:l.pos],
		spaceBefore: space,
	}
}

func (l *lexer) numberToken(start int, space bool) token {
	for l.pos < len(l.src) && l.src[l.pos] >= '0' && l.src[l.pos] <= '9' {
		l.pos++
	}
	if l.pos < len(l.src) && l.src[l.pos] == '.' &&
		l.pos+1 < len(l.src) && l.src[l.pos+1] >= '0' && l.src[l.pos+1] <= '9' {
		l.pos++
		for l.pos < len(l.src) && l.src[l.pos] >= '0' && l.src[l.pos] <= '9' {
			l.pos++
		}
	}
	if unit, end := numberSuffixAt(l.src, l.pos); end >= 0 && (unit == "em" || unit == "ex") {
		// `em` and `ex` are suffixes, not the start of a malformed exponent.
		l.pos = end
	}

	exponentStart := -1
	exponentDigits := false
	if l.pos < len(l.src) && (l.src[l.pos] == 'e' || l.src[l.pos] == 'E') {
		exponentStart = l.pos
		l.pos++
		if l.pos < len(l.src) && (l.src[l.pos] == '+' || l.src[l.pos] == '-') {
			l.pos++
		}
		digitsStart := l.pos
		for l.pos < len(l.src) && l.src[l.pos] >= '0' && l.src[l.pos] <= '9' {
			l.pos++
		}
		exponentDigits = l.pos > digitsStart
	}
	numberEnd := l.pos
	if _, end := numberSuffixAt(l.src, l.pos); end >= 0 {
		l.pos = end
	}

	kind := tokNumber
	if exponentStart >= 0 {
		number := l.src[start:numberEnd]
		switch {
		case !exponentDigits:
			kind = tokInvalidNumber
			l.diags = append(l.diags, source.Diagnostic{
				Range:    source.NewRange(start, numberEnd),
				Severity: source.SeverityError,
				Code:     source.CodeSyntax,
				Message:  fmt.Sprintf("invalid floating point number: %q", number),
			})
		case !isFiniteFloatLiteral(number):
			kind = tokInvalidNumber
			l.diags = append(l.diags, source.Diagnostic{
				Range:    source.NewRange(start, numberEnd),
				Severity: source.SeverityError,
				Code:     source.CodeInvalidValue,
				Message:  fmt.Sprintf("number is not finite: %q", number),
			})
		}
	}
	return l.tok(kind, start, space)
}

func numberSuffixAt(src string, pos int) (string, int) {
	for _, unit := range [...]string{"em", "ex", "pt", "mm", "cm", "in"} {
		if !strings.HasPrefix(src[pos:], unit) {
			continue
		}
		end := pos + len(unit)
		next, _ := utf8.DecodeRuneInString(src[end:])
		if end == len(src) || !isIdentContinue(next) {
			return unit, end
		}
	}
	return "", -1
}

func isFiniteFloatLiteral(text string) bool {
	v, ok := parseNumericLiteral(text)
	return ok && v.kind == valueFloat
}

func (l *lexer) invalidUTF8(start int) {
	l.diags = append(l.diags, source.Diagnostic{
		Range:    source.NewRange(start, start+1),
		Severity: source.SeverityError,
		Code:     source.CodeSyntax,
		Message:  "invalid UTF-8 byte",
	})
}

func (l *lexer) stringToken(start int, space bool) token {
	l.pos++ // opening quote
	var decoded strings.Builder
	for l.pos < len(l.src) {
		if l.src[l.pos] == '"' {
			l.pos++
			return token{
				kind:        tokString,
				span:        source.NewRange(start, l.pos),
				text:        l.src[start:l.pos],
				literal:     decoded.String(),
				spaceBefore: space,
			}
		}
		if l.src[l.pos] != '\\' {
			r, size := utf8.DecodeRuneInString(l.src[l.pos:])
			if r == utf8.RuneError && size == 1 {
				l.invalidUTF8(l.pos)
				l.pos++
				continue
			}
			decoded.WriteRune(r)
			l.pos += size
			continue
		}
		escaped, emit, ok := l.scanEscape()
		if !ok {
			break
		}
		if emit {
			decoded.WriteRune(escaped)
		}
	}
	l.diags = append(l.diags, source.Diagnostic{
		Range:    source.NewRange(start, l.pos),
		Severity: source.SeverityError,
		Code:     source.CodeUnclosedGroup,
		Message:  "unclosed string",
	})
	return token{
		kind:        tokString,
		span:        source.NewRange(start, l.pos),
		text:        l.src[start:l.pos],
		literal:     decoded.String(),
		spaceBefore: space,
	}
}

func (l *lexer) backslashToken(start int, space bool) token {
	if strings.HasPrefix(l.src[start:], `\u{`) {
		l.pos = start + 3
		for l.pos < len(l.src) && l.src[l.pos] != '}' {
			l.pos++
		}
		if l.pos >= len(l.src) {
			l.diags = append(l.diags, source.Diagnostic{
				Range:    source.NewRange(start, l.pos),
				Severity: source.SeverityError,
				Code:     source.CodeUnclosedGroup,
				Message:  "unclosed Unicode escape sequence",
			})
			return token{kind: tokGlyph, span: source.NewRange(start, l.pos), text: string(utf8.RuneError), spaceBefore: space}
		}
		l.pos++
		parsed := l.parseUnicodeEscape(start)
		return token{kind: tokGlyph, span: source.NewRange(start, l.pos), text: string(parsed), literal: string(parsed), spaceBefore: space}
	}
	if start+1 >= len(l.src) || isSpace(l.src[start+1]) {
		l.pos++
		return l.tok(tokLinebreak, start, space)
	}
	l.pos = start + 1
	r, size := utf8.DecodeRuneInString(l.src[l.pos:])
	l.pos += size
	return token{kind: tokGlyph, span: source.NewRange(start, l.pos), text: string(r), literal: string(r), spaceBefore: space}
}

func (l *lexer) scanEscape() (rune, bool, bool) {
	start := l.pos
	if strings.HasPrefix(l.src[start:], `\u{`) {
		l.pos += 3
		for l.pos < len(l.src) && l.src[l.pos] != '}' {
			l.pos++
		}
		if l.pos >= len(l.src) {
			l.diags = append(l.diags, source.Diagnostic{
				Range:    source.NewRange(start, l.pos),
				Severity: source.SeverityError,
				Code:     source.CodeUnclosedGroup,
				Message:  "unclosed Unicode escape sequence",
			})
			return utf8.RuneError, true, true
		}
		l.pos++
		return l.parseUnicodeEscape(start), true, true
	}
	l.pos++
	if l.pos >= len(l.src) {
		return 0, false, false
	}
	r, size := utf8.DecodeRuneInString(l.src[l.pos:])
	if r == utf8.RuneError && size == 1 {
		l.invalidUTF8(l.pos)
		l.pos++
		return 0, false, true
	}
	l.pos += size
	switch r {
	case 'n':
		return '\n', true, true
	case 'r':
		return '\r', true, true
	case 't':
		return '\t', true, true
	case '\\':
		return '\\', true, true
	case '"':
		return '"', true, true
	default:
		return r, true, true
	}
}

func (l *lexer) parseUnicodeEscape(start int) rune {
	end := l.pos - 1
	if end < start+3 || end > len(l.src) {
		return utf8.RuneError
	}
	value, err := strconv.ParseUint(l.src[start+3:end], 16, 32)
	if err != nil || !utf8.ValidRune(rune(value)) {
		l.diags = append(l.diags, source.Diagnostic{
			Range:    source.NewRange(start, l.pos),
			Severity: source.SeverityError,
			Code:     source.CodeSyntax,
			Message:  "invalid Unicode codepoint",
		})
		return utf8.RuneError
	}
	return rune(value)
}

func (l *lexer) skipBlockComment() {
	start := l.pos
	l.pos += 2
	depth := 1
	for l.pos < len(l.src) && depth > 0 {
		switch {
		case strings.HasPrefix(l.src[l.pos:], "/*"):
			depth++
			l.pos += 2
		case strings.HasPrefix(l.src[l.pos:], "*/"):
			depth--
			l.pos += 2
		default:
			_, size := utf8.DecodeRuneInString(l.src[l.pos:])
			l.pos += size
		}
	}
	if depth != 0 {
		l.diags = append(l.diags, source.Diagnostic{
			Range:    source.NewRange(start, l.pos),
			Severity: source.SeverityError,
			Code:     source.CodeUnclosedGroup,
			Message:  "unclosed block comment",
		})
	}
}

func isSpace(b byte) bool {
	return b == ' ' || b == '\t' || b == '\n' || b == '\r' || b == '\v' || b == '\f'
}

func isIdentStart(r rune) bool {
	return unicode.IsLetter(r) || r == '_'
}

func isIdentContinue(r rune) bool {
	// In math mode, underscore starts an attachment and never continues an
	// identifier. Typst excludes it from math identifiers for this reason.
	return unicode.IsLetter(r) || unicode.IsDigit(r) || unicode.IsMark(r)
}
