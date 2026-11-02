package main

import (
	"reflect"
	"testing"
)

func TestParseKaTeXMacroFallbacks(t *testing.T) {
	t.Parallel()

	src := `
defineMacro("\\gvertneqq", "\\html@mathml{\\@gvertneqq}{\u2269}");
defineMacro("\\imath", "\\html@mathml{\\@imath}{\u0131}");
defineMacro("\\sin", "\\operatorname{sin}");
`
	got, err := parseKaTeXMacroFallbacks(src)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]string{
		`\@gvertneqq`: "≩",
		`\@imath`:     "ı",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("fallbacks = %#v, want %#v", got, want)
	}
}

func TestParseTeXSymbolsSeparatesPrivateUseText(t *testing.T) {
	t.Parallel()

	src := `
defineSymbol(math, ams, rel, "\ue00d", "\\@gvertneqq");
defineSymbol(math, main, mathord, "\ue131", "\\@imath");
defineSymbol(math, main, rel, "\ue020", "\\@not");
defineSymbol(math, main, inner, "\u22ef", "\\@cdots", true);
defineSymbol(math, main, mathord, "\u03b1", "\\alpha", true);
`
	fallbacks := map[string]string{
		`\@gvertneqq`: "≩",
		`\@imath`:     "ı",
	}
	got, err := parseTeXSymbols(src, fallbacks)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 5 {
		t.Fatalf("symbols = %d, want 5", len(got))
	}
	tests := []struct {
		command     string
		text        string
		kind        string
		sourceGlyph rune
	}{
		{command: `\@cdots`, text: "⋯", kind: "Internal"},
		{command: `\@gvertneqq`, text: "≩", kind: "Internal", sourceGlyph: 0xe00d},
		{command: `\@imath`, text: "ı", kind: "Internal", sourceGlyph: 0xe131},
		{command: `\@not`, text: "", kind: "Internal", sourceGlyph: 0xe020},
		{command: `\alpha`, text: "α", kind: "Public"},
	}
	for i, test := range tests {
		symbol := got[i]
		if symbol.command != test.command || symbol.text != test.text ||
			symbol.kind != test.kind || symbol.sourceGlyph != test.sourceGlyph {
			t.Errorf("%s = %#v, want %#v", test.command, symbol, test)
		}
	}
}

func TestParseTeXSymbolsIgnoresFallbackForUnicodeInternal(t *testing.T) {
	t.Parallel()

	src := `defineSymbol(math, main, inner, "\u22ef", "\\@cdots");`
	got, err := parseTeXSymbols(src, map[string]string{`\@cdots`: "wrong"})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].text != "⋯" {
		t.Fatalf("symbols = %#v", got)
	}
}

func TestArrayAfterMarkerSkipsTypeBrackets(t *testing.T) {
	t.Parallel()

	src := `export const VALUES: readonly (readonly [string, string])[] = [["a", "b"]];`
	got, err := arrayAfterMarker(src, "VALUES")
	if err != nil {
		t.Fatal(err)
	}
	if want := `["a", "b"]`; got != want {
		t.Fatalf("array body = %q, want %q", got, want)
	}
}

func TestDecodeRustString(t *testing.T) {
	t.Parallel()

	got, err := decodeRustString(`lim\u{2009}inf\t\\`)
	if err != nil {
		t.Fatal(err)
	}
	if want := "lim\u2009inf\t\\"; got != want {
		t.Fatalf("decoded = %q, want %q", got, want)
	}
}

func TestParseShorthands(t *testing.T) {
	t.Parallel()

	src := `
impl MathShorthand<'_> {
    pub const LIST: &'static [(&'static str, char)] = &[
        ("...", '…'),
        ("->", '→'),
        ("|->", '↦'),
    ];
}
`
	got, err := parseShorthands(src)
	if err != nil {
		t.Fatal(err)
	}
	want := []shorthand{
		{syntax: "->", value: "→"},
		{syntax: "...", value: "…"},
		{syntax: "|->", value: "↦"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("shorthands = %#v, want %#v", got, want)
	}
}

func TestParseTypstOperators(t *testing.T) {
	t.Parallel()

	src := `
ops! {
    lim (limits),
    liminf: "lim\u{2009}inf" (limits),
    sin,
}
`
	got, err := parseTypstOperators(src)
	if err != nil {
		t.Fatal(err)
	}
	want := []operator{
		{name: "lim", text: "lim", limits: true},
		{name: "liminf", text: "lim\u2009inf", limits: true},
		{name: "sin", text: "sin"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("operators = %#v, want %#v", got, want)
	}
}

func TestParseClasses(t *testing.T) {
	t.Parallel()

	runs := `export const MATH_CLASS_RUNS: readonly number[] = [65, 1, 0, 0, 1, 10];`
	overrides := `
export function defaultMathClass(c: number): MathClass | undefined {
    switch (c) {
        case 0x3a:
            return MathClass.Relation;
        default:
            return unicodeMathClass(c);
    }
}
`
	ranges, gotOverrides, err := parseClasses(runs, overrides)
	if err != nil {
		t.Fatal(err)
	}
	wantRanges := []classRange{
		{start: 'A', end: 'B', class: "Normal"},
		{start: 'B', end: 'C', class: "Relation"},
	}
	wantOverrides := []classOverride{{r: ':', name: "Relation"}}
	if !reflect.DeepEqual(ranges, wantRanges) {
		t.Fatalf("ranges = %#v, want %#v", ranges, wantRanges)
	}
	if !reflect.DeepEqual(gotOverrides, wantOverrides) {
		t.Fatalf("overrides = %#v, want %#v", gotOverrides, wantOverrides)
	}
}
