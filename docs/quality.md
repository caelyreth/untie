# Compatibility and Verification

Untie's compatibility claims are tied to tracked corpora and expected
outcomes, not to a claim that either input language is implemented completely.

## Daily STEM Corpus

The product-priority corpus contains:

- 124 cases;
- 67 TeX and 57 Typst;
- 81 display and 43 inline;
- 14 categories and 51 coverage groups;
- 3,365 source bytes.

Expected outcomes include 112 rendered formulas, 7 recoverable parse outcomes,
4 missing-glyph font-boundary cases, and 1 output-limit case. Those expected
failure classes are reviewed outcomes; a passing corpus run does not mean every
formula rendered successfully.

The corpus checks source handling, diagnostics, layout outcome, selected Alt
text and speech, hit semantics, tags, wrapping, and overflow.

## TeX Coverage

The reviewed TeX baseline covers:

- 36 normalized KaTeX cases;
- 1,075 RaTeX core corpus lines;
- 103 RaTeX chemistry cases;
- 19 RaTeX centered-negation cases;
- 23 RaTeX slashed-symbol cases;
- 38 RaTeX proof-tree cases.

Current outcomes:

| Outcome     | Cases |
| ----------- | ----: |
| Pass        | 1,185 |
| Diagnostic  |    26 |
| Unsupported |    83 |
| Bug         |     0 |
| Unexplained |     0 |

Unsupported entries have explicit reviewed classifications. They are not
crashes or silently skipped inputs.

The bounded chemistry frontend parses 100 of the 103 RaTeX chemistry cases,
including embedded TeX math, signed scripts, adjacent arrow annotations, and
nested chemistry. The three reviewed exclusions are one whole-formula
dollar-delimited case and two specialist aromatic/multi-level bond cases.
Centered negation passes all 19 cases. Slashed-symbol and proof-tree suites are
tracked as explicit scope exclusions.

## Typst Coverage

The reviewed Typst baseline contains 171 cases from selected Typlet corpora and
fixtures:

| Outcome              | Cases |
| -------------------- | ----: |
| Pass                 |   156 |
| Diagnostic           |     3 |
| Unsupported          |     4 |
| Reviewed known issue |     8 |
| Unexplained          |     0 |

The eight reviewed known issues have a fixed ID set. New issues cannot appear
without an explicit compatibility update.

## Cross-Dialect Consistency

The 81-case paired corpus compares TeX and Typst formulas that should lower to
the same semantic representation or have an explicitly reviewed difference:

- 22 exact matches;
- 55 reviewed semantic differences;
- 2 reviewed fixture differences;
- 2 documented source-pair differences;
- 0 layout mismatches;
- 0 unexplained cases.

## Font Compatibility

The font substrate is checked against a pinned New Computer Modern Math Book
fixture covering:

- 8,598 glyph metrics;
- Unicode mappings and variation sequences;
- MATH constants;
- italic corrections and accent attachments;
- extended-shape membership;
- MATH kern records;
- size variants and assemblies;
- `ssty` and `dtls` shaping.

See [Fonts and fallback](fonts.md) and the
[`font` package guide](../font/README.md) for the public contract.

## What Verified Means

Verified behavior means that:

- input syntax lowers to validated canonical IR;
- the normal path produces a valid box or an explicitly expected failure;
- tracked corpora classify the result;
- known gaps are recorded rather than hidden.

It does not imply support for arbitrary LaTeX packages, the complete Typst
language, or every possible font. Use [Supported TeX](tex-support.md) and
[Supported Typst](typst-support.md) for the current user-facing boundaries.
