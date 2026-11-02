package conformance

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

// sourceFamilies extracts explicit command/environment names and a small set
// of lexical constructs useful for grouping regressions. It is intentionally a
// source scanner, not a second TeX parser.
func sourceFamilies(src string) (families, commands, environments []string) {
	seenFamily := make(map[string]struct{})
	seenCommand := make(map[string]struct{})
	seenEnvironment := make(map[string]struct{})
	addCommand := func(command string) {
		if command == "" {
			return
		}
		if _, ok := seenCommand[command]; ok {
			return
		}
		seenCommand[command] = struct{}{}
		commands = append(commands, command)
		seenFamily["command:"+command] = struct{}{}
	}
	addEnvironment := func(environment string) {
		if environment == "" {
			return
		}
		environment = strings.TrimSpace(environment)
		if environment == "" {
			return
		}
		if _, ok := seenEnvironment[environment]; ok {
			return
		}
		seenEnvironment[environment] = struct{}{}
		environments = append(environments, environment)
		seenFamily["environment:"+environment] = struct{}{}
	}

	for i := 0; i < len(src); {
		if src[i] != '\\' {
			i++
			continue
		}
		start := i
		i++
		if i == len(src) {
			addCommand(`\`)
			break
		}
		if isASCIILetter(src[i]) {
			for i < len(src) && isASCIILetter(src[i]) {
				i++
			}
		} else {
			_, size := utf8.DecodeRuneInString(src[i:])
			if size <= 0 {
				size = 1
			}
			i += size
		}
		command := src[start:i]
		addCommand(command)
		if command == `\begin` || command == `\end` {
			if name, end, ok := bracedName(src, i); ok {
				addEnvironment(name)
				i = end
			}
		}
	}

	if strings.Contains(src, "^") || strings.Contains(src, "_") {
		seenFamily["feature:scripts"] = struct{}{}
	}
	if strings.Contains(src, "&") {
		seenFamily["feature:alignment"] = struct{}{}
	}
	if strings.Contains(src, `\\`) {
		seenFamily["feature:row-break"] = struct{}{}
	}
	if strings.Contains(src, "{") || strings.Contains(src, "}") {
		seenFamily["feature:group"] = struct{}{}
	}
	if len(seenFamily) == 0 {
		seenFamily["literal"] = struct{}{}
	}
	families = make([]string, 0, len(seenFamily))
	for family := range seenFamily {
		families = append(families, family)
	}
	return families, commands, environments
}

func bracedName(src string, offset int) (string, int, bool) {
	if offset >= len(src) || src[offset] != '{' {
		return "", offset, false
	}
	end := strings.IndexByte(src[offset+1:], '}')
	if end < 0 {
		return "", offset, false
	}
	end += offset + 1
	return src[offset+1 : end], end + 1, true
}

func isASCIILetter(b byte) bool {
	return b < utf8.RuneSelf && unicode.IsLetter(rune(b))
}

// typstSourceFamilies groups stable Typst constructs without trying to parse
// or evaluate source. Comments and strings are skipped so prose does not create
// misleading function families.
func typstSourceFamilies(src string) (families, commands, environments []string) {
	seen := make(map[string]struct{})
	add := func(family string) {
		if family != "" {
			seen[family] = struct{}{}
		}
	}
	for i := 0; i < len(src); {
		switch {
		case strings.HasPrefix(src[i:], "//"):
			if end := strings.IndexByte(src[i:], '\n'); end >= 0 {
				i += end + 1
			} else {
				i = len(src)
			}
			continue
		case strings.HasPrefix(src[i:], "/*"):
			end := strings.Index(src[i+2:], "*/")
			if end < 0 {
				i = len(src)
			} else {
				i += end + 4
			}
			continue
		case src[i] == '"':
			add("feature:text")
			i++
			for i < len(src) {
				if src[i] == '\\' {
					i += min(2, len(src)-i)
					continue
				}
				if src[i] == '"' {
					i++
					break
				}
				i++
			}
			continue
		}

		switch src[i] {
		case '^', '_':
			add("feature:scripts")
			i++
			continue
		case '&':
			add("feature:alignment")
			i++
			continue
		case '\\':
			add("feature:linebreak")
			i++
			continue
		case '/':
			add("feature:fraction")
			i++
			continue
		case '#':
			add("feature:code")
			i++
			continue
		case '(', '[', '{':
			add("feature:group")
		}

		r, size := utf8.DecodeRuneInString(src[i:])
		if r == utf8.RuneError && size == 1 {
			add("feature:invalid-utf8")
			i++
			continue
		}
		if !unicode.IsLetter(r) {
			i += size
			continue
		}
		start := i
		i += size
		for i < len(src) {
			r, size = utf8.DecodeRuneInString(src[i:])
			if !unicode.IsLetter(r) && !unicode.IsDigit(r) && r != '.' {
				break
			}
			i += size
		}
		name := src[start:i]
		j := i
		for j < len(src) && isTypstSpace(src[j]) {
			j++
		}
		if j < len(src) && src[j] == '(' {
			if isTypstFunction(name) {
				add("function:" + name)
				commands = append(commands, name)
			} else {
				add("call:" + name)
			}
			switch name {
			case "frac", "binom":
				add("feature:fraction")
			case "sqrt", "root":
				add("feature:root")
			case "mat", "vec", "cases":
				add("feature:matrix")
			case "lr", "abs", "norm", "floor", "ceil", "round":
				add("feature:fence")
			case "attach", "scripts", "limits", "primes":
				add("feature:scripts")
			case "text", "box":
				add("feature:text")
			}
		} else if strings.ContainsRune(name, '.') {
			add("field:" + name)
		}
	}
	if len(seen) == 0 {
		add("literal")
	}
	families = make([]string, 0, len(seen))
	for family := range seen {
		families = append(families, family)
	}
	commands = uniqueStrings(commands)
	return families, commands, environments
}

func isTypstSpace(b byte) bool {
	switch b {
	case ' ', '\t', '\n', '\r':
		return true
	default:
		return false
	}
}

func uniqueStrings(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	out := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	return out
}

func isTypstFunction(name string) bool {
	switch name {
	case "abs", "accent", "arrow", "attach", "bb", "binom", "bold", "box",
		"breve", "cal", "cancel", "caron", "cases", "ceil", "circle", "class",
		"display", "dot", "dot.double", "dot.triple", "floor", "frac", "frak", "h",
		"hat", "hide", "inline", "italic", "limits", "lr", "macron", "mat",
		"math.attach", "math.frac", "mid", "mono", "norm", "not", "op",
		"overbrace", "overbracket", "overline", "overparen", "primes", "root",
		"round", "sans", "scr", "script", "scripts", "serif", "sqrt", "sscript",
		"stretch", "text", "tilde", "upright", "underbrace", "underbracket",
		"underline", "underparen", "vec":
		return true
	default:
		return false
	}
}
