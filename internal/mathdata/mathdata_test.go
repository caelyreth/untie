package mathdata

import (
	"slices"
	"testing"
)

func TestClassLookup(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		r     rune
		class Class
		ok    bool
	}{
		{name: "space", r: ' ', class: ClassSpace, ok: true},
		{name: "opening", r: '(', class: ClassOpening, ok: true},
		{name: "closing", r: ')', class: ClassClosing, ok: true},
		{name: "punctuation", r: ',', class: ClassPunctuation, ok: true},
		{name: "relation", r: '=', class: ClassRelation, ok: true},
		{name: "greek", r: 'α', class: ClassAlphabetic, ok: true},
		{name: "large", r: '∑', class: ClassLarge, ok: true},
		{name: "arrow", r: '→', class: ClassRelation, ok: true},
		{name: "unassigned", r: '\u0378', ok: false},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			class, ok := UnicodeClass(test.r)
			if ok != test.ok {
				t.Fatalf("UnicodeClass(%U) ok = %t, want %t", test.r, ok, test.ok)
			}
			if class != test.class {
				t.Fatalf("UnicodeClass(%U) = %s, want %s", test.r, class, test.class)
			}
		})
	}
}

func TestDefaultClassOverrides(t *testing.T) {
	t.Parallel()

	tests := []struct {
		r     rune
		class Class
	}{
		{r: ':', class: ClassRelation},
		{r: '.', class: ClassNormal},
		{r: '/', class: ClassNormal},
		{r: '⊥', class: ClassNormal},
		{r: '⋯', class: ClassNormal},
		{r: '⎰', class: ClassOpening},
		{r: '⎱', class: ClassClosing},
		{r: '،', class: ClassPunctuation},
	}
	for _, test := range tests {
		class, ok := DefaultClass(test.r)
		if !ok {
			t.Fatalf("DefaultClass(%U) was not found", test.r)
		}
		if class != test.class {
			t.Errorf("DefaultClass(%U) = %s, want %s", test.r, class, test.class)
		}
	}

	raw, ok := UnicodeClass('⊥')
	if !ok {
		t.Fatal("UnicodeClass(U+22A5) was not found")
	}
	if raw != ClassRelation {
		t.Fatalf("UnicodeClass(U+22A5) = %s, want relation", raw)
	}
}

func TestClassTableShape(t *testing.T) {
	t.Parallel()

	if got := len(unicodeClassRanges); got != 402 {
		t.Fatalf("class ranges = %d, want 402", got)
	}
	if got := len(defaultClassOverrides); got != 15 {
		t.Fatalf("default overrides = %d, want 15", got)
	}

	codePoints := 0
	for i, classRange := range unicodeClassRanges {
		if classRange.start >= classRange.end {
			t.Fatalf("range %d is empty or reversed: %#v", i, classRange)
		}
		if i > 0 && unicodeClassRanges[i-1].end > classRange.start {
			t.Fatalf("ranges %d and %d overlap", i-1, i)
		}
		if classRange.class > ClassSpecial {
			t.Fatalf("range %d has invalid class %d", i, classRange.class)
		}
		codePoints += int(classRange.end - classRange.start)
	}
	if codePoints != 2736 {
		t.Fatalf("classified code points = %d, want 2736", codePoints)
	}
	for i := 1; i < len(defaultClassOverrides); i++ {
		if defaultClassOverrides[i-1].rune >= defaultClassOverrides[i].rune {
			t.Fatalf("default overrides are not strictly sorted at %d", i)
		}
	}
}

func TestClassString(t *testing.T) {
	t.Parallel()

	tests := []struct {
		class Class
		want  string
	}{
		{ClassNormal, "normal"},
		{ClassAlphabetic, "alphabetic"},
		{ClassBinary, "binary"},
		{ClassClosing, "closing"},
		{ClassDiacritic, "diacritic"},
		{ClassFence, "fence"},
		{ClassGlyphPart, "glyph-part"},
		{ClassLarge, "large"},
		{ClassOpening, "opening"},
		{ClassPunctuation, "punctuation"},
		{ClassRelation, "relation"},
		{ClassSpace, "space"},
		{ClassUnary, "unary"},
		{ClassVary, "vary"},
		{ClassSpecial, "special"},
		{Class(255), "invalid"},
	}
	for _, test := range tests {
		if got := test.class.String(); got != test.want {
			t.Errorf("Class(%d).String() = %q, want %q", test.class, got, test.want)
		}
	}
}

func TestVariantRune(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		r        rune
		variant  AlphaVariant
		want     rune
		selector rune
		ok       bool
	}{
		{name: "plain", r: 'A', variant: VariantPlain, want: 'A', ok: true},
		{name: "bold upper", r: 'A', variant: VariantBold, want: '𝐀', ok: true},
		{name: "bold lower", r: 'a', variant: VariantBold, want: '𝐚', ok: true},
		{name: "bold digit", r: '1', variant: VariantBold, want: '𝟏', ok: true},
		{name: "bold alpha", r: 'α', variant: VariantBold, want: '𝛂', ok: true},
		{name: "bold partial", r: '∂', variant: VariantBold, want: '𝛛', ok: true},
		{name: "italic h exception", r: 'h', variant: VariantItalic, want: 'ℎ', ok: true},
		{name: "double-struck C", r: 'C', variant: VariantDoubleStruck, want: 'ℂ', ok: true},
		{name: "double-struck Gamma", r: 'Γ', variant: VariantDoubleStruck, want: 'ℾ', ok: true},
		{name: "double-struck pi", r: 'π', variant: VariantDoubleStruck, want: 'ℼ', ok: true},
		{name: "script", r: 'A', variant: VariantScript, want: '𝒜', selector: 0xfe01, ok: true},
		{name: "script exception", r: 'B', variant: VariantScript, want: 'ℬ', selector: 0xfe01, ok: true},
		{name: "calligraphic", r: 'A', variant: VariantCalligraphic, want: '𝒜', selector: 0xfe00, ok: true},
		{name: "fraktur", r: 'A', variant: VariantFraktur, want: '𝔄', ok: true},
		{name: "unsupported", r: 'α', variant: VariantSansSerif, ok: false},
		{name: "invalid variant", r: 'A', variant: AlphaVariant(255), ok: false},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			got, selector, ok := VariantRune(test.r, test.variant)
			if ok != test.ok || got != test.want || selector != test.selector {
				t.Fatalf("VariantRune(%U, %s) = (%U, %U, %t), want (%U, %U, %t)",
					test.r, test.variant, got, selector, ok, test.want, test.selector, test.ok)
			}
		})
	}
}

func TestApplyVariant(t *testing.T) {
	t.Parallel()

	got, ok := ApplyVariant('A', VariantScript)
	if want := "𝒜\ufe01"; got != want || !ok {
		t.Fatalf("ApplyVariant = (%q, %t), want (%q, true)", got, ok, want)
	}
	got, ok = ApplyVariant('α', VariantSansSerif)
	if got != "α" || ok {
		t.Fatalf("unsupported ApplyVariant = (%q, %t), want (\"α\", false)", got, ok)
	}

	got, ok = ApplyVariantString("Aα1", VariantBold)
	if want := "𝐀𝛂𝟏"; got != want || !ok {
		t.Fatalf("ApplyVariantString = (%q, %t), want (%q, true)", got, ok, want)
	}
	got, ok = ApplyVariantString("Aα", VariantSansSerif)
	if want := "𝖠α"; got != want || ok {
		t.Fatalf("partial ApplyVariantString = (%q, %t), want (%q, false)", got, ok, want)
	}
}

func TestVariantTableShape(t *testing.T) {
	t.Parallel()

	if got := len(alphaVariantData); got != int(VariantBoldCalligraphic)+1 {
		t.Fatalf("variant data entries = %d", got)
	}
	if got := len(alphaVariantExceptions); got != 45 {
		t.Fatalf("variant exceptions = %d, want 45", got)
	}
	if got := len(greekVariantExtras); got != 9 {
		t.Fatalf("Greek extras = %d, want 9", got)
	}
	for i := 1; i < len(alphaVariantExceptions); i++ {
		prev, current := alphaVariantExceptions[i-1], alphaVariantExceptions[i]
		if prev.variant > current.variant ||
			prev.variant == current.variant && prev.input >= current.input {
			t.Fatalf("variant exceptions are not strictly sorted at %d", i)
		}
	}
	for i := 1; i < len(greekVariantExtras); i++ {
		if greekVariantExtras[i-1].input >= greekVariantExtras[i].input {
			t.Fatalf("Greek extras are not strictly sorted at %d", i)
		}
	}
}

func TestAlphaVariantString(t *testing.T) {
	t.Parallel()

	tests := []struct {
		variant AlphaVariant
		want    string
	}{
		{VariantPlain, "plain"},
		{VariantBold, "bold"},
		{VariantItalic, "italic"},
		{VariantBoldItalic, "bold-italic"},
		{VariantDoubleStruck, "double-struck"},
		{VariantScript, "roundhand"},
		{VariantBoldScript, "bold-roundhand"},
		{VariantFraktur, "fraktur"},
		{VariantBoldFraktur, "bold-fraktur"},
		{VariantSansSerif, "sans-serif"},
		{VariantSansSerifBold, "sans-serif-bold"},
		{VariantSansSerifItalic, "sans-serif-italic"},
		{VariantSansSerifBoldItalic, "sans-serif-bold-italic"},
		{VariantMonospace, "monospace"},
		{VariantCalligraphic, "chancery"},
		{VariantBoldCalligraphic, "bold-chancery"},
		{AlphaVariant(255), "invalid"},
	}
	for _, test := range tests {
		if got := test.variant.String(); got != test.want {
			t.Errorf("AlphaVariant(%d).String() = %q, want %q", test.variant, got, test.want)
		}
	}
}

func TestTeXSymbols(t *testing.T) {
	t.Parallel()

	tests := []struct {
		command string
		text    string
		class   Class
		group   TeXGroup
		font    TeXFont
	}{
		{command: `\alpha`, text: "α", class: ClassAlphabetic, group: TeXGroupMathOrd, font: TeXFontMain},
		{command: `\Gamma`, text: "Γ", class: ClassNormal, group: TeXGroupTextOrd, font: TeXFontMain},
		{command: `\leq`, text: "≤", class: ClassRelation, group: TeXGroupRelation, font: TeXFontMain},
		{command: `\le`, text: "≤", class: ClassRelation, group: TeXGroupRelation, font: TeXFontMain},
		{command: `\rightarrow`, text: "→", class: ClassRelation, group: TeXGroupRelation, font: TeXFontMain},
		{command: `\infty`, text: "∞", class: ClassNormal, group: TeXGroupTextOrd, font: TeXFontMain},
	}
	for _, test := range tests {
		symbol, ok := LookupTeXSymbol(test.command)
		if !ok {
			t.Fatalf("LookupTeXSymbol(%q) was not found", test.command)
		}
		if symbol.Text != test.text || symbol.Class != test.class ||
			symbol.Group != test.group || symbol.Font != test.font {
			t.Errorf("%s = %#v", test.command, symbol)
		}
	}

	le, _ := LookupTeXSymbol(`\le`)
	leq, _ := LookupTeXSymbol(`\leq`)
	if le.Text != leq.Text || le.Class != leq.Class ||
		le.Group != leq.Group || le.Font != leq.Font {
		t.Fatalf("\\le and \\leq aliases differ: %#v != %#v", le, leq)
	}
	if _, ok := LookupTeXSymbol(`\definitely-not-a-command`); ok {
		t.Fatal("unknown TeX command was found")
	}
}

func TestTeXSymbolTableShape(t *testing.T) {
	t.Parallel()

	if got := len(texSymbols); got != 490 {
		t.Fatalf("TeX symbols = %d, want 490", got)
	}
	for i, symbol := range texSymbols {
		if i > 0 && texSymbols[i-1].Command >= symbol.Command {
			t.Fatalf("TeX symbols are not strictly sorted at %d", i)
		}
		if symbol.Command == "" || symbol.Class > ClassSpecial ||
			symbol.Group > TeXGroupSpacing ||
			(symbol.Font != TeXFontMain && symbol.Font != TeXFontAMS) {
			t.Fatalf("invalid TeX symbol at %d: %#v", i, symbol)
		}
	}
}

func TestTypstSymbols(t *testing.T) {
	t.Parallel()

	tests := []struct {
		path  string
		value string
	}{
		{path: "alpha", value: "α"},
		{path: "Delta", value: "Δ"},
		{path: "arrow.r", value: "→"},
		{path: "arrow.r.long", value: "⟶"},
		{path: "integral", value: "∫"},
		{path: "sum", value: "∑"},
	}
	for _, test := range tests {
		symbol, ok := LookupTypstSymbol(test.path)
		if !ok {
			t.Fatalf("LookupTypstSymbol(%q) was not found", test.path)
		}
		if symbol.Value != test.value {
			t.Errorf("%s = %q, want %q", test.path, symbol.Value, test.value)
		}
	}
	symbol, ok := LookupTypstSymbolVariant("arrow", "r")
	if !ok || symbol.Path != "arrow.r" || symbol.Modifier != "r" {
		t.Fatalf("LookupTypstSymbolVariant = (%#v, %t)", symbol, ok)
	}

	deprecated, ok := LookupTypstSymbol("gt.tri")
	if !ok || deprecated.Deprecation == "" {
		t.Fatalf("deprecated gt.tri = (%#v, %t)", deprecated, ok)
	}
	binding, ok := LookupTypstSymbol("join")
	if !ok || binding.Deprecation == "" {
		t.Fatalf("deprecated join = (%#v, %t)", binding, ok)
	}
	if _, ok := LookupTypstSymbol("not.a.symbol"); ok {
		t.Fatal("unknown Typst symbol was found")
	}
}

func TestTypstSymbolTableShape(t *testing.T) {
	t.Parallel()

	if got := len(typstSymbols); got != 1206 {
		t.Fatalf("Typst symbols = %d, want 1206", got)
	}
	families := map[string]bool{}
	for i, symbol := range typstSymbols {
		if i > 0 && typstSymbols[i-1].Path >= symbol.Path {
			t.Fatalf("Typst symbols are not strictly sorted at %d", i)
		}
		if symbol.Name == "" || symbol.Value == "" {
			t.Fatalf("invalid Typst symbol at %d: %#v", i, symbol)
		}
		families[symbol.Name] = true
	}
	if got := len(families); got != 334 {
		t.Fatalf("Typst symbol families = %d, want 334", got)
	}
}

func TestTypstShorthands(t *testing.T) {
	t.Parallel()

	tests := []struct {
		syntax string
		value  string
	}{
		{syntax: "...", value: "…"},
		{syntax: "<=", value: "≤"},
		{syntax: "->", value: "→"},
		{syntax: "|->", value: "↦"},
		{syntax: "<==>", value: "⟺"},
		{syntax: "[|", value: "⟦"},
	}
	for _, test := range tests {
		shorthand, ok := LookupTypstShorthand(test.syntax)
		if !ok || shorthand.Value != test.value {
			t.Errorf("LookupTypstShorthand(%q) = (%#v, %t)", test.syntax, shorthand, ok)
		}
	}

	match, ok := MatchTypstShorthandPrefix("->> x")
	if !ok || match.Syntax != "->>" || match.Value != "↠" {
		t.Fatalf("longest shorthand = (%#v, %t)", match, ok)
	}
	match, ok = MatchTypstShorthandPrefix("<==>x")
	if !ok || match.Syntax != "<==>" {
		t.Fatalf("longest comparison shorthand = (%#v, %t)", match, ok)
	}
	if _, ok := MatchTypstShorthandPrefix("name"); ok {
		t.Fatal("non-shorthand matched")
	}
	if len(typstShorthands) != 38 {
		t.Fatalf("Typst shorthands = %d, want 38", len(typstShorthands))
	}
	if !slices.IsSortedFunc(typstShorthands[:], func(a, b TypstShorthand) int {
		switch {
		case a.Syntax < b.Syntax:
			return -1
		case a.Syntax > b.Syntax:
			return 1
		default:
			return 0
		}
	}) {
		t.Fatal("Typst shorthands are not sorted")
	}
}

func TestOperators(t *testing.T) {
	t.Parallel()

	texTests := []struct {
		name   string
		text   string
		limits bool
		symbol bool
	}{
		{name: `\sin`, text: "sin"},
		{name: `\lim`, text: "lim", limits: true},
		{name: `\sum`, limits: true, symbol: true},
		{name: `\int`, symbol: true},
	}
	for _, test := range texTests {
		op, ok := LookupTeXOperator(test.name)
		if !ok {
			t.Fatalf("LookupTeXOperator(%q) was not found", test.name)
		}
		if op.Text != test.text || op.Limits != test.limits || op.Symbol != test.symbol {
			t.Errorf("%s = %#v", test.name, op)
		}
	}

	typstTests := []struct {
		name   string
		text   string
		limits bool
	}{
		{name: "sin", text: "sin"},
		{name: "lim", text: "lim", limits: true},
		{name: "liminf", text: "lim\u2009inf", limits: true},
	}
	for _, test := range typstTests {
		op, ok := LookupTypstOperator(test.name)
		if !ok {
			t.Fatalf("LookupTypstOperator(%q) was not found", test.name)
		}
		if op.Text != test.text || op.Limits != test.limits || op.Symbol {
			t.Errorf("%s = %#v", test.name, op)
		}
	}
	if _, ok := LookupTypstOperator("sum"); ok {
		t.Fatal("Typst sum is a symbol, not a text operator")
	}
}

func TestOperatorTableShape(t *testing.T) {
	t.Parallel()

	if got := len(texOperators); got != 60 {
		t.Fatalf("TeX operators = %d, want 60", got)
	}
	if got := len(typstOperators); got != 42 {
		t.Fatalf("Typst operators = %d, want 42", got)
	}
	assertOperatorOrder(t, "TeX", texOperators[:])
	assertOperatorOrder(t, "Typst", typstOperators[:])
}

func assertOperatorOrder(t *testing.T, dialect string, ops []Operator) {
	t.Helper()
	for i, op := range ops {
		if op.Name == "" {
			t.Fatalf("%s operator %d has an empty name", dialect, i)
		}
		if i > 0 && ops[i-1].Name >= op.Name {
			t.Fatalf("%s operators are not strictly sorted at %d", dialect, i)
		}
	}
}

func FuzzLookups(f *testing.F) {
	f.Add("alpha")
	f.Add(`\alpha`)
	f.Add("->")
	f.Add("∑")
	f.Fuzz(func(t *testing.T, text string) {
		_, _ = LookupTeXSymbol(text)
		_, _ = LookupTypstSymbol(text)
		_, _ = LookupTypstShorthand(text)
		_, _ = MatchTypstShorthandPrefix(text)
		_, _ = LookupTeXOperator(text)
		_, _ = LookupTypstOperator(text)
		for _, r := range text {
			_, _ = UnicodeClass(r)
			_, _ = DefaultClass(r)
			for variant := VariantPlain; variant <= VariantBoldCalligraphic; variant++ {
				_, _, _ = VariantRune(r, variant)
			}
		}
	})
}

func BenchmarkUnicodeClass(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_, _ = UnicodeClass('≤')
	}
}

func BenchmarkLookupTypstSymbol(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_, _ = LookupTypstSymbol("arrow.r.long")
	}
}
