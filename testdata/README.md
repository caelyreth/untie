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
- `tex/mhchem_cases.json` contains source-only chemistry fixtures transcribed
  from static KaTeX, Typlet, and RaTeX references, plus explicit Untie
  malformed-recovery cases. It copies no reference output.

## Hydrating Upstream Inputs

Raw upstream inputs are pinned by commit, path, and SHA-256 rather than
committed. After a fresh checkout, run:

```sh
go run ./internal/quality/upstreamfetch fetch
```

Use `go run ./internal/quality/upstreamfetch --check` to verify existing files
without network access. Reviewed selections, hand-transcribed cases, derived
fonts, licenses, and manifests remain tracked. Upstream golden images are never
fetched because Untie compares canonical structure, not upstream screenshots.

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
[Compatibility and Verification](../docs/quality.md).
