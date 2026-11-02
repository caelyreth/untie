package tex

import (
	"unicode"
	"unicode/utf8"

	"github.com/caelyreth/untie/ir"
)

type symbolSpec struct {
	text    string
	class   ir.MathClass
	variant ir.Variant
	limits  ir.LimitsPlacement
}

type delimiterSpec struct {
	text  string
	class ir.MathClass
}

type accentSpec struct {
	text     string
	stretchy bool
}

type spaceSpec struct {
	kind   ir.SpaceKind
	amount ir.Length
}

var (
	symbolCommands    = map[string]symbolSpec{}
	delimiterCommands = map[string]delimiterSpec{}
	accentCommands    = map[string]accentSpec{}
	spacingCommands   = map[string]spaceSpec{}
	namedOperators    = map[string]string{}
	limitOperators    = map[string]bool{}
	fontCommands      = map[string]ir.Variant{}
	infixCommands     = map[string]fractionInfix{}
)

type fractionInfix struct {
	bar   bool
	style ir.FractionStyle
}

func init() {
	addSymbols(ir.ClassRelation,
		`equiv ≡
prec ≺
succ ≻
sim ∼
perp ⊥
preceq ⪯
succeq ⪰
simeq ≃
mid ∣
ll ≪
gg ≫
asymp ≍
parallel ∥
bowtie ⋈
smile ⌣
sqsubseteq ⊑
sqsupseteq ⊒
doteq ≐
frown ⌢
ni ∋
owns ∋
propto ∝
vdash ⊢
dashv ⊣
approx ≈
cong ≅
ge ≥
geq ≥
gets ←
gt >
in ∈
subset ⊂
supset ⊃
subseteq ⊆
supseteq ⊇
models ⊨
leftarrow ←
le ≤
leq ≤
lt <
rightarrow →
to →
uparrow ↑
Uparrow ⇑
downarrow ↓
Downarrow ⇓
updownarrow ↕
Updownarrow ⇕
longrightarrow ⟶
Longrightarrow ⟹
longleftarrow ⟵
Longleftarrow ⟸
leftrightarrow ↔
Leftrightarrow ⇔
longleftrightarrow ⟷
Longleftrightarrow ⟺
mapsto ↦
longmapsto ⟼
nearrow ↗
searrow ↘
swarrow ↙
nwarrow ↖
hookleftarrow ↩
hookrightarrow ↪
leftharpoonup ↼
rightharpoonup ⇀
leftharpoondown ↽
rightharpoondown ⇁
rightleftharpoons ⇌
iff ⟺
implies ⟹
impliedby ⟸
neq ≠
ne ≠
notin ∉
subsetneq ⊊
supsetneq ⊋
triangleq ≜
therefore ∴
because ∵
varpropto ∝
parallel ∥
nmid ∤
nleq ≰
ngeq ≱
nsubseteq ⊈
nsupseteq ⊉`)
	addSymbols(ir.ClassBinary,
		`mp ∓
ominus ⊖
uplus ⊎
sqcap ⊓
ast ∗
sqcup ⊔
bigcirc ◯
bullet ∙
ddagger ‡
wr ≀
amalg ⨿
cdot ⋅
circ ∘
div ÷
pm ±
times ×
cap ∩
cup ∪
setminus ∖
land ∧
lor ∨
wedge ∧
vee ∨
odot ⊙
oplus ⊕
otimes ⊗
oslash ⊘
dagger †
diamond ⋄
star ⋆
triangleleft ◃
triangleright ▹
barwedge ⊼
veebar ⊻
dotplus ∔
smallsetminus ∖
Cap ⋒
Cup ⋓
boxplus ⊞
boxminus ⊟
boxtimes ⊠
ltimes ⋉
rtimes ⋊
intercal ⊺`)
	addSymbols(ir.ClassNormal,
		`aleph ℵ
forall ∀
hbar ℏ
exists ∃
nabla ∇
flat ♭
ell ℓ
natural ♮
clubsuit ♣
wp ℘
sharp ♯
diamondsuit ♢
Re ℜ
heartsuit ♡
Im ℑ
spadesuit ♠
S §
P ¶
dag †
ddag ‡
partial ∂
infty ∞
prime ′
angle ∠
triangle △
emptyset ∅
varnothing ∅
neg ¬
lnot ¬
top ⊤
bot ⊥
degree °
pounds £
surd √
backslash \
imath ı
jmath ȷ
ldots …
mathellipsis …
cdots ⋯
ddots ⋱
vdots ⋮
checkmark ✓
square □
Box □
Diamond ◇
blacktriangle ▲
blacktriangledown ▼
blacksquare ■
bigstar ★
beth ℶ
daleth ℸ
gimel ℷ
digamma ϝ
varkappa ϰ
hslash ℏ
lozenge ◊
circledS Ⓢ
maltese ✠`)
	addSymbols(ir.ClassAlphabetic,
		`alpha α
beta β
gamma γ
delta δ
epsilon ϵ
zeta ζ
eta η
theta θ
iota ι
kappa κ
lambda λ
mu μ
nu ν
xi ξ
omicron ο
pi π
rho ρ
sigma σ
tau τ
upsilon υ
phi ϕ
chi χ
psi ψ
omega ω
varepsilon ε
vartheta ϑ
varpi ϖ
varrho ϱ
varsigma ς
varphi φ
Gamma Γ
Delta Δ
Theta Θ
Lambda Λ
Xi Ξ
Pi Π
Sigma Σ
Upsilon Υ
Phi Φ
Psi Ψ
Omega Ω`)
	addSymbols(ir.ClassPunctuation,
		`ldotp .
cdotp ⋅
ldots …
vdots ⋮
ddots ⋱`)
	addSymbols(ir.ClassLarge,
		`coprod ∐
bigvee ⋁
bigwedge ⋀
biguplus ⨄
bigcap ⋂
bigcup ⋃
int ∫
intop ∫
iint ∬
iiint ∭
prod ∏
sum ∑
bigotimes ⨂
bigoplus ⨁
bigodot ⨀
oint ∮
oiint ∯
oiiint ∰
bigsqcup ⨆
smallint ∫`)

	for _, name := range []string{"sum", "prod", "coprod", "bigcap", "bigcup", "bigvee", "bigwedge",
		"biguplus", "bigotimes", "bigoplus", "bigodot", "bigsqcup"} {
		spec := symbolCommands["\\"+name]
		spec.limits = ir.LimitsAuto
		symbolCommands["\\"+name] = spec
	}

	addDelimiters()
	addAccents()
	addSpaces()
	addOperators()
	addFonts()
	addInfixes()
	addEscapedSymbols()
}

func addSymbols(class ir.MathClass, table string) {
	for _, line := range splitLines(table) {
		name, text, ok := cutSpace(line)
		if !ok {
			continue
		}
		limits := ir.LimitsAuto
		if class == ir.ClassLarge {
			limits = ir.LimitsNever
		}
		symbolCommands["\\"+name] = symbolSpec{
			text:    text,
			class:   class,
			variant: ir.VariantAuto,
			limits:  limits,
		}
	}
}

func addEscapedSymbols() {
	// Escaped TeX input characters are ordinary glyphs, not macro names.
	for _, entry := range []struct {
		name string
		text string
	}{
		{"\\$", "$"},
		{"\\%", "%"},
		{"\\#", "#"},
		{"\\&", "&"},
		{"\\_", "_"},
		{"\\colon", ":"},
	} {
		symbolCommands[entry.name] = symbolSpec{
			text:    entry.text,
			class:   ir.ClassNormal,
			variant: ir.VariantAuto,
		}
	}
	delimiterCommands["\\|"] = delimiterSpec{text: "‖", class: ir.ClassFence}
}

func addDelimiters() {
	for _, entry := range []struct {
		name  string
		text  string
		class ir.MathClass
	}{
		{"{", "{", ir.ClassOpening},
		{"}", "}", ir.ClassClosing},
		{"lbrace", "{", ir.ClassOpening},
		{"rbrace", "}", ir.ClassClosing},
		{"lbrack", "[", ir.ClassOpening},
		{"rbrack", "]", ir.ClassClosing},
		{"lparen", "(", ir.ClassOpening},
		{"rparen", ")", ir.ClassClosing},
		{"langle", "⟨", ir.ClassOpening},
		{"rangle", "⟩", ir.ClassClosing},
		{"lfloor", "⌊", ir.ClassOpening},
		{"rfloor", "⌋", ir.ClassClosing},
		{"lceil", "⌈", ir.ClassOpening},
		{"rceil", "⌉", ir.ClassClosing},
		{"lvert", "|", ir.ClassOpening},
		{"rvert", "|", ir.ClassClosing},
		{"vert", "|", ir.ClassFence},
		{"lVert", "‖", ir.ClassOpening},
		{"rVert", "‖", ir.ClassClosing},
		{"Vert", "‖", ir.ClassFence},
		{"lmoustache", "⎰", ir.ClassOpening},
		{"rmoustache", "⎱", ir.ClassClosing},
		{"lgroup", "⟮", ir.ClassOpening},
		{"rgroup", "⟯", ir.ClassClosing},
		{"lbrace", "{", ir.ClassOpening},
		{"rbrace", "}", ir.ClassClosing},
	} {
		delimiterCommands["\\"+entry.name] = delimiterSpec{text: entry.text, class: entry.class}
	}
	for _, entry := range []struct {
		text  string
		class ir.MathClass
	}{
		{"(", ir.ClassOpening},
		{")", ir.ClassClosing},
		{"[", ir.ClassOpening},
		{"]", ir.ClassClosing},
		{"{", ir.ClassOpening},
		{"}", ir.ClassClosing},
		{"|", ir.ClassFence},
		{"‖", ir.ClassFence},
		{"⟨", ir.ClassOpening},
		{"⟩", ir.ClassClosing},
		{"⌊", ir.ClassOpening},
		{"⌋", ir.ClassClosing},
		{"⌈", ir.ClassOpening},
		{"⌉", ir.ClassClosing},
		{".", ir.ClassNormal},
	} {
		delimiterCommands[entry.text] = delimiterSpec{text: entry.text, class: entry.class}
	}
}

func addAccents() {
	for _, entry := range []struct {
		name    string
		text    string
		stretch bool
	}{
		{"acute", "ˊ", false},
		{"grave", "ˋ", false},
		{"ddot", "¨", false},
		{"dddot", "⃛", false},
		{"ddddot", "⃜", false},
		{"tilde", "~", false},
		{"bar", "¯", false},
		{"breve", "˘", false},
		{"check", "ˇ", false},
		{"hat", "^", false},
		{"vec", "⃗", false},
		{"dot", "˙", false},
		{"mathring", "˚", false},
		{"widehat", "^", true},
		{"widetilde", "~", true},
		{"widecheck", "ˇ", true},
		{"overrightarrow", "→", true},
		{"overleftarrow", "←", true},
		{"overleftrightarrow", "↔", true},
		{"overgroup", "⏠", true},
		{"overlinesegment", "‾", true},
		{"overleftharpoon", "↼", true},
		{"overrightharpoon", "⇀", true},
	} {
		accentCommands["\\"+entry.name] = accentSpec{text: entry.text, stretchy: entry.stretch}
	}
}

func addSpaces() {
	mustMuLength := func(value float64) ir.Length {
		length, err := ir.MuLength(value)
		if err != nil {
			panic("invalid built-in TeX math-unit space: " + err.Error())
		}
		return length
	}
	mustEmLength := func(value float64) ir.Length {
		length, err := ir.EmLength(value)
		if err != nil {
			panic("invalid built-in TeX em space: " + err.Error())
		}
		return length
	}
	spacingCommands["\\,"] = spaceSpec{kind: ir.SpaceExplicit, amount: mustMuLength(3)}
	spacingCommands["\\thinspace"] = spacingCommands["\\,"]
	spacingCommands["\\:"] = spaceSpec{kind: ir.SpaceExplicit, amount: mustMuLength(4)}
	spacingCommands["\\>"] = spacingCommands["\\:"]
	spacingCommands["\\medspace"] = spacingCommands["\\:"]
	spacingCommands["\\;"] = spaceSpec{kind: ir.SpaceExplicit, amount: mustMuLength(5)}
	spacingCommands["\\thickspace"] = spacingCommands["\\;"]
	spacingCommands["\\!"] = spaceSpec{kind: ir.SpaceExplicit, amount: mustMuLength(-3)}
	spacingCommands["\\negthinspace"] = spacingCommands["\\!"]
	spacingCommands["\\negmedspace"] = spaceSpec{kind: ir.SpaceExplicit, amount: mustMuLength(-4)}
	spacingCommands["\\negthickspace"] = spaceSpec{kind: ir.SpaceExplicit, amount: mustMuLength(-5)}
	spacingCommands["\\enspace"] = spaceSpec{kind: ir.SpaceExplicit, amount: mustEmLength(0.5)}
	spacingCommands["\\enskip"] = spacingCommands["\\enspace"]
	spacingCommands["\\quad"] = spaceSpec{kind: ir.SpaceExplicit, amount: mustEmLength(1)}
	spacingCommands["\\qquad"] = spaceSpec{kind: ir.SpaceExplicit, amount: mustEmLength(2)}
	spacingCommands["\\ "] = spaceSpec{kind: ir.SpaceInterword}
	spacingCommands["\\space"] = spacingCommands["\\ "]
	spacingCommands["\\nobreakspace"] = spacingCommands["\\ "]
}

func addOperators() {
	for _, name := range []string{
		"arccos", "arcsin", "arctan", "arg", "cos", "cosh", "cot", "coth", "csc",
		"deg", "det", "dim", "exp", "gcd", "hom", "inf", "ker", "lg", "lim",
		"liminf", "limsup", "ln", "log", "max", "min", "Pr", "sec", "sin",
		"sinh", "sup", "tan", "tanh",
	} {
		namedOperators["\\"+name] = name
	}
	for _, name := range []string{"det", "gcd", "inf", "lim", "liminf", "limsup", "max", "min", "Pr", "sup"} {
		limitOperators["\\"+name] = true
	}
}

func addFonts() {
	fontCommands = map[string]ir.Variant{
		"\\mathrm":     ir.VariantNormal,
		"\\mathnormal": ir.VariantAuto,
		"\\mathit":     ir.VariantItalic,
		"\\mathbf":     ir.VariantBold,
		"\\boldsymbol": ir.VariantBoldItalic,
		"\\bm":         ir.VariantBoldItalic,
		"\\mathbb":     ir.VariantDoubleStruck,
		"\\Bbb":        ir.VariantDoubleStruck,
		"\\mathcal":    ir.VariantScript,
		"\\mathscr":    ir.VariantScript,
		"\\mathfrak":   ir.VariantFraktur,
		"\\frak":       ir.VariantFraktur,
		"\\mathsf":     ir.VariantSans,
		"\\mathsfit":   ir.VariantSans,
		"\\mathtt":     ir.VariantMonospace,
		"\\bold":       ir.VariantBold,
	}
}

func addInfixes() {
	infixCommands["\\over"] = fractionInfix{bar: true, style: ir.FractionVertical}
	infixCommands["\\atop"] = fractionInfix{bar: false, style: ir.FractionVertical}
	infixCommands["\\choose"] = fractionInfix{bar: false, style: ir.FractionVertical}
}

func symbolForChar(text string) symbolSpec {
	if text == "" {
		return symbolSpec{}
	}
	r, _ := utf8.DecodeRuneInString(text)
	switch r {
	case '+', '*':
		if r == '*' {
			return symbolSpec{text: "∗", class: ir.ClassBinary, variant: ir.VariantAuto}
		}
		return symbolSpec{text: "+", class: ir.ClassBinary, variant: ir.VariantAuto}
	case '-':
		return symbolSpec{text: "−", class: ir.ClassBinary, variant: ir.VariantAuto}
	case '=':
		return symbolSpec{text: "=", class: ir.ClassRelation, variant: ir.VariantAuto}
	case '<', '>':
		return symbolSpec{text: text, class: ir.ClassRelation, variant: ir.VariantAuto}
	case ':':
		return symbolSpec{text: ":", class: ir.ClassRelation, variant: ir.VariantAuto}
	case ',':
		return symbolSpec{text: ",", class: ir.ClassPunctuation, variant: ir.VariantAuto}
	case ';':
		return symbolSpec{text: ";", class: ir.ClassPunctuation, variant: ir.VariantAuto}
	case '(':
		return symbolSpec{text: "(", class: ir.ClassOpening, variant: ir.VariantAuto}
	case ')':
		return symbolSpec{text: ")", class: ir.ClassClosing, variant: ir.VariantAuto}
	case '[':
		return symbolSpec{text: "[", class: ir.ClassOpening, variant: ir.VariantAuto}
	case ']':
		return symbolSpec{text: "]", class: ir.ClassClosing, variant: ir.VariantAuto}
	case '|':
		return symbolSpec{text: "|", class: ir.ClassFence, variant: ir.VariantAuto}
	case '/':
		return symbolSpec{text: "/", class: ir.ClassNormal, variant: ir.VariantAuto}
	case '`':
		return symbolSpec{text: "‘", class: ir.ClassNormal, variant: ir.VariantAuto}
	case '\u2212', '\u00B1', '\u00D7', '\u00F7', '\u00B7':
		return symbolSpec{text: text, class: ir.ClassBinary, variant: ir.VariantAuto}
	case '\u2264', '\u2265', '\u2260', '\u2248', '\u2192', '\u2190', '\u21D2', '\u21D4':
		return symbolSpec{text: text, class: ir.ClassRelation, variant: ir.VariantAuto}
	}
	if unicode.IsLetter(r) {
		return symbolSpec{text: text, class: ir.ClassAlphabetic, variant: ir.VariantAuto}
	}
	if unicode.IsDigit(r) {
		return symbolSpec{text: text, class: ir.ClassNormal, variant: ir.VariantAuto}
	}
	if unicode.IsSpace(r) {
		return symbolSpec{text: " ", class: ir.ClassSpace, variant: ir.VariantAuto}
	}
	return symbolSpec{text: text, class: ir.ClassNormal, variant: ir.VariantAuto}
}

func splitLines(table string) []string {
	var out []string
	start := 0
	for i := 0; i <= len(table); i++ {
		if i == len(table) || table[i] == '\n' {
			if i > start {
				out = append(out, table[start:i])
			}
			start = i + 1
		}
	}
	return out
}

func cutSpace(line string) (string, string, bool) {
	for i := 0; i < len(line); i++ {
		if line[i] == ' ' {
			if i == 0 || i+1 >= len(line) {
				return "", "", false
			}
			return line[:i], line[i+1:], true
		}
	}
	return "", "", false
}
