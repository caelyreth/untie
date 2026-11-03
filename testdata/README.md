# Reference Test Data

This directory contains reviewed inputs, expected outcomes, and immutable
reference material used to verify Untie's parsers, layout, fonts, and
cross-dialect behavior. It is not a runtime dependency.

## Structure

- `daily-stem/` is the product-priority formula corpus, with case IDs, counts,
  review classifications, and a canonical content digest in its manifest.
- `conformance/` contains reviewed TeX and Typst conformance policies and
  baselines.
- `crosscheck/` contains the reviewed paired TeX/Typst crosscheck baseline.
- `tex/` contains additional reviewed TeX parser cases.
- `upstream/` contains immutable reference corpora, frames, fonts, and
  attribution material.

## Review Rules

- Do not update an expected result merely to make a failing implementation
  pass.
- Keep declared counts, IDs, hashes, and source/frame pairs exact.
- Do not silently skip a case. Record a supported, diagnostic, unsupported, or
  reviewed-difference disposition.
- Do not regenerate an expected result from the implementation under test.
- Convert fixture points and string offsets only at the fixture boundary.
- Keep reviewed known issues explicit; do not replace one with another without
  a compatibility review.

Compatibility summaries and current reviewed outcomes are documented in
[Compatibility and Verification](../web/src/content/docs/quality.md).
