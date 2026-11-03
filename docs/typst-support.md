# Supported Typst

Untie accepts the source of a Typst math expression, not a complete Typst
document. It implements a restricted evaluator over math syntax and selected
code-mode values. It does not execute the Typst language.

## Source Shape

Pass math content directly:

```go
expr := untie.ParseTypst(
	`(a+b)/(2a) + sqrt(x^2+y^2)`,
	untie.Display,
	untie.ParseOptions{},
)
```

Do not include document markup or `$...$` delimiters unless the surrounding
application strips them. Inline `$...$` and block `$ ... $` are document
syntax, not part of the expression string accepted by the frontend.

## Math Syntax

Supported and tested:

- Unicode math characters, identifiers, Greek names and aliases;
- basic numeric literals and fractions such as `1/2`;
- superscripts, subscripts, primes, and multiline math rows;
- relation, binary, punctuation, and large-operator spacing;
- supported Typst shorthands, including `<=`, `>=`, `!=`,
  `->`, `and`, and `in`;
- `sqrt`, `root`, `frac`, and `binom`;
- `attach`, `scripts`, and `limits` for explicit script/limit semantics;
- `lr`, `mid`, literal fences, `abs`, `norm`, `floor`, `ceil`, and `round`;
- `mat`, `vec`, and `cases`;
- accents, `accent`, `overline`, `underline`, and brace/bracket decorations;
- `cancel`, `text`, `strong`, `emph`, `box`, and `hide`;
- alphabet/style calls: `bold`, `italic`, `upright`, `serif`, `sans`, `mono`,
  `bb`, `cal`, `scr`, and `frak`;
- math style calls: `display`, `inline`, `script`, and `sscript`;
- `class`, `op`, `stretch`, and `primes`;
- spacing names such as `thin`, `med`, `thick`, `quad`, and `wide`;
- code-mode values used by the supported calls, including lengths, colors,
  booleans, `none`, arrays used by tested multi-delimiter forms, and selected
  dictionaries.

The symbol catalogue is substantially larger than the examples above.

## Common Constructs

```typst
x_i^2 + f'' + sum_(i=1)^n i
integral_0^1 x^2 dif x
lim_(x -> 0) (sin x)/x
sqrt(x) + root(3, x) + 1/2
lr([x/y]) + abs(x) + norm(y)
mat(1, 2; 3, 4)
vec(1, 2, 3)
cases(x & "if" x > 0, -x & "otherwise")
hat(x) + tilde(a) + dot(c) + arrow(e)
overbrace(x+y, n) + underbrace(x+y, n)
bold(A) + italic(Gamma) + bb(R) + cal(A)
text(fill: #red, "word") + #hide[$x$]
```

## Code Mode

`#` enters the restricted evaluator. Tested examples include:

- `#math.frac(1, 2)` and `#std.math.frac(1, 2)`;
- `#sym.arrow.r` and `#std.sym.alpha`;
- `#red`, `#rgb(...)`, `#luma(...)`;
- `#1em` and simple numeric lengths;
- `#none`, `#auto`, `#true`, and `#false`;
- dictionaries and arrays in supported argument shapes.

Bare code values are not the same as math atoms. For example, bare `red` in
math mode is an unknown variable, while `#red` is a code value that can be
lowered to a visible color description where the syntax permits.

## Delimiters and Tables

`lr(...)` creates paired fences. A single delimiter can use an explicit size:

```typst
lr(|, size: #2em)
```

`mat`, `vec`, and `cases` accept the tested positional and named forms,
including delimiters, augmentation, row/column gaps, and alignment metadata.
Not every Typst matrix styling option has a neutral IR representation; an
unsupported option produces a warning or syntax diagnostic rather than a
silent visual claim.

## Styling and Content

The frontend lowers supported content calls into neutral text, color, box,
phantom, line, accent, and class nodes. A text run still requires glyph
coverage in the host font set.

For example:

- `text(fill: #blue, "place")` preserves text and paint;
- `box(stroke: 0.5pt, inset: 3pt, $x$)` lowers supported padding/stroke fields;
- `hide[$x$]` becomes a phantom-like hidden body;
- unsupported fields such as `width`, `height`, `baseline`, `outset`, and
  `clip` produce a warning and do not become host-specific drawing
  instructions.

## Links

Typst links are text-only:

```typst
#link("https://example.org", $x^2$)
```

The visible body becomes normal IR, the destination is discarded, and a valid
link emits one `unsupported` warning. If the body is omitted, the visible
destination text is rendered. Empty, non-string, or otherwise invalid link
arguments produce normal argument diagnostics.

## Explicit Exclusions

The restricted frontend does not execute:

- `#let`, `#set`, `#show`, `#import`, `#include`, `#if`, `#while`, or `#for`;
- user-defined function bodies or arbitrary code blocks;
- preamble definitions;
- loops that compute formulas;
- document layout, page styles, references, bibliographies, or external
  resources;
- arbitrary Typst package evaluation;
- interactive or network operations.

Those forms produce `unsupported` diagnostics and a bounded recovery tree.

The following are also boundaries rather than complete-language claims:

- full Typst value semantics outside selected code values;
- all named arguments for every Typst function;
- all matrix/table styling and stroke fields;
- equation numbering and tags (the current tag contract is TeX Phase 1);
- every parser or concrete-syntax-tree stress probe from upstream Typst.

The tracked conformance policy records reviewed exclusions and known
limitations. At this revision, selected Typlet syntax probes involving BOM
handling, escape forms, and numeric suffixes/overflow remain reviewed gaps, and
scientific notation should not be assumed complete. They recover with
diagnostics rather than being treated as evidence of full Typst support.

## Differences from TeX

Equivalent formulas can share IR while source-specific syntax remains visible
to diagnostics and source hits. For example:

```text
TeX:   \frac{a+b}{2}
Typst: (a+b)/2
```

Both can lower to the same semantic fraction, but the source ranges, diagnostic
spans, and source-hit text remain dialect-specific.

Typst's `math` namespace is not a valid bare math-mode atom in the pinned
behavior. Use `#math` or `#std.math` in code mode. Likewise, `#sym.arrow.r`
works while bare `sym.arrow.r` is an unknown variable.

## What "Supported" Means

A Typst construct is documented here only when it is:

- accepted by the restricted parser/evaluator;
- lowered to validated canonical IR;
- covered by compatibility evidence; and
- assigned an explicit outcome in the conformance policy when it differs from
  upstream Typst.

Green conformance tests can include reviewed `bug` dispositions. They do not
retroactively turn those cases into supported behavior.
