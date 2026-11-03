// Command gen generates Untie's checked-in math metadata.
//
// The generator reads only pinned static files. It never compiles or executes
// KaTeX, Typst, Typlet, or any JavaScript/Rust toolchain.
package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"flag"
	"fmt"
	"go/format"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"
)

const (
	katexSymbolsPath    = "KaTeX/src/symbols.ts"
	katexOpsPath        = "KaTeX/src/functions/op.ts"
	katexMacrosPath     = "KaTeX/src/macros.ts"
	typstSyntaxPath     = "typst/crates/typst-syntax/src/ast.rs"
	typstOpsPath        = "typst/crates/typst-library/src/math/op.rs"
	typletClassRunsPath = "Typlet/src/generated/math-class.ts"
	typletClassPath     = "Typlet/src/utils/math-class.ts"
	typletStylePath     = "Typlet/src/utils/styling.ts"
	typletSymbolPath    = "Typlet/src/generated/symbols.ts"
)

type sourceRef struct {
	path string
	sum  string
}

type classRange struct {
	start rune
	end   rune
	class string
}

type classOverride struct {
	r    rune
	name string
}

type variantInfo struct {
	style    string
	latin    rune
	greek    rune
	digits   rune
	selector rune
}

type variantException struct {
	style  string
	input  rune
	output rune
}

type greekExtra struct {
	input  rune
	offset rune
}

type texSymbol struct {
	command       string
	text          string
	class         string
	group         string
	font          string
	acceptUnicode bool
	kind          string
	sourceGlyph   rune
}

type typstSymbol struct {
	path        string
	name        string
	modifier    string
	value       string
	deprecation string
}

type negationPair struct {
	base    string
	negated string
}

var negatedRelationBaseAliases = map[string]string{
	// Typlet exposes the relation form as `tilde.op`, not a bare `tilde` path.
	"tilde.not": "tilde.op",
}

type shorthand struct {
	syntax string
	value  string
}

type operator struct {
	name   string
	text   string
	limits bool
	symbol bool
}

const variantBoldCalligraphic = 15

func main() {
	sources := flag.String("sources", "", "path to the pinned source tree")
	out := flag.String("out", "", "output directory for generated Go files")
	flag.Parse()
	if *sources == "" || *out == "" {
		fmt.Fprintln(os.Stderr, "mathdata gen: -sources and -out are required")
		os.Exit(2)
	}
	if err := run(*sources, *out); err != nil {
		fmt.Fprintln(os.Stderr, "mathdata gen:", err)
		os.Exit(1)
	}
}

func run(sources, out string) error {
	read := func(rel string) (string, sourceRef, error) {
		data, err := os.ReadFile(filepath.Join(sources, rel))
		if err != nil {
			return "", sourceRef{}, fmt.Errorf("read %s: %w", rel, err)
		}
		sum := sha256.Sum256(data)
		return string(data), sourceRef{path: rel, sum: hex.EncodeToString(sum[:])}, nil
	}

	classRunsSrc, classRunsRef, err := read(typletClassRunsPath)
	if err != nil {
		return err
	}
	classSrc, classRef, err := read(typletClassPath)
	if err != nil {
		return err
	}
	styleSrc, styleRef, err := read(typletStylePath)
	if err != nil {
		return err
	}
	katexSymSrc, katexSymRef, err := read(katexSymbolsPath)
	if err != nil {
		return err
	}
	katexOpsSrc, katexOpsRef, err := read(katexOpsPath)
	if err != nil {
		return err
	}
	katexMacrosSrc, katexMacrosRef, err := read(katexMacrosPath)
	if err != nil {
		return err
	}
	typstSyntaxSrc, typstSyntaxRef, err := read(typstSyntaxPath)
	if err != nil {
		return err
	}
	typstOpsSrc, typstOpsRef, err := read(typstOpsPath)
	if err != nil {
		return err
	}
	typletSymSrc, typletSymRef, err := read(typletSymbolPath)
	if err != nil {
		return err
	}

	ranges, overrides, err := parseClasses(classRunsSrc, classSrc)
	if err != nil {
		return fmt.Errorf("%s, %s: %w", typletClassRunsPath, typletClassPath, err)
	}
	infos, exceptions, extras, err := parseVariants(styleSrc)
	if err != nil {
		return fmt.Errorf("%s: %w", typletStylePath, err)
	}
	fallbacks, err := parseKaTeXMacroFallbacks(katexMacrosSrc)
	if err != nil {
		return fmt.Errorf("%s: %w", katexMacrosPath, err)
	}
	symbols, err := parseTeXSymbols(katexSymSrc, fallbacks)
	if err != nil {
		return fmt.Errorf("%s: %w", katexSymbolsPath, err)
	}
	texOps, err := parseTeXOperators(katexOpsSrc)
	if err != nil {
		return fmt.Errorf("%s: %w", katexOpsPath, err)
	}
	shorthands, err := parseShorthands(typstSyntaxSrc)
	if err != nil {
		return fmt.Errorf("%s: %w", typstSyntaxPath, err)
	}
	typstOps, err := parseTypstOperators(typstOpsSrc)
	if err != nil {
		return fmt.Errorf("%s: %w", typstOpsPath, err)
	}
	tsymbols, err := parseTypstSymbols(typletSymSrc)
	if err != nil {
		return fmt.Errorf("%s: %w", typletSymbolPath, err)
	}
	negations, err := parseNegatedRelations(tsymbols)
	if err != nil {
		return fmt.Errorf("%s: %w", typletSymbolPath, err)
	}

	files := []struct {
		name    string
		sources []sourceRef
		body    string
	}{
		{
			name:    "class_generated.go",
			sources: []sourceRef{classRunsRef, classRef},
			body:    generateClasses(ranges, overrides),
		},
		{
			name:    "variant_generated.go",
			sources: []sourceRef{styleRef},
			body:    generateVariants(infos, exceptions, extras),
		},
		{
			name:    "tex_generated.go",
			sources: []sourceRef{katexSymRef, katexMacrosRef},
			body:    generateTeXSymbols(symbols),
		},
		{
			name:    "typst_generated.go",
			sources: []sourceRef{typletSymRef, typstSyntaxRef},
			body:    generateTypst(tsymbols, shorthands),
		},
		{
			name:    "negation_generated.go",
			sources: []sourceRef{typletSymRef},
			body:    generateNegatedRelations(negations),
		},
		{
			name:    "operator_generated.go",
			sources: []sourceRef{katexOpsRef, typstOpsRef},
			body:    generateOperators(texOps, typstOps),
		},
	}

	for _, file := range files {
		data := []byte(header(file.sources) + file.body)
		formatted, err := format.Source(data)
		if err != nil {
			return fmt.Errorf("format %s: %w", file.name, err)
		}
		if err := os.WriteFile(filepath.Join(out, file.name), formatted, 0o644); err != nil {
			return fmt.Errorf("write %s: %w", file.name, err)
		}
	}
	return nil
}

func header(sources []sourceRef) string {
	var b strings.Builder
	b.WriteString("// Code generated by internal/mathdata/gen; DO NOT EDIT.\n")
	b.WriteString("//\n")
	for _, source := range sources {
		fmt.Fprintf(&b, "// Source: %s\n// SHA-256: %s\n", source.path, source.sum)
	}
	b.WriteString("// Provenance: PROVENANCE.md.\n\npackage mathdata\n\n")
	return b.String()
}

func parseClasses(runSrc, overrideSrc string) ([]classRange, []classOverride, error) {
	body, err := matchBody(runSrc, `MATH_CLASS_RUNS:\s*readonly number\[\]\s*=\s*\[`, `\]`)
	if err != nil {
		return nil, nil, err
	}
	parts := strings.Split(body, ",")
	numbers := make([]int, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		n, err := strconv.Atoi(part)
		if err != nil {
			return nil, nil, fmt.Errorf("class run %q: %w", part, err)
		}
		numbers = append(numbers, n)
	}
	if len(numbers)%3 != 0 {
		return nil, nil, errors.New("class run array length is not divisible by three")
	}

	var ranges []classRange
	end := 0
	for i := 0; i < len(numbers); i += 3 {
		start := end + numbers[i]
		end = start + numbers[i+1]
		class := numbers[i+2]
		if class < 0 || class >= len(classNames) {
			return nil, nil, fmt.Errorf("class run %d has invalid class %d", i/3, class)
		}
		if end < start {
			return nil, nil, fmt.Errorf("class run %d has negative length", i/3)
		}
		if start == end {
			continue
		}
		ranges = append(ranges, classRange{start: rune(start), end: rune(end), class: className(class)})
	}
	for i := 1; i < len(ranges); i++ {
		if ranges[i-1].end > ranges[i].start {
			return nil, nil, fmt.Errorf("class ranges overlap at U+%04X", ranges[i].start)
		}
	}

	defaultAt := strings.Index(overrideSrc, "export function defaultMathClass")
	if defaultAt < 0 {
		return nil, nil, errors.New("defaultMathClass not found")
	}
	var pending []rune
	byRune := map[rune]string{}
	caseRE := regexp.MustCompile(`case 0x([0-9a-fA-F]+):`)
	returnRE := regexp.MustCompile(`return MathClass\.([A-Za-z]+);`)
	for _, line := range strings.Split(overrideSrc[defaultAt:], "\n") {
		if match := caseRE.FindStringSubmatch(line); match != nil {
			n, err := strconv.ParseUint(match[1], 16, 32)
			if err != nil {
				return nil, nil, err
			}
			pending = append(pending, rune(n))
		}
		if match := returnRE.FindStringSubmatch(line); match != nil {
			if _, ok := classNumber(match[1]); !ok {
				pending = nil
				continue
			}
			for _, r := range pending {
				if old, exists := byRune[r]; exists && old != match[1] {
					return nil, nil, fmt.Errorf("conflicting default classes for U+%04X", r)
				}
				byRune[r] = match[1]
			}
			pending = nil
		}
	}
	overrides := make([]classOverride, 0, len(byRune))
	for r, class := range byRune {
		overrides = append(overrides, classOverride{r: r, name: class})
	}
	sort.Slice(overrides, func(i, j int) bool { return overrides[i].r < overrides[j].r })
	return ranges, overrides, nil
}

var classNames = []string{
	"Normal", "Alphabetic", "Binary", "Closing", "Diacritic", "Fence",
	"GlyphPart", "Large", "Opening", "Punctuation", "Relation", "Space",
	"Unary", "Vary", "Special",
}

func className(n int) string {
	if n < 0 || n >= len(classNames) {
		panic("invalid class")
	}
	return classNames[n]
}

func classNumber(name string) (int, bool) {
	for i, candidate := range classNames {
		if candidate == name {
			return i, true
		}
	}
	return 0, false
}

func parseVariants(src string) ([]variantInfo, []variantException, []greekExtra, error) {
	alphabets, err := objectBody(src, "const ALPHABETS")
	if err != nil {
		return nil, nil, nil, err
	}
	script, err := objectBody(src, "const SCRIPT")
	if err != nil {
		return nil, nil, nil, err
	}
	boldScript, err := objectBody(src, "const BOLD_SCRIPT")
	if err != nil {
		return nil, nil, nil, err
	}
	extrasBody, err := objectBody(src, "const GREEK_EXTRAS")
	if err != nil {
		return nil, nil, nil, err
	}

	styles := []variantInfo{
		{style: "Plain"},
	}
	definitions := []struct {
		style    string
		key      string
		block    string
		selector rune
	}{
		{style: "Bold", key: "bold", block: alphabets},
		{style: "Italic", key: "italic", block: alphabets},
		{style: "BoldItalic", key: "bold-italic", block: alphabets},
		{style: "DoubleStruck", key: "double-struck", block: alphabets},
		{style: "Script", key: "roundhand", block: script, selector: 0xfe01},
		{style: "BoldScript", key: "bold-roundhand", block: boldScript, selector: 0xfe01},
		{style: "Fraktur", key: "fraktur", block: alphabets},
		{style: "BoldFraktur", key: "bold-fraktur", block: alphabets},
		{style: "SansSerif", key: "sans-serif", block: alphabets},
		{style: "SansSerifBold", key: "sans-serif-bold", block: alphabets},
		{style: "SansSerifItalic", key: "sans-serif-italic", block: alphabets},
		{style: "SansSerifBoldItalic", key: "sans-serif-bold-italic", block: alphabets},
		{style: "Monospace", key: "monospace", block: alphabets},
		{style: "Calligraphic", key: "chancery", block: script, selector: 0xfe00},
		{style: "BoldCalligraphic", key: "bold-chancery", block: boldScript, selector: 0xfe00},
	}

	var exceptions []variantException
	for _, def := range definitions {
		var block string
		if def.block == alphabets {
			block, err = namedObject(def.block, def.key)
		} else {
			block = def.block
		}
		if err != nil {
			return nil, nil, nil, fmt.Errorf("%s alphabet: %w", def.style, err)
		}
		info := variantInfo{
			style:    def.style,
			latin:    fieldRune(block, "latin"),
			greek:    fieldRune(block, "greek"),
			digits:   fieldRune(block, "digits"),
			selector: def.selector,
		}
		styles = append(styles, info)
		pairs := regexp.MustCompile(`0x([0-9a-fA-F]+)\s*:\s*0x([0-9a-fA-F]+)`)
		for _, match := range pairs.FindAllStringSubmatch(block, -1) {
			input, err := parseHexRune(match[1])
			if err != nil {
				return nil, nil, nil, err
			}
			output, err := parseHexRune(match[2])
			if err != nil {
				return nil, nil, nil, err
			}
			exceptions = append(exceptions, variantException{
				style: def.style, input: input, output: output,
			})
		}
	}
	if len(styles) != int(variantBoldCalligraphic)+1 {
		return nil, nil, nil, errors.New("variant count does not match public constants")
	}
	sort.Slice(exceptions, func(i, j int) bool {
		a, b := exceptions[i], exceptions[j]
		if variantNumber(a.style) != variantNumber(b.style) {
			return variantNumber(a.style) < variantNumber(b.style)
		}
		return a.input < b.input
	})
	for i := 1; i < len(exceptions); i++ {
		if exceptions[i-1].style == exceptions[i].style &&
			exceptions[i-1].input == exceptions[i].input {
			return nil, nil, nil, fmt.Errorf("duplicate %s exception U+%04X",
				exceptions[i].style, exceptions[i].input)
		}
	}

	var extras []greekExtra
	pairRE := regexp.MustCompile(`0x([0-9a-fA-F]+)\s*:\s*(\d+)`)
	for _, match := range pairRE.FindAllStringSubmatch(extrasBody, -1) {
		input, err := parseHexRune(match[1])
		if err != nil {
			return nil, nil, nil, err
		}
		offset, err := strconv.Atoi(match[2])
		if err != nil {
			return nil, nil, nil, err
		}
		extras = append(extras, greekExtra{input: input, offset: rune(offset)})
	}
	sort.Slice(extras, func(i, j int) bool { return extras[i].input < extras[j].input })
	if len(extras) == 0 {
		return nil, nil, nil, errors.New("no Greek variant extras found")
	}
	return styles, exceptions, extras, nil
}

func variantNumber(style string) int {
	names := []string{
		"Plain", "Bold", "Italic", "BoldItalic", "DoubleStruck", "Script",
		"BoldScript", "Fraktur", "BoldFraktur", "SansSerif", "SansSerifBold",
		"SansSerifItalic", "SansSerifBoldItalic", "Monospace", "Calligraphic",
		"BoldCalligraphic",
	}
	for i, name := range names {
		if name == style {
			return i
		}
	}
	panic("unknown variant " + style)
}

func fieldRune(block, name string) rune {
	re := regexp.MustCompile(`(?m)(?:^|,)\s*` + regexp.QuoteMeta(name) + `\s*:\s*0x([0-9a-fA-F]+)`)
	match := re.FindStringSubmatch(block)
	if match == nil {
		return 0
	}
	r, _ := parseHexRune(match[1])
	return r
}

func parseHexRune(hexText string) (rune, error) {
	n, err := strconv.ParseUint(hexText, 16, 32)
	if err != nil || !utf8.ValidRune(rune(n)) {
		return 0, fmt.Errorf("invalid Unicode code point 0x%s", hexText)
	}
	return rune(n), nil
}

func parseTeXSymbols(src string, fallbacks map[string]string) ([]texSymbol, error) {
	re := regexp.MustCompile(`(?m)^defineSymbol\(math,\s*(main|ams),\s*([a-z-]+),\s*("(?:\\.|[^"\\])*")\s*,\s*("(?:\\.|[^"\\])*")(?:\s*,\s*(true|false))?\s*\)\s*;`)
	var out []texSymbol
	for _, match := range re.FindAllStringSubmatch(src, -1) {
		text, err := strconv.Unquote(match[3])
		if err != nil {
			return nil, err
		}
		command, err := strconv.Unquote(match[4])
		if err != nil {
			return nil, err
		}
		if !strings.HasPrefix(command, `\`) {
			continue
		}
		gname := match[2]
		group, ok := texGroupName(gname)
		if !ok {
			return nil, fmt.Errorf("unknown symbol group %q for %s", gname, command)
		}
		kind := "Public"
		if strings.HasPrefix(command, `\@`) {
			kind = "Internal"
		}
		sourceGlyph, privateUse := privateUseGlyph(text)
		if privateUse {
			if kind != "Internal" {
				return nil, fmt.Errorf("public TeX symbol %s uses private-use text %U", command, sourceGlyph)
			}
			if utf8.RuneCountInString(text) != 1 {
				return nil, fmt.Errorf("TeX symbol %s has private-use text mixed with other runes", command)
			}
			if fallback, ok := fallbacks[command]; ok {
				text = fallback
			} else {
				text = ""
			}
		}
		if glyph, ok := privateUseGlyph(text); ok {
			return nil, fmt.Errorf("TeX symbol %s retains private-use text %U", command, glyph)
		}
		out = append(out, texSymbol{
			command:       command,
			text:          text,
			class:         teXClassForGroup(group),
			group:         group,
			font:          map[string]string{"main": "Main", "ams": "AMS"}[match[1]],
			acceptUnicode: match[5] == "true",
			kind:          kind,
			sourceGlyph:   sourceGlyph,
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].command < out[j].command })
	if len(out) == 0 {
		return nil, errors.New("no TeX symbols parsed")
	}
	for i := 1; i < len(out); i++ {
		if out[i-1].command == out[i].command {
			return nil, fmt.Errorf("duplicate TeX symbol %s", out[i].command)
		}
	}
	return out, nil
}

func privateUseGlyph(text string) (rune, bool) {
	for _, r := range text {
		if isPrivateUse(r) {
			return r, true
		}
	}
	return 0, false
}

func isPrivateUse(r rune) bool {
	return (r >= 0xE000 && r <= 0xF8FF) ||
		(r >= 0xF0000 && r <= 0xFFFFD) ||
		(r >= 0x100000 && r <= 0x10FFFD)
}

func parseKaTeXMacroFallbacks(src string) (map[string]string, error) {
	re := regexp.MustCompile(`(?m)^defineMacro\(\s*("(?:\\.|[^"\\])*")\s*,\s*("(?:\\.|[^"\\])*")\s*\)\s*;`)
	fallbacks := make(map[string]string)
	for _, match := range re.FindAllStringSubmatch(src, -1) {
		command, err := strconv.Unquote(match[1])
		if err != nil {
			return nil, err
		}
		body, err := strconv.Unquote(match[2])
		if err != nil {
			return nil, err
		}
		internal, alternative, ok, err := parseHTMLMathMLAlternative(body)
		if err != nil {
			return nil, fmt.Errorf("macro %s: %w", command, err)
		}
		if !ok || !strings.HasPrefix(internal, `\@`) {
			continue
		}
		if alternative == "" {
			return nil, fmt.Errorf("macro %s has an empty MathML alternative", command)
		}
		if glyph, ok := privateUseGlyph(alternative); ok {
			return nil, fmt.Errorf("macro %s alternative contains private-use text %U", command, glyph)
		}
		if previous, ok := fallbacks[internal]; ok && previous != alternative {
			return nil, fmt.Errorf("macro fallback for %s is both %q and %q", internal, previous, alternative)
		}
		fallbacks[internal] = alternative
	}
	if len(fallbacks) == 0 {
		return nil, errors.New("no KaTeX private-use macro fallbacks found")
	}
	return fallbacks, nil
}

func parseHTMLMathMLAlternative(body string) (internal, alternative string, ok bool, err error) {
	const prefix = `\html@mathml`
	if !strings.HasPrefix(body, prefix) {
		return "", "", false, nil
	}
	open := len(prefix)
	if open >= len(body) || body[open] != '{' {
		return "", "", false, fmt.Errorf("invalid html@mathml call")
	}
	internal, end, err := mathBraceGroupAt(body, open)
	if err != nil {
		return "", "", false, err
	}
	if end >= len(body) || body[end] != '{' {
		return "", "", false, fmt.Errorf("html@mathml call has no second argument")
	}
	alternative, end, err = mathBraceGroupAt(body, end)
	if err != nil {
		return "", "", false, err
	}
	if strings.TrimSpace(body[end:]) != "" {
		return "", "", false, fmt.Errorf("unexpected text after html@mathml call")
	}
	return internal, alternative, true, nil
}

func mathBraceGroupAt(src string, open int) (string, int, error) {
	if open < 0 || open >= len(src) || src[open] != '{' {
		return "", 0, errors.New("internal generator error: brace offset")
	}
	depth := 0
	escaped := false
	for i := open; i < len(src); i++ {
		if escaped {
			escaped = false
			continue
		}
		switch src[i] {
		case '\\':
			escaped = true
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return src[open+1 : i], i + 1, nil
			}
		}
	}
	return "", 0, errors.New("unclosed brace")
}

func texGroupName(name string) (string, bool) {
	names := map[string]string{
		"textord": "TextOrd",
		"mathord": "MathOrd",
		"op":      "Operator",
		"bin":     "Binary",
		"rel":     "Relation",
		"open":    "Opening",
		"close":   "Closing",
		"punct":   "Punctuation",
		"inner":   "Inner",
		"accent":  "Accent",
		"spacing": "Spacing",
	}
	value, ok := names[name]
	return value, ok
}

func teXClassForGroup(group string) string {
	switch group {
	case "MathOrd":
		return "Alphabetic"
	case "Operator":
		return "Large"
	case "Binary":
		return "Binary"
	case "Relation":
		return "Relation"
	case "Opening":
		return "Opening"
	case "Closing":
		return "Closing"
	case "Punctuation":
		return "Punctuation"
	case "Inner":
		return "Special"
	case "Accent":
		return "Diacritic"
	case "Spacing":
		return "Space"
	default:
		return "Normal"
	}
}

func parseTeXOperators(src string) ([]operator, error) {
	var out []operator
	for _, block := range functionBlocks(src) {
		if !strings.Contains(block, `type: "op"`) || !regexp.MustCompile(`numArgs:\s*0`).MatchString(block) {
			continue
		}
		namesBody, err := bracketBody(block, "names:")
		if err != nil {
			return nil, err
		}
		names, err := jsStrings(namesBody)
		if err != nil {
			return nil, err
		}
		handlerAt := strings.Index(block, "handler")
		if handlerAt < 0 {
			continue
		}
		handler := block[handlerAt:]
		limits, err := singleBoolField(handler, "limits")
		if err != nil {
			return nil, err
		}
		symbol, err := singleBoolField(handler, "symbol")
		if err != nil {
			return nil, err
		}
		for _, name := range names {
			if !strings.HasPrefix(name, `\`) || name == `\mathop` {
				continue
			}
			text := ""
			if !symbol {
				text = strings.TrimPrefix(name, `\`)
			}
			out = append(out, operator{name: name, text: text, limits: limits, symbol: symbol})
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].name < out[j].name })
	if len(out) == 0 {
		return nil, errors.New("no TeX operators parsed")
	}
	for i := 1; i < len(out); i++ {
		if out[i-1].name == out[i].name {
			return nil, fmt.Errorf("duplicate TeX operator %s", out[i].name)
		}
	}
	return out, nil
}

func parseTypstOperators(src string) ([]operator, error) {
	start := strings.Index(src, "ops! {")
	if start < 0 {
		return nil, errors.New("ops! block not found")
	}
	block, err := bracesAt(src, strings.Index(src[start:], "{")+start)
	if err != nil {
		return nil, err
	}
	lineRE := regexp.MustCompile(`^\s*([A-Za-z]+)(?:\s*:\s*"((?:\\.|[^"\\])*)")?(?:\s*\((limits)\))?,\s*$`)
	var out []operator
	for _, line := range strings.Split(block, "\n") {
		match := lineRE.FindStringSubmatch(line)
		if match == nil {
			continue
		}
		text := match[1]
		if match[2] != "" {
			text, err = decodeRustString(match[2])
			if err != nil {
				return nil, err
			}
		}
		out = append(out, operator{name: match[1], text: text, limits: match[3] == "limits"})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].name < out[j].name })
	if len(out) == 0 {
		return nil, errors.New("no Typst operators parsed")
	}
	for i := 1; i < len(out); i++ {
		if out[i-1].name == out[i].name {
			return nil, fmt.Errorf("duplicate Typst operator %s", out[i].name)
		}
	}
	return out, nil
}

func parseShorthands(src string) ([]shorthand, error) {
	impl := strings.Index(src, "impl MathShorthand")
	if impl < 0 {
		return nil, errors.New("MathShorthand implementation not found")
	}
	start := strings.Index(src[impl:], "pub const LIST")
	if start < 0 {
		return nil, errors.New("MathShorthand::LIST not found")
	}
	tail := src[impl+start:]
	equals := strings.Index(tail, "= &[")
	if equals < 0 {
		return nil, errors.New("MathShorthand::LIST array not found")
	}
	body, err := bracketsAt(tail, equals+3)
	if err != nil {
		return nil, err
	}
	lineRE := regexp.MustCompile(`\("((?:\\.|[^"\\])*)",\s*'((?:\\.|[^'\\])*)'\)`)
	var out []shorthand
	for _, match := range lineRE.FindAllStringSubmatch(body, -1) {
		syntax, err := decodeRustString(match[1])
		if err != nil {
			return nil, err
		}
		value, err := decodeRustChar(match[2])
		if err != nil {
			return nil, err
		}
		out = append(out, shorthand{syntax: syntax, value: string(value)})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].syntax < out[j].syntax })
	if len(out) == 0 {
		return nil, errors.New("no Typst shorthands parsed")
	}
	for i := 1; i < len(out); i++ {
		if out[i-1].syntax == out[i].syntax {
			return nil, fmt.Errorf("duplicate Typst shorthand %q", out[i].syntax)
		}
	}
	return out, nil
}

func parseTypstSymbols(src string) ([]typstSymbol, error) {
	literal, err := stringAfterAssignment(src, "export const SYMBOLS")
	if err != nil {
		return nil, err
	}
	decoded, err := strconv.Unquote(literal)
	if err != nil {
		return nil, fmt.Errorf("decode SYMBOLS: %w", err)
	}
	var out []typstSymbol
	index := map[string]int{}
	for _, line := range strings.Split(decoded, "\n") {
		fields := strings.Split(line, "\t")
		if len(fields) < 2 {
			return nil, fmt.Errorf("malformed symbol line %q", line)
		}
		name := fields[0]
		for i := 1; i < len(fields); i++ {
			key, value, ok := strings.Cut(fields[i], "=")
			if !ok {
				return nil, fmt.Errorf("malformed variant %q", fields[i])
			}
			modifier := key
			path := name
			if key != "" {
				path = name + "." + key
			} else {
				modifier = ""
			}
			if _, exists := index[path]; exists {
				return nil, fmt.Errorf("duplicate Typst symbol %s", path)
			}
			index[path] = len(out)
			out = append(out, typstSymbol{
				path: path, name: name, modifier: modifier, value: value,
			})
		}
	}

	variantBody, err := arrayAfterMarker(src, "VARIANT_DEPRECATIONS")
	if err != nil {
		return nil, err
	}
	variantStrings, err := jsStrings(variantBody)
	if err != nil {
		return nil, err
	}
	if len(variantStrings)%3 != 0 {
		return nil, errors.New("VARIANT_DEPRECATIONS is not a flat triple array")
	}
	for i := 0; i < len(variantStrings); i += 3 {
		path := variantStrings[i] + "." + variantStrings[i+1]
		at, ok := index[path]
		if !ok {
			return nil, fmt.Errorf("deprecated symbol %s not found", path)
		}
		out[at].deprecation = variantStrings[i+2]
	}

	bindingBody, err := arrayAfterMarker(src, "BINDING_DEPRECATIONS")
	if err != nil {
		return nil, err
	}
	bindingStrings, err := jsStrings(bindingBody)
	if err != nil {
		return nil, err
	}
	if len(bindingStrings)%2 != 0 {
		return nil, errors.New("BINDING_DEPRECATIONS is not a flat pair array")
	}
	for i := 0; i < len(bindingStrings); i += 2 {
		at, ok := index[bindingStrings[i]]
		if !ok {
			return nil, fmt.Errorf("deprecated binding %s not found", bindingStrings[i])
		}
		out[at].deprecation = bindingStrings[i+1]
	}

	sort.Slice(out, func(i, j int) bool { return out[i].path < out[j].path })
	return out, nil
}

func generateClasses(ranges []classRange, overrides []classOverride) string {
	var b strings.Builder
	b.WriteString("type classRange struct {\n\tstart rune\n\tend   rune\n\tclass Class\n}\n\n")
	b.WriteString("var unicodeClassRanges = [...]classRange{\n")
	for _, r := range ranges {
		fmt.Fprintf(&b, "\t{start: 0x%04x, end: 0x%04x, class: Class%s},\n", r.start, r.end, r.class)
	}
	b.WriteString("}\n\n")
	b.WriteString("type classOverride struct {\n\trune  rune\n\tclass Class\n}\n\n")
	b.WriteString("var defaultClassOverrides = [...]classOverride{\n")
	for _, override := range overrides {
		fmt.Fprintf(&b, "\t{rune: 0x%04x, class: Class%s},\n", override.r, override.name)
	}
	b.WriteString("}\n")
	return b.String()
}

func generateVariants(infos []variantInfo, exceptions []variantException, extras []greekExtra) string {
	var b strings.Builder
	b.WriteString("type alphaVariantInfo struct {\n\tlatin    rune\n\tgreek    rune\n\tdigits   rune\n\tselector rune\n}\n\n")
	b.WriteString("// Indexed by AlphaVariant.\nvar alphaVariantData = [...]alphaVariantInfo{\n")
	for _, info := range infos {
		fmt.Fprintf(&b, "\t0x%04x: {latin: 0x%04x, greek: 0x%04x, digits: 0x%04x, selector: 0x%04x}, // %s\n",
			variantNumber(info.style), info.latin, info.greek, info.digits, info.selector, info.style)
	}
	b.WriteString("}\n\n")
	b.WriteString("type alphaVariantException struct {\n\tvariant AlphaVariant\n\tinput   rune\n\toutput  rune\n}\n\n")
	b.WriteString("var alphaVariantExceptions = [...]alphaVariantException{\n")
	for _, exception := range exceptions {
		fmt.Fprintf(&b, "\t{variant: AlphaVariant(%d), input: 0x%04x, output: 0x%04x}, // %s\n",
			variantNumber(exception.style), exception.input, exception.output, exception.style)
	}
	b.WriteString("}\n\n")
	b.WriteString("type greekVariantExtra struct {\n\tinput  rune\n\toffset rune\n}\n\n")
	b.WriteString("var greekVariantExtras = [...]greekVariantExtra{\n")
	for _, extra := range extras {
		fmt.Fprintf(&b, "\t{input: 0x%04x, offset: %d},\n", extra.input, extra.offset)
	}
	b.WriteString("}\n")
	return b.String()
}

func parseNegatedRelations(symbols []typstSymbol) ([]negationPair, error) {
	byPath := make(map[string]typstSymbol, len(symbols))
	for _, symbol := range symbols {
		byPath[symbol.path] = symbol
	}
	byBase := make(map[string]string)
	for _, symbol := range symbols {
		if !strings.HasSuffix(symbol.path, ".not") {
			continue
		}
		basePath := strings.TrimSuffix(symbol.path, ".not")
		if alias, ok := negatedRelationBaseAliases[symbol.path]; ok {
			basePath = alias
		}
		base, ok := byPath[basePath]
		if !ok {
			continue
		}
		if utf8.RuneCountInString(base.value) != 1 || utf8.RuneCountInString(symbol.value) != 1 {
			continue
		}
		if base.value == symbol.value {
			continue
		}
		if previous, ok := byBase[base.value]; ok && previous != symbol.value {
			return nil, fmt.Errorf("negated relation %q is both %q and %q", base.value, previous, symbol.value)
		}
		byBase[base.value] = symbol.value
	}
	if len(byBase) == 0 {
		return nil, errors.New("no Typst negated relations found")
	}
	out := make([]negationPair, 0, len(byBase))
	for base, negated := range byBase {
		out = append(out, negationPair{base: base, negated: negated})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].base < out[j].base })
	return out, nil
}

func generateTeXSymbols(symbols []texSymbol) string {
	var b strings.Builder
	b.WriteString("var texSymbols = [...]TeXSymbol{\n")
	for _, symbol := range symbols {
		fmt.Fprintf(&b,
			"\t{Command: %s, Text: %s, Class: Class%s, Group: TeXGroup%s, Font: TeXFont%s, AcceptUnicode: %t, Kind: TeXSymbol%s, SourceGlyph: 0x%04x},\n",
			strconv.Quote(symbol.command), strconv.Quote(symbol.text), symbol.class,
			symbol.group, symbol.font, symbol.acceptUnicode, symbol.kind, symbol.sourceGlyph)
	}
	b.WriteString("}\n")
	return b.String()
}

func generateNegatedRelations(pairs []negationPair) string {
	var b strings.Builder
	b.WriteString("var negatedRelations = [...]negatedRelation{\n")
	for _, pair := range pairs {
		fmt.Fprintf(&b, "\t{base: %s, negated: %s},\n",
			strconv.Quote(pair.base), strconv.Quote(pair.negated))
	}
	b.WriteString("}\n")
	return b.String()
}

func generateTypst(symbols []typstSymbol, shorthands []shorthand) string {
	var b strings.Builder
	b.WriteString("var typstSymbols = [...]TypstSymbol{\n")
	for _, symbol := range symbols {
		fmt.Fprintf(&b,
			"\t{Path: %s, Name: %s, Modifier: %s, Value: %s, Deprecation: %s},\n",
			strconv.Quote(symbol.path), strconv.Quote(symbol.name), strconv.Quote(symbol.modifier),
			strconv.Quote(symbol.value), strconv.Quote(symbol.deprecation))
	}
	b.WriteString("}\n\n")
	b.WriteString("var typstShorthands = [...]TypstShorthand{\n")
	for _, shorthand := range shorthands {
		fmt.Fprintf(&b, "\t{Syntax: %s, Value: %s},\n",
			strconv.Quote(shorthand.syntax), strconv.Quote(shorthand.value))
	}
	b.WriteString("}\n")
	return b.String()
}

func generateOperators(tex, typst []operator) string {
	var b strings.Builder
	b.WriteString("var texOperators = [...]Operator{\n")
	for _, op := range tex {
		fmt.Fprintf(&b, "\t{Name: %s, Text: %s, Limits: %t, Symbol: %t},\n",
			strconv.Quote(op.name), strconv.Quote(op.text), op.limits, op.symbol)
	}
	b.WriteString("}\n\n")
	b.WriteString("var typstOperators = [...]Operator{\n")
	for _, op := range typst {
		fmt.Fprintf(&b, "\t{Name: %s, Text: %s, Limits: %t, Symbol: %t},\n",
			strconv.Quote(op.name), strconv.Quote(op.text), op.limits, op.symbol)
	}
	b.WriteString("}\n")
	return b.String()
}

func matchBody(src, startPattern, endPattern string) (string, error) {
	start := regexp.MustCompile(startPattern).FindStringIndex(src)
	if start == nil {
		return "", fmt.Errorf("pattern %q not found", startPattern)
	}
	end := regexp.MustCompile(endPattern).FindStringIndex(src[start[1]:])
	if end == nil {
		return "", fmt.Errorf("end pattern %q not found", endPattern)
	}
	return src[start[1] : start[1]+end[0]], nil
}

func objectBody(src, prefix string) (string, error) {
	at := strings.Index(src, prefix)
	if at < 0 {
		return "", fmt.Errorf("%s not found", prefix)
	}
	brace := strings.Index(src[at:], "{")
	if brace < 0 {
		return "", fmt.Errorf("opening brace for %s not found", prefix)
	}
	return bracesAt(src, at+brace)
}

func namedObject(block, key string) (string, error) {
	pattern := regexp.MustCompile(`(?:^|[\n,{])\s*(?:'` + regexp.QuoteMeta(key) + `'|` +
		regexp.QuoteMeta(key) + `)\s*:\s*\{`)
	match := pattern.FindStringIndex(block)
	if match == nil {
		return "", fmt.Errorf("object %q not found", key)
	}
	brace := strings.LastIndex(block[match[0]:match[1]], "{") + match[0]
	return bracesAt(block, brace)
}

func bracesAt(src string, open int) (string, error) {
	body, _, err := braceGroupAt(src, open)
	return body, err
}

func braceGroupAt(src string, open int) (string, int, error) {
	if open < 0 || open >= len(src) || src[open] != '{' {
		return "", 0, errors.New("internal generator error: brace offset")
	}
	depth := 0
	inString := byte(0)
	escaped := false
	for i := open; i < len(src); i++ {
		c := src[i]
		if inString != 0 {
			if escaped {
				escaped = false
			} else if c == '\\' {
				escaped = true
			} else if c == inString {
				inString = 0
			}
			continue
		}
		switch c {
		case '"', '`':
			inString = c
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return src[open+1 : i], i + 1, nil
			}
		}
	}
	return "", 0, errors.New("unclosed brace")
}

func bracketBody(src, marker string) (string, error) {
	at := strings.Index(src, marker)
	if at < 0 {
		return "", fmt.Errorf("%s not found", marker)
	}
	open := strings.Index(src[at:], "[")
	if open < 0 {
		return "", fmt.Errorf("opening bracket for %s not found", marker)
	}
	open += at
	return bracketsAt(src, open)
}

func arrayAfterMarker(src, marker string) (string, error) {
	at := strings.Index(src, marker)
	if at < 0 {
		return "", fmt.Errorf("%s not found", marker)
	}
	equal := strings.Index(src[at:], "= [")
	if equal < 0 {
		return "", fmt.Errorf("array assignment for %s not found", marker)
	}
	return bracketsAt(src, at+equal+2)
}

func bracketsAt(src string, open int) (string, error) {
	if open < 0 || open >= len(src) || src[open] != '[' {
		return "", errors.New("internal generator error: bracket offset")
	}
	depth := 0
	inString := byte(0)
	escaped := false
	for i := open; i < len(src); i++ {
		c := src[i]
		if inString != 0 {
			if escaped {
				escaped = false
			} else if c == '\\' {
				escaped = true
			} else if c == inString {
				inString = 0
			}
			continue
		}
		switch c {
		case '"', '`':
			inString = c
		case '[':
			depth++
		case ']':
			depth--
			if depth == 0 {
				return src[open+1 : i], nil
			}
		}
	}
	return "", errors.New("unclosed bracket")
}

func functionBlocks(src string) []string {
	var blocks []string
	for offset := 0; ; {
		at := strings.Index(src[offset:], "defineFunction({")
		if at < 0 {
			break
		}
		at += offset
		open := strings.Index(src[at:], "{") + at
		body, err := bracesAt(src, open)
		if err != nil {
			break
		}
		blocks = append(blocks, body)
		offset = open + len(body) + 2
	}
	return blocks
}

func singleBoolField(src, field string) (bool, error) {
	re := regexp.MustCompile(`\b` + regexp.QuoteMeta(field) + `\s*:\s*(true|false)\b`)
	matches := re.FindAllStringSubmatch(src, -1)
	if len(matches) != 1 {
		return false, fmt.Errorf("expected one %s field, found %d", field, len(matches))
	}
	return matches[0][1] == "true", nil
}

func jsStrings(src string) ([]string, error) {
	re := regexp.MustCompile(`"((?:\\.|[^"\\])*)"`)
	matches := re.FindAllStringSubmatch(src, -1)
	out := make([]string, 0, len(matches))
	for _, match := range matches {
		value, err := strconv.Unquote(`"` + match[1] + `"`)
		if err != nil {
			return nil, err
		}
		out = append(out, value)
	}
	return out, nil
}

func stringAfterAssignment(src, marker string) (string, error) {
	at := strings.Index(src, marker)
	if at < 0 {
		return "", fmt.Errorf("%s not found", marker)
	}
	open := strings.Index(src[at:], `"`)
	if open < 0 {
		return "", fmt.Errorf("string after %s not found", marker)
	}
	open += at
	escaped := false
	for i := open + 1; i < len(src); i++ {
		switch {
		case escaped:
			escaped = false
		case src[i] == '\\':
			escaped = true
		case src[i] == '"':
			return src[open : i+1], nil
		}
	}
	return "", fmt.Errorf("unclosed string after %s", marker)
}

func decodeRustString(src string) (string, error) {
	if !strings.Contains(src, `\`) {
		return src, nil
	}
	var b bytes.Buffer
	for len(src) > 0 {
		if src[0] != '\\' {
			r, size := utf8.DecodeRuneInString(src)
			b.WriteRune(r)
			src = src[size:]
			continue
		}
		value, used, err := decodeRustEscape(src)
		if err != nil {
			return "", err
		}
		b.WriteString(value)
		src = src[used:]
	}
	return b.String(), nil
}

func decodeRustChar(src string) (rune, error) {
	value, err := decodeRustString(src)
	if err != nil {
		return 0, err
	}
	r, size := utf8.DecodeRuneInString(value)
	if r == utf8.RuneError || size != len(value) {
		return 0, fmt.Errorf("invalid Rust character %q", src)
	}
	return r, nil
}

func decodeRustEscape(src string) (string, int, error) {
	if len(src) < 2 || src[0] != '\\' {
		return "", 0, errors.New("invalid Rust escape")
	}
	switch src[1] {
	case '\\', '\'', '"':
		return string(src[1]), 2, nil
	case 'n':
		return "\n", 2, nil
	case 'r':
		return "\r", 2, nil
	case 't':
		return "\t", 2, nil
	case '0':
		return "\x00", 2, nil
	case 'u':
		if len(src) < 4 || src[2] != '{' {
			return "", 0, errors.New("invalid Rust Unicode escape")
		}
		end := strings.IndexByte(src[3:], '}')
		if end < 0 {
			return "", 0, errors.New("unclosed Rust Unicode escape")
		}
		digits := src[3 : 3+end]
		n, err := strconv.ParseUint(digits, 16, 32)
		if err != nil || !utf8.ValidRune(rune(n)) {
			return "", 0, fmt.Errorf("invalid Rust Unicode escape %q", digits)
		}
		return string(rune(n)), 4 + end, nil
	default:
		return "", 0, fmt.Errorf("unsupported Rust escape \\%c", src[1])
	}
}
