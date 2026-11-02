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
