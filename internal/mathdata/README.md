# Math Data

`internal/mathdata` is the shared, generated metadata layer for Untie's TeX and
Typst frontends and for the font layer. It contains data only: no parser, no
semantic IR, no font handles, and no layout measurements.

## Contract

The package has six narrow data families:

| Family | API | Generated size |
|---|---|---|
| Unicode math classes | `UnicodeClass`, `DefaultClass` | 402 ranges, 2,736 code points |
| Math alphanumerics | `VariantRune`, `ApplyVariant`, `ApplyVariantString` | 16 styles, 45 exceptions |
| TeX symbols | `LookupTeXSymbol` | 490 no-argument math symbol commands |
| Typst symbols | `LookupTypstSymbol`, `LookupTypstSymbolVariant` | 334 families, 1,206 variants |
| Typst shorthands | `LookupTypstShorthand`, `MatchTypstShorthandPrefix` | 38 |
| Operators | `LookupTeXOperator`, `LookupTypstOperator` | 60 TeX, 42 Typst |

All backing tables are package-private. APIs return value copies, so callers
cannot mutate process-global data. Lookups use binary search wherever exact-key
ordering is meaningful. The shorthand prefix helper is linear over its
deliberately small table and chooses the longest match.

`Class` values match Typst's `unicode_math_class::MathClass` numeric order.
`DefaultClass` applies Typst's documented overrides; `UnicodeClass` returns the
raw MathClass.txt result.

`AlphaVariant` distinguishes calligraphic/chancery from script/roundhand because
their Unicode base letters are often shared and the difference is a variation
selector. A failed variant transform is non-destructive: it returns the original
rune and `ok == false`.

The TeX symbol table records the original KaTeX `group`, normalized `Class`,
source font class, and the `acceptUnicodeChar` metadata. `Kind` marks KaTeX's
private `\@...` implementation commands as internal. `Text` contains only
portable Unicode; a source-only KaTeX private-use code point is retained in
`SourceGlyph` for provenance and must not be rendered directly. Where KaTeX's
`macros.ts` supplies a MathML alternative for a private-use glyph, `Text`
contains that portable scalar. Commands with no standalone portable scalar,
such as `\@not`, have empty `Text`.

The Typst table preserves deprecation messages as metadata; deprecated paths
still resolve so a frontend can emit the diagnostic while rendering them.

## Non-Goals

- Table entries are not a support promise. Frontends still implement semantics,
  macro expansion, arity, diagnostics, and layout.
- The TeX table contains no-argument literal symbol commands, not macro
  expansions or higher-level command implementations.
- Internal TeX commands remain visible as metadata because macro lowering may
  need them. They are not part of the public TeX command surface.
- Typst symbols and shorthands are resolution metadata, not a Typst evaluator.
- Unicode variant lookup does not invent a glyph when Unicode has no direct
  mapping. The caller owns the documented fallback.
- Operator tables contain statically registered, no-argument operators. Operators
  synthesized by macros or arbitrary `operatorname` content are handled by the
  frontends.
- This package does not read reference repositories at build or runtime.

## Regeneration

From the repository root, with the pinned `references/` checkout available:

```sh
nix develop --command go generate ./internal/mathdata
```

The directive expands to:

```sh
go run ./gen -refs ../../references -out .
```

The generator reads only the files listed in `PROVENANCE.md`, validates expected
relationships, sorts all tables deterministically, formats the result with
`go/format`, and writes the five `*_generated.go` files. It does not compile or
execute KaTeX, Typst, Typlet, JavaScript, Rust, or any reference build.

`references/` is locally ignored and absent in CI. Normal builds and tests use
only checked-in Go files. Regeneration is a reviewed maintainer operation, not a
build step.

When a pinned upstream changes:

1. Update all affected source pins and hashes in `PROVENANCE.md`.
2. Regenerate.
3. Review the source-hash and table diffs.
4. Update exact-count tests only when the upstream change is understood.
5. Add or update a representative semantic test for every new data family.

Do not edit generated files by hand. Do not update a generated table to make an
unrelated parser or layout test pass.
