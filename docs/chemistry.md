# Bounded TeX Chemistry

Untie supports a deliberately bounded subset of KaTeX's `mhchem` extension.
The TeX frontend lowers `\ce{...}` and `\pu{...}` to the existing neutral IR:
upright text, math glyphs, scripts, fences, fractions, explicit spaces, and
error nodes. There is no chemistry-specific IR node and no runtime dependency
on KaTeX, MathJax, mhchem, RaTeX, or a periodic-table database.

This boundary is lexical and structural. Element-like letter sequences are
rendered without asserting that every symbol is a real element. The supported
grammar is intended for formulas, reaction equations, states, and common
physical quantities, not for validating chemistry facts.

## Chemical Formulas

Supported formula syntax includes:

- element-like sequences such as `H2O`, `NaCl`, `Cl2`, and `C6H5-CHO`;
- positive integer, decimal, and simple numeric-fraction coefficients, such as
  `2H2O`, `0.5 H2O`, and `(1/2) H2O`;
- subscripts and superscripts, including charges such as `CrO4^2-`,
  `[AgCl2]-`, `Y^{99+}`, and `Fe^{III}`;
- isotope and prescript forms such as `^{227}_{90}Th+` and
  `^227_90Th+`;
- visible parentheses and brackets, including nested groups and multipliers:
  `(NH4)2S` and `[\{(X2)3\}2]^3+`;
- raw TeX braces as invisible grouping, and escaped `\{` / `\}` as visible
  braces;
- state annotations such as `H2(aq)`, `NaCl(s)`, and `CO3^2-_{(aq)}`;
- additional compounds written with `.` or `*`, such as
  `KCr(SO4)2.12H2O`.
- embedded TeX math segments such as `$n$`, `$x_i$`, and nested `\ce{...}`,
  evaluated through the same bounded TeX parser;
- the mhchem `\ca` approximation mark and the gas/precipitate arrows `^` and
  `v` in standalone positions.

Chemical symbols are rendered upright. Formula digits are ordinary math
glyphs. Scripts retain normal neutral script semantics, so display layout can
apply the same limit and attachment rules as equivalent TeX math.

## Bonds And Reactions

The common bond spellings `-`, `=`, and `#` are accepted between formula
terms. Explicit forms accept their numeric aliases:

| Input | Result |
| --- | --- |
| `\bond{-}`, `\bond{1}` | single bond |
| `\bond{=}`, `\bond{2}` | double bond |
| `\bond{#}`, `\bond{3}` | triple bond |
| `\bond{...}`, `\bond{....}` | dotted bond |
| `\bond{->}`, `\bond{<-}` | directional bond |

Reaction arrows include `->`, `<-`, `<->`, `<-->`, `<=>`, `<=>>`, and
`<<=>`. The second bracket form places an annotation below the arrow:

```tex
\ce{A ->[heat] B}
\ce{A ->[catalyst][heat] B}
```

Arrow annotations lower to `ScriptRoleLimits` with `LimitsAlways`. Source
ranges for the arrow and both annotations remain independently addressable.
Only adjacent bracket annotations form the top and bottom labels; whitespace
after the first annotation starts the next formula term.

## Physical Quantities

`\pu{...}` supports a leading signed number, decimal fractions, scientific
notation, and a unit expression:

```tex
\pu{123 kJ//mol}
\pu{123 kJ mol-1}
\pu{123 mm2}
\pu{1.2e3 J s}
```

It accepts `*` and `.` as multiplication separators and `/` or `//` as
division separators. Unit words are upright. A suffix such as `mm2`,
`mol-1`, or `s^2` becomes a superscript. `\pm`, common symbol commands, `%`,
and `°` are accepted inside the argument.

## Diagnostics And Recovery

Malformed and unsupported chemistry syntax does not disappear:

- an unclosed argument keeps the parsed body and emits `unclosed-group`;
- an empty or missing argument emits a placeholder with a parse diagnostic;
- an unsupported bond kind emits `unsupported` and leaves the bond text
  visible in the error paint;
- an unsupported token or command emits a visible error node and preserves
  the surrounding formula;
- a nonnumeric `\pu` argument emits `invalid-value` and displays the rejected
  source text.

Chemistry parsing shares the TeX source, token, node, nesting, text, and
dimension limits. Error nodes, text clusters, source hits, and carets use the
ordinary display-list paths.

## Accessibility And Canonical IR

The neutral lowering gives chemistry the same accessibility behavior as other
math. `Alt.Text` and `Alt.Speech` are generated from the resulting text,
scripts, fences, fractions, and relations rather than from the raw `\ce`
command.

The result validates, clones, and serializes through the canonical IR without
new node kinds. Text clusters identify the source range of each upright rune;
numbers and operators carry their own glyph ranges.

## Explicit Exclusions

The following remain outside the bounded grammar and are diagnosed instead of
silently approximated:

- periodic-table, oxidation-state, charge-balance, or reaction-balance
  validation;
- global chemistry databases, automatic element-name completion, and
  structure search;
- oxidation-state syntax beyond ordinary superscripts;
- variable or symbolic coefficients beyond the accepted numeric forms;
- arbitrary `\color` commands inside a chemistry argument;
- specialist bonds such as donor/acceptor, aromatic, and multi-level
  coordinate-bond diagrams, including the `\bond{~...}` family;
- measurement-uncertainty notation such as `1.23(4)e5` and correlated
  uncertainty expressions;
- nested document-level chemistry macros or package configuration;
- automatic wording expansion such as element names in accessibility output.

These cases should be added only with a neutral representation and reviewed
fixtures. They must not be implemented as source-string rewrites or as
renderer-side special cases.
