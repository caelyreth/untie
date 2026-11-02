package main

import (
	"reflect"
	"testing"
)

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
