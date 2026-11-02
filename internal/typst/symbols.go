package typst

import (
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/caelyreth/untie/ir"
)

type symbolDef struct {
	text  string
	class ir.MathClass
}

var typstSymbols = map[string]symbolDef{
	// Greek.
	"alpha": {text: "α", class: ir.ClassAlphabetic}, "beta": {text: "β", class: ir.ClassAlphabetic},
	"gamma": {text: "γ", class: ir.ClassAlphabetic}, "delta": {text: "δ", class: ir.ClassAlphabetic},
	"epsilon": {text: "ϵ", class: ir.ClassAlphabetic}, "epsilon.alt": {text: "ε", class: ir.ClassAlphabetic},
	"zeta": {text: "ζ", class: ir.ClassAlphabetic}, "eta": {text: "η", class: ir.ClassAlphabetic},
	"theta": {text: "θ", class: ir.ClassAlphabetic}, "theta.alt": {text: "ϑ", class: ir.ClassAlphabetic},
	"iota": {text: "ι", class: ir.ClassAlphabetic}, "kappa": {text: "κ", class: ir.ClassAlphabetic},
	"lambda": {text: "λ", class: ir.ClassAlphabetic}, "mu": {text: "μ", class: ir.ClassAlphabetic},
	"nu": {text: "ν", class: ir.ClassAlphabetic}, "xi": {text: "ξ", class: ir.ClassAlphabetic},
	"omicron": {text: "ο", class: ir.ClassAlphabetic}, "pi": {text: "π", class: ir.ClassAlphabetic},
	"pi.alt": {text: "ϖ", class: ir.ClassAlphabetic}, "rho": {text: "ρ", class: ir.ClassAlphabetic},
	"rho.alt": {text: "ϱ", class: ir.ClassAlphabetic}, "sigma": {text: "σ", class: ir.ClassAlphabetic},
	"sigma.alt": {text: "ς", class: ir.ClassAlphabetic}, "tau": {text: "τ", class: ir.ClassAlphabetic},
	"upsilon": {text: "υ", class: ir.ClassAlphabetic}, "phi": {text: "ϕ", class: ir.ClassAlphabetic},
	"phi.alt": {text: "φ", class: ir.ClassAlphabetic}, "chi": {text: "χ", class: ir.ClassAlphabetic},
	"psi": {text: "ψ", class: ir.ClassAlphabetic}, "omega": {text: "ω", class: ir.ClassAlphabetic},
	"Gamma": {text: "Γ", class: ir.ClassAlphabetic}, "Delta": {text: "Δ", class: ir.ClassAlphabetic},
	"Theta": {text: "Θ", class: ir.ClassAlphabetic}, "Lambda": {text: "Λ", class: ir.ClassAlphabetic},
	"Xi": {text: "Ξ", class: ir.ClassAlphabetic}, "Pi": {text: "Π", class: ir.ClassAlphabetic},
	"Sigma": {text: "Σ", class: ir.ClassAlphabetic}, "Upsilon": {text: "Υ", class: ir.ClassAlphabetic},
	"Phi": {text: "Φ", class: ir.ClassAlphabetic}, "Psi": {text: "Ψ", class: ir.ClassAlphabetic},
	"Omega": {text: "Ω", class: ir.ClassAlphabetic},

	// Common symbols and operators.
	"aleph": {text: "ℵ", class: ir.ClassNormal}, "beth": {text: "ℶ", class: ir.ClassNormal},
	"oo": {text: "∞", class: ir.ClassNormal}, "infinity": {text: "∞", class: ir.ClassNormal},
	"emptyset": {text: "∅", class: ir.ClassNormal}, "nothing": {text: "∅", class: ir.ClassNormal},
	"dif": {text: "d", class: ir.ClassNormal}, "diff": {text: "d", class: ir.ClassNormal},
	"sum": {text: "∑", class: ir.ClassLarge}, "product": {text: "∏", class: ir.ClassLarge},
	"coproduct": {text: "∐", class: ir.ClassLarge}, "integral": {text: "∫", class: ir.ClassLarge},
	"integral.double": {text: "∬", class: ir.ClassLarge}, "integral.triple": {text: "∭", class: ir.ClassLarge},
	"integral.cont": {text: "∮", class: ir.ClassLarge}, "union": {text: "⋃", class: ir.ClassLarge},
	"intersection": {text: "⋂", class: ir.ClassLarge}, "union.big": {text: "⋃", class: ir.ClassLarge},
	"intersection.big": {text: "⋂", class: ir.ClassLarge}, "sect": {text: "⋀", class: ir.ClassLarge},
	"vee": {text: "⋁", class: ir.ClassLarge}, "wedge": {text: "⋀", class: ir.ClassLarge},
	"tensor": {text: "⊗", class: ir.ClassLarge},

	"plus": {text: "+", class: ir.ClassBinary}, "minus": {text: "−", class: ir.ClassBinary},
	"plus.minus": {text: "±", class: ir.ClassBinary}, "minus.plus": {text: "∓", class: ir.ClassBinary},
	"times": {text: "×", class: ir.ClassBinary}, "div": {text: "÷", class: ir.ClassBinary},
	"dot": {text: "⋅", class: ir.ClassBinary}, "ast": {text: "∗", class: ir.ClassBinary},
	"circle": {text: "∘", class: ir.ClassBinary}, "oplus": {text: "⊕", class: ir.ClassBinary},
	"ominus": {text: "⊖", class: ir.ClassBinary}, "otimes": {text: "⊗", class: ir.ClassBinary},
	"odot": {text: "⊙", class: ir.ClassBinary}, "sqcap": {text: "⊓", class: ir.ClassBinary},
	"sqcup": {text: "⊔", class: ir.ClassBinary}, "intercal": {text: "⊺", class: ir.ClassBinary},
	"and": {text: "∧", class: ir.ClassBinary}, "or": {text: "∨", class: ir.ClassBinary},

	"=": {text: "=", class: ir.ClassRelation}, "eq": {text: "=", class: ir.ClassRelation},
	"<": {text: "<", class: ir.ClassRelation}, ">": {text: ">", class: ir.ClassRelation},
	"<=": {text: "≤", class: ir.ClassRelation}, ">=": {text: "≥", class: ir.ClassRelation},
	"!=": {text: "≠", class: ir.ClassRelation}, "approx": {text: "≈", class: ir.ClassRelation},
	"equiv": {text: "≡", class: ir.ClassRelation}, "prop": {text: "∝", class: ir.ClassRelation},
	"in": {text: "∈", class: ir.ClassRelation}, "in.not": {text: "∉", class: ir.ClassRelation},
	"subset": {text: "⊂", class: ir.ClassRelation}, "subset.eq": {text: "⊆", class: ir.ClassRelation},
	"supset": {text: "⊃", class: ir.ClassRelation}, "supset.eq": {text: "⊇", class: ir.ClassRelation},
	"prec": {text: "≺", class: ir.ClassRelation}, "succ": {text: "≻", class: ir.ClassRelation},
	"sim": {text: "∼", class: ir.ClassRelation}, "perp": {text: "⟂", class: ir.ClassRelation},
	"parallel": {text: "∥", class: ir.ClassRelation}, "models": {text: "⊨", class: ir.ClassRelation},
	"arrow.r": {text: "→", class: ir.ClassRelation}, "arrow.l": {text: "←", class: ir.ClassRelation},
	"arrow.l.r": {text: "↔", class: ir.ClassRelation}, "arrow.r.long": {text: "⟶", class: ir.ClassRelation},
	"arrow.l.long": {text: "⟵", class: ir.ClassRelation}, "arrow.l.r.long": {text: "⟷", class: ir.ClassRelation},
	"arrow.r.double": {text: "⇒", class: ir.ClassRelation}, "arrow.l.double": {text: "⇐", class: ir.ClassRelation},
	"arrow.l.r.double": {text: "⇔", class: ir.ClassRelation}, "arrow": {text: "→", class: ir.ClassRelation},
	"arrow.t": {text: "↑", class: ir.ClassRelation},
	"mapsto":  {text: "↦", class: ir.ClassRelation}, "maps.to": {text: "↦", class: ir.ClassRelation},
	"implies": {text: "⇒", class: ir.ClassRelation}, "iff": {text: "⇔", class: ir.ClassRelation},
	"tilde.op": {text: "∼", class: ir.ClassRelation},

	// Delimiters.
	"paren.l": {text: "(", class: ir.ClassOpening}, "paren.r": {text: ")", class: ir.ClassClosing},
	"bracket.l": {text: "[", class: ir.ClassOpening}, "bracket.r": {text: "]", class: ir.ClassClosing},
	"brace.l": {text: "{", class: ir.ClassOpening}, "brace.r": {text: "}", class: ir.ClassClosing},
	"angle.l": {text: "⟨", class: ir.ClassOpening}, "angle.r": {text: "⟩", class: ir.ClassClosing},
	"floor.l": {text: "⌊", class: ir.ClassOpening}, "floor.r": {text: "⌋", class: ir.ClassClosing},
	"ceil.l": {text: "⌈", class: ir.ClassOpening}, "ceil.r": {text: "⌉", class: ir.ClassClosing},
	"chevron.l": {text: "⟨", class: ir.ClassOpening}, "chevron.r": {text: "⟩", class: ir.ClassClosing},
	"bar.v": {text: "|", class: ir.ClassFence}, "bar.v.double": {text: "‖", class: ir.ClassFence},
	"||": {text: "‖", class: ir.ClassFence},

	// Punctuation and annotation marks.
	"comma": {text: ",", class: ir.ClassPunctuation}, "semicolon": {text: ";", class: ir.ClassPunctuation},
	"colon": {text: ":", class: ir.ClassRelation}, "dots": {text: "…", class: ir.ClassPunctuation},
	"dots.h": {text: "…", class: ir.ClassPunctuation}, "dots.v": {text: "⋮", class: ir.ClassPunctuation},
	"dots.down": {text: "⋱", class: ir.ClassPunctuation}, "dots.up": {text: "⋰", class: ir.ClassPunctuation},
	"prime": {text: "′", class: ir.ClassNormal}, "degree": {text: "°", class: ir.ClassNormal},
	"hat": {text: "̂", class: ir.ClassDiacritic}, "tilde": {text: "̃", class: ir.ClassDiacritic},
	"macron": {text: "̄", class: ir.ClassDiacritic}, "bar": {text: "¯", class: ir.ClassDiacritic},
	"breve": {text: "̆", class: ir.ClassDiacritic}, "acute": {text: "́", class: ir.ClassDiacritic},
	"grave": {text: "̀", class: ir.ClassDiacritic}, "caron": {text: "̌", class: ir.ClassDiacritic},
	"dot.double": {text: "̈", class: ir.ClassDiacritic}, "dot.triple": {text: "⃛", class: ir.ClassDiacritic},
	"dot.quad": {text: "⃜", class: ir.ClassDiacritic}, "circle.stroked": {text: "○", class: ir.ClassDiacritic},
}

var textOperators = map[string]bool{
	"Pr": true, "arccos": true, "arcsin": true, "arctan": true, "arg": true,
	"cos": true, "cosh": true, "cot": true, "coth": true, "csc": true,
	"csch": true, "ctg": true, "deg": true, "det": true, "dim": true,
	"exp": true, "gcd": true, "hom": true, "id": true, "im": true,
	"inf": true, "ker": true, "lcm": true, "lg": true, "lim": true,
	"liminf": true, "limsup": true, "ln": true, "log": true, "max": true,
	"min": true, "mod": true, "sec": true, "sech": true, "sin": true,
	"sinc": true, "sinh": true, "sup": true, "tan": true, "tanh": true,
	"tg": true, "tr": true, "argmax": true, "argmin": true,
}

var spacingSymbols = map[string]ir.Length{
	"space": builtinEmLength(0.25),
	"thin":  builtinEmLength(1.0 / 6.0),
	"med":   builtinEmLength(2.0 / 9.0),
	"thick": builtinEmLength(5.0 / 18.0),
	"quad":  builtinEmLength(1),
	"wide":  builtinEmLength(2),
}

func builtinEmLength(value float64) ir.Length {
	length, err := ir.EmLength(value)
	if err != nil {
		panic("invalid built-in Typst em length: " + err.Error())
	}
	return length
}

var accentAliases = map[string]string{
	"hat": "̂", "tilde": "̃", "macron": "̄", "bar": "¯", "breve": "̆",
	"acute": "́", "grave": "̀", "caron": "̌", "dot": "̇", "dot.double": "̈",
	"arrow": "⃗", "arrow.l": "⃖", "arrow.l.r": "⃡", "circle": "̊",
}

func symbol(name string) (symbolDef, bool) {
	name = strings.TrimPrefix(name, "sym.")
	name = strings.TrimPrefix(name, "math.")
	def, ok := typstSymbols[name]
	return def, ok
}

func classForRune(r rune) ir.MathClass {
	switch {
	case unicode.IsLetter(r):
		return ir.ClassAlphabetic
	case strings.ContainsRune("([{⟨⌈⌊⟦", r):
		return ir.ClassOpening
	case strings.ContainsRune(")]}⟩⌉⌋⟧", r):
		return ir.ClassClosing
	case strings.ContainsRune("|‖", r):
		return ir.ClassFence
	case strings.ContainsRune("+−±∓×÷·⋅∗∘⊕⊖⊗⊙", r):
		return ir.ClassBinary
	case strings.ContainsRune("=<>≤≥≠≈≡∈∉⊂⊃⊆⊇⟂∥→←↔⇒⇔↦∼∝", r):
		return ir.ClassRelation
	case strings.ContainsRune(",;", r):
		return ir.ClassPunctuation
	case strings.ContainsRune("∑∏∐∫∬∭∮⋃⋂", r):
		return ir.ClassLarge
	default:
		return ir.ClassNormal
	}
}

func classForText(text string) ir.MathClass {
	if utf8.RuneCountInString(text) == 1 {
		r, _ := utf8.DecodeRuneInString(text)
		return classForRune(r)
	}
	return ir.ClassNormal
}
