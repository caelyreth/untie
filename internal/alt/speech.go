package alt

import (
	"strings"
	"unicode"
)

func glyphSpeech(text string) string {
	text = strings.TrimSpace(text)
	if text == "" {
		return "empty"
	}
	if name, ok := symbolSpeech[text]; ok {
		return name
	}
	runes := []rune(text)
	if len(runes) == 2 {
		if runes[1] >= '\ufe00' && runes[1] <= '\ufe0f' {
			return glyphSpeech(string(runes[0]))
		}
	}
	allLetters := true
	for _, r := range text {
		if !unicode.IsLetter(r) && !unicode.IsDigit(r) {
			allLetters = false
			break
		}
	}
	if allLetters {
		return text
	}
	return "symbol " + text
}

func textSpeech(text string) string {
	if strings.TrimSpace(text) == "" {
		return text
	}
	return text
}

func operatorSpeech(text string) string {
	text = strings.TrimSpace(text)
	if name, ok := operatorNames[text]; ok {
		return name
	}
	return glyphSpeech(text)
}

var operatorNames = map[string]string{
	"sin":    "sine",
	"cos":    "cosine",
	"tan":    "tangent",
	"cot":    "cotangent",
	"sec":    "secant",
	"csc":    "cosecant",
	"sinh":   "hyperbolic sine",
	"cosh":   "hyperbolic cosine",
	"tanh":   "hyperbolic tangent",
	"log":    "logarithm",
	"ln":     "natural logarithm",
	"exp":    "exponential",
	"lim":    "limit",
	"max":    "maximum",
	"min":    "minimum",
	"sup":    "supremum",
	"inf":    "infimum",
	"arg":    "argument",
	"det":    "determinant",
	"gcd":    "greatest common divisor",
	"lcm":    "least common multiple",
	"mod":    "modulo",
	"ker":    "kernel",
	"dim":    "dimension",
	"rank":   "rank",
	"argmax": "arg max",
	"argmin": "arg min",
}

var symbolSpeech = map[string]string{
	"+": "plus",
	"-": "minus",
	"−": "minus",
	"*": "asterisk",
	"/": "slash",
	"=": "equals",
	"≠": "not equal to",
	"<": "less than",
	">": "greater than",
	"≤": "less than or equal to",
	"≥": "greater than or equal to",
	"≦": "less than or equal to",
	"≧": "greater than or equal to",
	"≈": "approximately equal to",
	"≉": "not approximately equal to",
	"≡": "equivalent to",
	"≢": "not equivalent to",
	"≅": "congruent to",
	"∼": "similar to",
	"±": "plus or minus",
	"∓": "minus or plus",
	"×": "times",
	"·": "dot",
	"⋅": "dot",
	"÷": "divided by",
	"∞": "infinity",
	"∑": "sum",
	"∏": "product",
	"∐": "coproduct",
	"∫": "integral",
	"∬": "double integral",
	"∭": "triple integral",
	"∮": "contour integral",
	"∂": "partial",
	"∇": "nabla",
	"√": "square root",
	"∛": "cube root",
	"∜": "fourth root",
	"∀": "for all",
	"∃": "there exists",
	"∄": "there does not exist",
	"∈": "element of",
	"∉": "not an element of",
	"∋": "contains",
	"∌": "does not contain",
	"∩": "intersection",
	"∪": "union",
	"∧": "and",
	"∨": "or",
	"¬": "not",
	"→": "right arrow",
	"←": "left arrow",
	"↔": "left right arrow",
	"⇒": "implies",
	"⇐": "implied by",
	"⇔": "if and only if",
	"↦": "maps to",
	"↑": "up arrow",
	"↓": "down arrow",
	"↗": "up right arrow",
	"↘": "down right arrow",
	"↙": "down left arrow",
	"↖": "up left arrow",
	"⊂": "subset of",
	"⊆": "subset of or equal to",
	"⊃": "superset of",
	"⊇": "superset of or equal to",
	"⊥": "perpendicular to",
	"∥": "parallel to",
	"∠": "angle",
	"∘": "composed with",
	"′": "prime",
	"″": "double prime",
	"‴": "triple prime",
	"(": "left parenthesis",
	")": "right parenthesis",
	"[": "left bracket",
	"]": "right bracket",
	"{": "left brace",
	"}": "right brace",
	"|": "vertical bar",
	"‖": "double vertical bar",
	"⟨": "left angle bracket",
	"⟩": "right angle bracket",
	"⌈": "left ceiling",
	"⌉": "right ceiling",
	"⌊": "left floor",
	"⌋": "right floor",
	",": "comma",
	".": "point",
	":": "colon",
	";": "semicolon",
	"!": "factorial",
	"?": "question mark",
	"%": "percent",
}
