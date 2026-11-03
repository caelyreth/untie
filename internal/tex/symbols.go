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

type sizedDelimiterSpec struct {
	sizing ir.DelimiterSizing
	class  ir.MathClass
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
	symbolCommands         = map[string]symbolSpec{}
	delimiterCommands      = map[string]delimiterSpec{}
	sizedDelimiterCommands = map[string]sizedDelimiterSpec{}
	sizedDelimiterText     = map[string]string{}
	accentCommands         = map[string]accentSpec{}
	spacingCommands        = map[string]spaceSpec{}
	namedOperators         = map[string]string{}
	limitOperators         = map[string]bool{}
	fontCommands           = map[string]ir.Variant{}
	infixCommands          = map[string]fractionInfix{}
	sizingCommands         = map[string]uint8{}
	sizingFontSizes        = map[string]ir.Length{}
)

type fractionInfix struct {
	bar       bool
	style     ir.FractionStyle
	open      string
	close     string
	dimension bool
	role      ir.FractionRole
}

func init() {
	addSizedDelimiters()
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
		"biguplus", "bigotimes", "bigoplus", "bigodot", "bigsqcup", "intop", "smallint"} {
		spec := symbolCommands["\\"+name]
		spec.limits = ir.LimitsAuto
		symbolCommands["\\"+name] = spec
	}

	addDelimiters()
	addAccents()
	addSpaces()
	addOperators()
	addFonts()
	addSizing()
	addInfixes()
	addEscapedSymbols()
	addTeXAliases()
}

func addSizedDelimiters() {
	for command, spec := range map[string]sizedDelimiterSpec{
		"\\big":   {sizing: texDelimiterTier(1), class: ir.ClassNormal},
		"\\Big":   {sizing: texDelimiterTier(2), class: ir.ClassNormal},
		"\\bigg":  {sizing: texDelimiterTier(3), class: ir.ClassNormal},
		"\\Bigg":  {sizing: texDelimiterTier(4), class: ir.ClassNormal},
		"\\bigl":  {sizing: texDelimiterTier(1), class: ir.ClassOpening},
		"\\Bigl":  {sizing: texDelimiterTier(2), class: ir.ClassOpening},
		"\\biggl": {sizing: texDelimiterTier(3), class: ir.ClassOpening},
		"\\Biggl": {sizing: texDelimiterTier(4), class: ir.ClassOpening},
		"\\bigr":  {sizing: texDelimiterTier(1), class: ir.ClassClosing},
		"\\Bigr":  {sizing: texDelimiterTier(2), class: ir.ClassClosing},
		"\\biggr": {sizing: texDelimiterTier(3), class: ir.ClassClosing},
		"\\Biggr": {sizing: texDelimiterTier(4), class: ir.ClassClosing},
		"\\bigm":  {sizing: texDelimiterTier(1), class: ir.ClassRelation},
		"\\Bigm":  {sizing: texDelimiterTier(2), class: ir.ClassRelation},
		"\\biggm": {sizing: texDelimiterTier(3), class: ir.ClassRelation},
		"\\Biggm": {sizing: texDelimiterTier(4), class: ir.ClassRelation},
	} {
		sizedDelimiterCommands[command] = spec
	}

	// This set and the canonical glyph substitutions mirror KaTeX's static
	// delimsizing.ts table. Aliases such as \lt intentionally resolve to the
	// angle delimiter glyph selected by makeSizedDelim.
	for source, text := range map[string]string{
		"(": "(", "\\lparen": "(", ")": ")", "\\rparen": ")",
		"[": "[", "\\lbrack": "[", "]": "]", "\\rbrack": "]",
		"\\{": "{", "\\lbrace": "{", "\\}": "}", "\\rbrace": "}",
		"\\lfloor": "⌊", "\\rfloor": "⌋", "⌊": "⌊", "⌋": "⌋",
		"\\lceil": "⌈", "\\rceil": "⌉", "⌈": "⌈", "⌉": "⌉",
		"<": "⟨", "\\langle": "⟨", "⟨": "⟨",
		">": "⟩", "\\rangle": "⟩", "⟩": "⟩",
		"\\lt": "⟨", "\\gt": "⟩",
		"\\lvert": "|", "\\rvert": "|", "\\lVert": "‖", "\\rVert": "‖",
		"\\lgroup": "⟮", "\\rgroup": "⟯", "⟮": "⟮", "⟯": "⟯",
		"\\lmoustache": "⎰", "\\rmoustache": "⎱", "⎰": "⎰", "⎱": "⎱",
		"/": "/", "\\backslash": "\\",
		"|": "|", "\\vert": "|", "\\|": "‖", "\\Vert": "‖",
		"\\uparrow": "↑", "\\Uparrow": "⇑",
		"\\downarrow": "↓", "\\Downarrow": "⇓",
		"\\updownarrow": "↕", "\\Updownarrow": "⇕",
		".": ".",
	} {
		sizedDelimiterText[source] = text
	}
}

// KaTeX sizeToMaxHeight is [1.2em, 1.8em, 2.4em, 3.0em]. Keeping those
// quantities in the TeX adapter prevents the neutral IR from encoding TeX's
// discrete command tiers.
func texDelimiterTier(tier int) ir.DelimiterSizing {
	switch tier {
	case 1:
		return ir.AbsoluteDelimiterSizing(ir.RawLength(ir.LengthEm, 1_200_000))
	case 2:
		return ir.AbsoluteDelimiterSizing(ir.RawLength(ir.LengthEm, 1_800_000))
	case 3:
		return ir.AbsoluteDelimiterSizing(ir.RawLength(ir.LengthEm, 2_400_000))
	case 4:
		return ir.AbsoluteDelimiterSizing(ir.RawLength(ir.LengthEm, 3_000_000))
	default:
		panic("invalid static TeX delimiter tier")
	}
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
	symbolCommands["\\colon"] = symbolSpec{text: ":", class: ir.ClassPunctuation, variant: ir.VariantAuto}
	delimiterCommands["\\|"] = delimiterSpec{text: "‖", class: ir.ClassFence}
}

func addTeXAliases() {
	aliases := map[string]symbolSpec{
		"\\Alpha":          {text: "A", class: ir.ClassNormal, variant: ir.VariantNormal},
		"\\angln":          {text: "∠n", class: ir.ClassNormal},
		"\\Bbbk":           {text: "𝕜", class: ir.ClassAlphabetic, variant: ir.VariantDoubleStruck},
		"\\Beta":           {text: "B", class: ir.ClassNormal, variant: ir.VariantNormal},
		"\\Chi":            {text: "X", class: ir.ClassNormal, variant: ir.VariantNormal},
		"\\Epsilon":        {text: "E", class: ir.ClassNormal, variant: ir.VariantNormal},
		"\\Eta":            {text: "H", class: ir.ClassNormal, variant: ir.VariantNormal},
		"\\Iota":           {text: "I", class: ir.ClassNormal, variant: ir.VariantNormal},
		"\\Kappa":          {text: "K", class: ir.ClassNormal, variant: ir.VariantNormal},
		"\\Mu":             {text: "M", class: ir.ClassNormal, variant: ir.VariantNormal},
		"\\Nu":             {text: "N", class: ir.ClassNormal, variant: ir.VariantNormal},
		"\\Omicron":        {text: "O", class: ir.ClassNormal, variant: ir.VariantNormal},
		"\\Rho":            {text: "P", class: ir.ClassNormal, variant: ir.VariantNormal},
		"\\Tau":            {text: "T", class: ir.ClassNormal, variant: ir.VariantNormal},
		"\\Zeta":           {text: "Z", class: ir.ClassNormal, variant: ir.VariantNormal},
		"\\alef":           {text: "ℵ", class: ir.ClassNormal},
		"\\alefsym":        {text: "ℵ", class: ir.ClassNormal},
		"\\bull":           {text: "•", class: ir.ClassBinary},
		"\\clubs":          {text: "♣", class: ir.ClassNormal},
		"\\Dagger":         {text: "‡", class: ir.ClassNormal},
		"\\diamonds":       {text: "♢", class: ir.ClassNormal},
		"\\empty":          {text: "∅", class: ir.ClassNormal},
		"\\exist":          {text: "∃", class: ir.ClassNormal},
		"\\hearts":         {text: "♡", class: ir.ClassNormal},
		"\\image":          {text: "ℑ", class: ir.ClassNormal},
		"\\infin":          {text: "∞", class: ir.ClassNormal},
		"\\isin":           {text: "∈", class: ir.ClassRelation},
		"\\gvertneqq":      {text: "≩", class: ir.ClassRelation},
		"\\lvertneqq":      {text: "≨", class: ir.ClassRelation},
		"\\ngeqq":          {text: "≱", class: ir.ClassRelation},
		"\\ngeqslant":      {text: "≱", class: ir.ClassRelation},
		"\\nleqq":          {text: "≰", class: ir.ClassRelation},
		"\\nleqslant":      {text: "≰", class: ir.ClassRelation},
		"\\nshortmid":      {text: "∤", class: ir.ClassRelation},
		"\\nshortparallel": {text: "∦", class: ir.ClassRelation},
		"\\nsubseteqq":     {text: "⊈", class: ir.ClassRelation},
		"\\nsupseteqq":     {text: "⊉", class: ir.ClassRelation},
		"\\notni":          {text: "∌", class: ir.ClassRelation},
		"\\plusmn":         {text: "±", class: ir.ClassBinary},
		"\\ratio":          {text: ":", class: ir.ClassRelation},
		"\\real":           {text: "ℜ", class: ir.ClassNormal},
		"\\sdot":           {text: "⋅", class: ir.ClassBinary},
		"\\spades":         {text: "♠", class: ir.ClassNormal},
		"\\sub":            {text: "⊂", class: ir.ClassRelation},
		"\\sube":           {text: "⊆", class: ir.ClassRelation},
		"\\supe":           {text: "⊇", class: ir.ClassRelation},
		"\\thetasym":       {text: "ϑ", class: ir.ClassAlphabetic},
		"\\varDelta":       {text: "Δ", class: ir.ClassNormal, variant: ir.VariantItalic},
		"\\varGamma":       {text: "Γ", class: ir.ClassNormal, variant: ir.VariantItalic},
		"\\varLambda":      {text: "Λ", class: ir.ClassNormal, variant: ir.VariantItalic},
		"\\varOmega":       {text: "Ω", class: ir.ClassNormal, variant: ir.VariantItalic},
		"\\varPhi":         {text: "Φ", class: ir.ClassNormal, variant: ir.VariantItalic},
		"\\varPi":          {text: "Π", class: ir.ClassNormal, variant: ir.VariantItalic},
		"\\varPsi":         {text: "Ψ", class: ir.ClassNormal, variant: ir.VariantItalic},
		"\\varSigma":       {text: "Σ", class: ir.ClassNormal, variant: ir.VariantItalic},
		"\\varTheta":       {text: "Θ", class: ir.ClassNormal, variant: ir.VariantItalic},
		"\\varUpsilon":     {text: "Υ", class: ir.ClassNormal, variant: ir.VariantItalic},
		"\\varXi":          {text: "Ξ", class: ir.ClassNormal, variant: ir.VariantItalic},
		"\\varsubsetneq":   {text: "⊊", class: ir.ClassRelation},
		"\\varsubsetneqq":  {text: "⫋", class: ir.ClassRelation},
		"\\varsupsetneq":   {text: "⊋", class: ir.ClassRelation},
		"\\varsupsetneqq":  {text: "⫌", class: ir.ClassRelation},
		"\\weierp":         {text: "℘", class: ir.ClassNormal},
		"\\N":              {text: "ℕ", class: ir.ClassNormal, variant: ir.VariantDoubleStruck},
		"\\natnums":        {text: "ℕ", class: ir.ClassNormal, variant: ir.VariantDoubleStruck},
		"\\Z":              {text: "ℤ", class: ir.ClassNormal, variant: ir.VariantDoubleStruck},
		"\\R":              {text: "ℝ", class: ir.ClassNormal, variant: ir.VariantDoubleStruck},
		"\\reals":          {text: "ℝ", class: ir.ClassNormal, variant: ir.VariantDoubleStruck},
		"\\Reals":          {text: "ℝ", class: ir.ClassNormal, variant: ir.VariantDoubleStruck},
		"\\cnums":          {text: "ℂ", class: ir.ClassNormal, variant: ir.VariantDoubleStruck},
		"\\Complex":        {text: "ℂ", class: ir.ClassNormal, variant: ir.VariantDoubleStruck},
		"\\lq":             {text: "‘", class: ir.ClassNormal},
		"\\rq":             {text: "’", class: ir.ClassNormal},
		"\\AA":             {text: "Å", class: ir.ClassNormal},
		"\\aa":             {text: "å", class: ir.ClassNormal},
		"\\AE":             {text: "Æ", class: ir.ClassNormal},
		"\\ae":             {text: "æ", class: ir.ClassNormal},
		"\\O":              {text: "Ø", class: ir.ClassNormal},
		"\\o":              {text: "ø", class: ir.ClassNormal},
		"\\OE":             {text: "Œ", class: ir.ClassNormal},
		"\\oe":             {text: "œ", class: ir.ClassNormal},
		"\\ss":             {text: "ß", class: ir.ClassNormal},
		"\\copyright":      {text: "©", class: ir.ClassNormal},
		"\\textcopyright":  {text: "©", class: ir.ClassNormal},
		"\\minuso":         {text: "⦵", class: ir.ClassBinary},
	}
	for name, spec := range aliases {
		symbolCommands[name] = spec
	}

	alias := func(names []string, text string, class ir.MathClass) {
		for _, name := range names {
			symbolCommands[name] = symbolSpec{text: text, class: class, variant: ir.VariantAuto}
		}
	}
	alias([]string{"\\larr"}, "←", ir.ClassRelation)
	alias([]string{"\\Larr", "\\lArr"}, "⇐", ir.ClassRelation)
	alias([]string{"\\rarr"}, "→", ir.ClassRelation)
	alias([]string{"\\Rarr", "\\rArr"}, "⇒", ir.ClassRelation)
	alias([]string{"\\harr", "\\lrarr"}, "↔", ir.ClassRelation)
	alias([]string{"\\Harr", "\\hArr", "\\Lrarr", "\\lrArr"}, "⇔", ir.ClassRelation)
	alias([]string{"\\uarr", "\\Uarr"}, "↑", ir.ClassRelation)
	alias([]string{"\\uArr"}, "⇑", ir.ClassRelation)
	alias([]string{"\\darr", "\\Darr"}, "↓", ir.ClassRelation)
	alias([]string{"\\dArr"}, "⇓", ir.ClassRelation)
	alias([]string{"\\lang"}, "⟨", ir.ClassOpening)
	alias([]string{"\\rang"}, "⟩", ir.ClassClosing)
	alias([]string{"\\llbracket"}, "⟦", ir.ClassOpening)
	alias([]string{"\\rrbracket"}, "⟧", ir.ClassClosing)
	alias([]string{"\\llcorner"}, "⌞", ir.ClassOpening)
	alias([]string{"\\lrcorner"}, "⌟", ir.ClassClosing)
	alias([]string{"\\ulcorner"}, "⌜", ir.ClassOpening)
	alias([]string{"\\urcorner"}, "⌝", ir.ClassClosing)
	alias([]string{"\\lBrace"}, "⦃", ir.ClassOpening)
	alias([]string{"\\rBrace"}, "⦄", ir.ClassClosing)
	alias([]string{"\\VERT"}, "⦀", ir.ClassFence)
	alias([]string{"\\dots", "\\dotsc", "\\dotso"}, "…", ir.ClassNormal)
	alias([]string{"\\dotsb", "\\dotsi", "\\dotsm"}, "⋯", ir.ClassNormal)
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
		{"textcircled", "◯", false},
	} {
		accentCommands["\\"+entry.name] = accentSpec{text: entry.text, stretchy: entry.stretch}
	}
}

func addSpaces() {
	mustMuLength := func(value float64) ir.Length {
		length, err := ir.MuLength(value)
		if err != nil {
			panic("invalid built-in TeX math space: " + err.Error())
		}
		return length
	}
	thin := mustMuLength(3)
	medium := mustMuLength(4)
	thick := mustMuLength(5)

	// TeX defines explicit math kerns in mu. Keep that source unit instead of
	// prematurely rounding through a fixed-point em rational.
	spacingCommands["\\,"] = spaceSpec{kind: ir.SpaceExplicit, amount: thin}
	spacingCommands["\\thinspace"] = spacingCommands["\\,"]
	spacingCommands["\\:"] = spaceSpec{kind: ir.SpaceExplicit, amount: medium}
	spacingCommands["\\>"] = spacingCommands["\\:"]
	spacingCommands["\\medspace"] = spacingCommands["\\:"]
	spacingCommands["\\;"] = spaceSpec{kind: ir.SpaceExplicit, amount: thick}
	spacingCommands["\\thickspace"] = spacingCommands["\\;"]
	spacingCommands["\\!"] = spaceSpec{kind: ir.SpaceExplicit, amount: mustMuLength(-3)}
	spacingCommands["\\negthinspace"] = spacingCommands["\\!"]
	spacingCommands["\\negmedspace"] = spaceSpec{kind: ir.SpaceExplicit, amount: mustMuLength(-4)}
	spacingCommands["\\negthickspace"] = spaceSpec{kind: ir.SpaceExplicit, amount: mustMuLength(-5)}
	spacingCommands["\\enspace"] = spaceSpec{kind: ir.SpaceExplicit, amount: mustMuLength(9)}
	spacingCommands["\\enskip"] = spacingCommands["\\enspace"]
	spacingCommands["\\quad"] = spaceSpec{kind: ir.SpaceExplicit, amount: mustMuLength(18)}
	spacingCommands["\\qquad"] = spaceSpec{kind: ir.SpaceExplicit, amount: mustMuLength(36)}
	spacingCommands["\\ "] = spaceSpec{kind: ir.SpaceInterword}
	spacingCommands["\\space"] = spacingCommands["\\ "]
	spacingCommands["\\nobreakspace"] = spacingCommands["\\ "]
}

func addOperators() {
	for _, name := range []string{
		"arccos", "arcsin", "arctan", "arg", "cos", "cosh", "cot", "coth", "csc",
		"deg", "det", "dim", "exp", "gcd", "hom", "inf", "ker", "lg", "lim",
		"liminf", "limsup", "ln", "log", "max", "min", "Pr", "sec", "sin",
		"sinh", "sup", "tan", "tanh", "arcctg", "arctg", "argmax", "argmin",
		"ch", "cosec", "cotg", "ctg", "cth", "injlim", "plim", "projlim",
		"sh", "tg", "th", "varinjlim", "varliminf", "varlimsup", "varprojlim",
	} {
		namedOperators["\\"+name] = name
	}
	for _, name := range []string{
		"inf", "lim", "liminf", "limsup", "max", "min", "Pr", "sup",
		"argmax", "argmin", "injlim", "plim", "projlim", "varinjlim", "varliminf",
		"varlimsup", "varprojlim",
	} {
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
		"\\pmb":        ir.VariantBold,
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

func addSizing() {
	for i, name := range sizingNames {
		sizingCommands["\\"+name] = uint8(i + 1)
		sizingFontSizes["\\"+name] = ir.RawLength(ir.LengthPercent, katexSizePercents[i])
	}
}

var sizingNames = []string{
	"tiny", "sixptsize", "scriptsize", "footnotesize", "small",
	"normalsize", "large", "Large", "LARGE", "huge", "Huge",
}

// KaTeX Options.ts sizeMultipliers, expressed in millionths of a percent.
var katexSizePercents = [...]int64{
	50_000_000,
	60_000_000,
	70_000_000,
	80_000_000,
	90_000_000,
	100_000_000,
	120_000_000,
	144_000_000,
	172_800_000,
	207_400_000,
	248_800_000,
}

func addInfixes() {
	infixCommands["\\over"] = fractionInfix{bar: true, style: ir.FractionVertical}
	infixCommands["\\atop"] = fractionInfix{bar: false, style: ir.FractionVertical, role: ir.FractionRoleStacked}
	infixCommands["\\choose"] = fractionInfix{bar: false, style: ir.FractionVertical, open: "(", close: ")", role: ir.FractionRoleBinomial}
	infixCommands["\\brace"] = fractionInfix{bar: false, style: ir.FractionVertical, open: "{", close: "}", role: ir.FractionRoleStacked}
	infixCommands["\\brack"] = fractionInfix{bar: false, style: ir.FractionVertical, open: "[", close: "]", role: ir.FractionRoleStacked}
	infixCommands["\\above"] = fractionInfix{bar: true, style: ir.FractionVertical, dimension: true}
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
