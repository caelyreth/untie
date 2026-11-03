# Supported TeX

Untie implements a KaTeX-shaped TeX math subset. It is not a LaTeX compiler
and does not load packages. This page summarizes the tested surface and the
explicit boundaries a host should show to users.

The representative commands below are evidence-backed, not an exhaustive
grammar. The symbol and command tables are broader than these examples.

## Core Math

Supported and tested:

- letters, digits, Unicode math characters, control symbols, and named symbols;
- automatic atom classes and relation/binary/operator spacing;
- superscripts, subscripts, primes, and simultaneous scripts;
- `\frac`, `\dfrac`, `\tfrac`, binomials, supported `\genfrac` forms, and
  barless/infix fraction forms represented by the neutral IR;
- `\sqrt` and indexed roots;
- named operators such as `\sin`, `\lim`, `\det`, and `\operatorname`;
- `\limits` and `\nolimits` when attached to a valid operator;
- math alphabet commands such as `\mathrm`, `\mathit`, `\mathbf`,
  `\mathbb`, `\mathcal`, `\mathfrak`, `\mathsf`, and `\mathtt`;
- math class commands such as `\mathrel`, `\mathbin`, and `\mathop`;
- KaTeX-compatible size declarations from `\tiny` through `\Huge`, with
  group-scoped text-style reset and exact size multipliers;
- explicit spacing, `\allowbreak`, and basic `\not` negation.

Examples exercised by the daily corpus include quadratic formulas, sums,
products, integrals, limits, derivatives, Maxwell equations, the
Schrodinger equation, probability expressions, sets, and piecewise cases.

## Delimiters and Fences

Supported and tested:

- literal delimiters and named forms such as `\lfloor`, `\rfloor`,
  `\langle`, `\rangle`, `\vert`, and `\Vert`;
- `\left...\right`, including `\middle`;
- TeX size commands such as `\Bigl...\Bigr`;
- paired and one-sided fences, null delimiters, and fence hits;
- matrices with `matrix`, `pmatrix`, `bmatrix`, `Bmatrix`, `vmatrix`, and
  `Vmatrix`, including starred variants;
- `cases`, `dcases`, `rcases`, and `drcases`;
- `array`, `darray`, and `subarray` with `l`, `c`, and `r` column
  specifications and supported rule syntax;
- `aligned`, `gathered`, `split`, `alignedat`, `alignat`, `align`,
  `gather`, `equation`, and their tested starred forms where implemented.

Unmatched fences, unknown environments, mismatched `\begin`/`\end`, and invalid
column specifications report diagnostics and recover.

## Decorations and Boxes

Supported and tested:

- `\overline`, `\underline`, and `\underbar`;
- braces, brackets, groups, under/over arrows, and common extensible arrows;
- `\overset`, `\stackrel`, and `\underset`;
- common accents such as `\hat`, `\tilde`, `\bar`, `\vec`, `\dot`, `\ddot`,
  and combining accents accepted by the text frontend;
- `\cancel`, `\bcancel`, `\xcancel`, and `\sout`;
- `\boxed`, `\fbox`, `\colorbox`, and `\fcolorbox`;
- `\phantom`, `\hphantom`, `\vphantom`, `\smash`, and `\mathstrut`;
- zero-width overlap forms `\llap`, `\rlap`, `\clap`, `\mathllap`,
  `\mathrlap`, and `\mathclap`;
- signed vertical `\raisebox` with a text-mode hbox body;
- `\rule` and supported explicit dimension units.

## Text, Color, Links, and Styling

Supported and tested:

- `\text`, `\textrm`, `\textnormal`, `\textsf`, `\texttt`, `\textbf`,
  `\textmd`, `\textit`, `\textup`, `\textsl`, `\emph`, `\mbox`, and `\hbox`;
- common text symbols and text accents;
- named colors and supported `#rrggbb` color syntax;
- declarative `\displaystyle`, `\textstyle`, `\scriptstyle`, and
  `\scriptscriptstyle`;
- declarative alphabet switches such as `\rm`, `\it`, `\bf`, `\sf`, `\tt`,
  and `\cal`;
- `\char` and `\verb` where their argument form is supported;
- visible link bodies for `\href` and `\url`.

Links are deliberately text-only. The destination is discarded and the parser
emits an `unsupported` warning after the arguments validate. Formula labels and
explicitly resolved equation references are supported; document-level link and
cross-file resolution are outside the API.

## Macros

The TeX frontend supports:

- host macros through `ParseOptions.Macros`;
- `\def`, `\gdef`, `\edef`, and `\xdef`;
- `\newcommand`, `\renewcommand`, and `\providecommand`;
- scoped expansion with argument placeholders and macro origin tracking.

This is a bounded formula macro expander. It is not a general package or
document execution engine.

## Labels and Equation References

The formula frontend supports:

- `\label{name}` as unique formula metadata with no visible output;
- `\eqref{name}` resolved from `ParseOptions.References`;
- `\eqref` output as text-font `(<host text>)`;
- multiple distinct labels, including labels inside groups and scripts.

The host supplies the visible numbering or reference body. For example,
`References: {"eq:a": "3"}` renders `\eqref{eq:a}` as `(3)`. Reference values
are opaque displayed text and are never re-parsed as TeX.

The first duplicate label is retained and later duplicates emit
`duplicate-label`. An absent map entry emits `unresolved-reference`; malformed
entries emit `invalid-value` or `limit-exceeded`. Each failed `\eqref` renders
a visible placeholder while preserving surrounding content.

Untie does not maintain document state. A formula's labels do not populate its
references, and references cannot resolve from another formula, a global
counter, a bibliography, or a citation database.

## Equation Tags

Top-level TeX display tags and direct row tags in supported multiline
environments are represented:

- `\tag{...}` renders parenthesized tag text;
- `\tag*{...}` renders without automatic parentheses;
- `\nonumber` and `\notag` produce suppression metadata.

Formula tags use `ir.FormulaTagLine`. Tags and suppression events directly
inside `align`, `align*`, `gather`, `gather*`, `equation`, `equation*`,
`aligned`, `gathered`, `split`, and aligned-at forms use the zero-based source
row. Multiple events on one row, descending rows, and mixing formula and row
scopes are rejected.

The following are explicitly outside the current tag contract and produce
diagnostics rather than being promoted silently:

- tags in inline math;
- tags inside a nested group;
- tags inside a nested environment or ordinary group;
- more than one manual formula-level tag;
- promotion of a tag from a script, fraction argument, root index, or similar
  syntactic argument;
- automatic document counters and hidden cross-formula reference resolution.

Constrained tagged display output intentionally does not wrap to multiple
lines. See [Rendering the display list](rendering.md) for the tag-box drawing
contract.

## Explicit Exclusions

The following capabilities are not treated as silently ignorable TeX:

| Input family                                                                                              | Current behavior                                                       |
| --------------------------------------------------------------------------------------------------------- | ---------------------------------------------------------------------- |
| Chemistry and units: `\ce`, `\pu`                                                                         | Error diagnostic; visible unsupported placeholder.                     |
| External images: `\includegraphics`                                                                       | Error diagnostic; visible unsupported placeholder.                     |
| Raw HTML/DOM hooks: `\htmlClass`, `\htmlData`, `\htmlId`, `\htmlStyle`                                    | Unsupported or discarded metadata with diagnostics; no DOM.            |
| Commutative diagrams: `CD` environment                                                                    | Unsupported diagnostic; bounded environment skip.                      |
| Manual centering/enclosure: `\vcenter`, `\phase`, `\enclose`                                               | Unsupported; no lossless neutral primitive yet.                        |
| `\nobreak`                                                                                                | Unsupported because suppression-only break policy is not represented.  |
| Inline math delimiters nested in math mode: `\(`, `\)`, `\]`                                              | Parse diagnostic.                                                      |
| Math commands in text mode, such as `\sqrt` inside `\text`                                                | `unsupported` diagnostic at the text-mode boundary.                    |
| Document packages, automatic counters, citations, file inclusion, and arbitrary conditionals              | Outside the formula frontend.                                          |

## Recovery Is Part of the Contract

Common malformed inputs are tested:

- `\frac{1}` -> `missing-argument`, recoverable;
- `\left(x` -> unclosed-group diagnostics, recoverable;
- unknown environments -> `invalid-environment`, recoverable;
- repeated/conflicting infix operators -> `unexpected-token`;
- undefined control sequences -> `unknown-command`;
- excess input or parser work -> `limit-exceeded`.

The preview remains drawable unless layout fails for a separate reason.

## What "Supported" Means

For this project, "supported TeX" means:

- the syntax lowers to canonical IR;
- canonical IR validates;
- layout is tested to produce a valid box or an explicitly expected layout
  error;
- tracked conformance and daily corpora classify the result;
- any known gap is recorded in a policy or baseline instead of being hidden.

It does not mean every command accepted by KaTeX, every LaTeX package, or every
visual edge case of TeX is implemented.
