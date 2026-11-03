# Compatibility and Verification

Untie's compatibility claims are tied to tracked corpora and expected
outcomes, not to a claim that either input language is implemented completely.

## Daily STEM Corpus

The product-priority corpus contains:

- 92 cases;
- 51 TeX and 41 Typst;
- 55 display and 37 inline;
- 14 categories and 35 coverage groups;
- 2,252 source bytes.

Expected outcomes include 80 rendered formulas, 7 recoverable parse outcomes,
4 missing-glyph font-boundary cases, and 1 output-limit case. Those expected
failure classes are reviewed outcomes; a passing corpus run does not mean every
formula rendered successfully.

The corpus checks source handling, diagnostics, layout outcome, selected Alt
text and speech, hit semantics, tags, wrapping, and overflow.

## TeX Coverage

The reviewed TeX baseline covers:

- 36 normalized KaTeX cases;
- 1,075 RaTeX core corpus lines.

Current outcomes:

| Outcome     | Cases |
| ----------- | ----: |
| Pass        | 1,045 |
| Diagnostic  |    26 |
| Unsupported |    40 |
| Bug         |     0 |
| Unexplained |     0 |

Unsupported entries have explicit reviewed classifications. They are not
crashes or silently skipped inputs.

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
