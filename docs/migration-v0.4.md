# Migrating To 0.4

Untie 0.4 expands the neutral IR so high-value TeX layout constructs no longer
require negative-space workarounds or parser-specific layout metadata. The
runtime version is `0.4.0+layout.3`.

## Font-Size Declarations

The KaTeX-compatible declarations `\tiny`, `\sixptsize`, `\scriptsize`,
`\footnotesize`, `\small`, `\normalsize`, `\large`, `\Large`, `\LARGE`,
`\huge`, and `\Huge` now lower to neutral IR.

`ir.Style` gained:

```go
SetFontSize bool
FontSize    ir.Length
```

`FontSize` is a positive percentage of the root host font size. The frontend
uses the exact KaTeX multipliers from 50% through 248.8%. The same wrapper also
resets the math style to text style, matching KaTeX's declaration behavior.

Size declarations scope to the remainder of their source group. A nested
declaration replaces the active absolute size instead of multiplying it, while
scripts laid out under that declaration continue to apply the active MATH
script scale.

Text-mode declarations use the same IR. No separate text-only size state is
vendored into the parser.

## Overlap And Raise Primitives

Two neutral node families replace the old explicit-unsupported behavior:

- `ir.Overlay` places content at a zero-width insertion point with start,
  center, or end alignment;
- `ir.Raise` shifts content vertically by a signed source length without
  changing horizontal advance.

TeX lowering is:

| TeX | Neutral IR |
| --- | --- |
| `\mathrlap`, `\rlap` | `Overlay` with start alignment |
| `\mathclap`, `\clap` | `Overlay` with center alignment |
| `\mathllap`, `\llap` | `Overlay` with end alignment |
| `\raisebox{length}{body}` | `Raise` with a text-mode body |

The non-`math` TeX forms parse their argument in text mode. `\raisebox` also
requires a text-mode hbox body. Overlay ink can extend outside the zero-width
box and is represented through the normal `Box.Overflow` contract.

`\vcenter`, `\phase`, `\enclose`, and other constructs without a lossless
neutral representation remain explicit unsupported diagnostics.

## Multiline Row Tags

`\tag`, `\tag*`, `\nonumber`, and `\notag` now work in the direct row scope of
supported multiline environments. `ir.EquationTag.Line` is the zero-based
source row; `ir.FormulaTagLine` remains the formula-level value.

Validation now rejects:

- mixing formula-level and row-level tags;
- duplicate tag events on one row;
- row events that are not in strictly increasing row order.

The display contract distinguishes the two scopes:

- `Box.Tag` remains the single formula-level tag;
- `Box.Tags` contains row-scoped `TagBox` values with `Line >= 0`;
- `Box.Tag` and `Box.Tags` are mutually exclusive;
- each row tag's `Items` and `Hits` ranges occupy a contiguous parent-frame
  suffix, ordered by source row;
- constrained display reserves a right-aligned tag column and keeps the
  multiline formula block as one unbreakable line.

Nested environments, tags inside ordinary groups or syntactic arguments, tags
in inline math, and automatic document counters remain outside the contract.

## Cache Identity

The library version, layout revision, and process-wide Engine cache schema
changed:

- `untie.Version()` is now `0.4.0+layout.3`;
- the Engine cache schema is `3`.

Drop persisted layout or display caches created by 0.3.
