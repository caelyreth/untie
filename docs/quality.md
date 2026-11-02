# Compatibility Evidence

Untie's compatibility claims are tied to tracked corpora and executable tests,
not to a claim that either input language is implemented completely.

## Core Commands

```sh
CGO_ENABLED=0 go test ./...
CGO_ENABLED=0 go vet ./...
```

For race-sensitive host integration:

```sh
go test -race ./...
```

The repository-wide corpus, architecture, and conformance gates also run from:

```sh
CGO_ENABLED=0 go test ./internal/quality
```

The separate example has its own module:

```sh
cd examples/mygo
go test ./...
```

## Daily STEM Corpus

Tracked at [`testdata/daily-stem`](../testdata/daily-stem):

- 92 cases;
- 51 TeX and 41 Typst;
- 55 display and 37 inline;
- 14 categories and 35 coverage groups;
- 2,252 source bytes.

Expected outcomes include 80 rendered formulas, 7 recoverable parse outcomes,
4 missing-glyph font-boundary cases, and 1 output-limit case. The runner treats
those expected failure classes as reviewed outcomes; "corpus pass" does not
mean every formula rendered successfully.

The corpus checks source, diagnostics, layout outcome, selected Alt text and
speech, hit semantics, tag presence, wrapping, and overflow. See
[`internal/quality/daily_test.go`](../internal/quality/daily_test.go) and the
manifest at
[`testdata/daily-stem/manifest.json`](../testdata/daily-stem/manifest.json).

## TeX Conformance

The reviewed TeX baseline covers:

- 36 normalized KaTeX cases;
- 1,075 RaTeX core corpus lines.

Baseline summary at this revision:

| Outcome | Cases |
| --- | ---: |
| Pass | 1,045 |
| Diagnostic | 26 |
| Unsupported | 40 |
| Bug | 0 |
| Unexplained | 0 |

The unsupported entries are explicitly classified in
[`testdata/conformance/tex-policy.json`](../testdata/conformance/tex-policy.json).
They are not runner crashes or silently skipped fixtures.

See [`testdata/conformance/tex-baseline.json`](../testdata/conformance/tex-baseline.json)
and [`internal/quality/conformance_test.go`](../internal/quality/conformance_test.go).

## Typst Conformance

The reviewed Typst baseline contains 171 cases from selected Typlet corpora and
fixtures.

| Outcome | Cases |
| --- | ---: |
| Pass | 156 |
| Diagnostic | 3 |
| Unsupported | 4 |
| Reviewed bug | 8 |
| Unexplained | 0 |

The known-bug count has an exact ID set and a ratchet test. New reviewed bugs
cannot silently appear, and an old ID cannot be replaced without changing the
reviewed code. See
[`internal/quality/typst_bug_budget_test.go`](../internal/quality/typst_bug_budget_test.go).

The tracked policy at
[`testdata/conformance/typst-policy.json`](../testdata/conformance/typst-policy.json)
documents exclusions for full code execution and parser/CST stress probes.

## Paired Crosscheck

The 81-case Typlet paired corpus compares Untie's TeX and Typst frontends
against paired expectations. At this revision:

- 22 exact passes;
- 55 reviewed semantic-IR differences;
- 2 reviewed fixture differences;
- 2 documented source-pair differences;
- 0 layout mismatches;
- 0 unexplained cases.

This gate prevents a frontend change from silently altering a reviewed
cross-dialect relationship. See
[`testdata/crosscheck/paired-baseline.json`](../testdata/crosscheck/paired-baseline.json).

## Font Conformance

The font tests compare against a pinned New Computer Modern Math Book fixture:

- 8,598 glyph metrics;
- Unicode mappings and variation sequences;
- MATH constants;
- italic corrections and accent attachments;
- extended-shape membership;
- MATH kern records;
- size variants and assemblies;
- `ssty` and `dtls` shaping.

The font and fixture provenance are documented in
[`font/README.md`](../font/README.md) and
[`testdata/upstream/ATTRIBUTION.md`](../testdata/upstream/ATTRIBUTION.md).

## Architecture Gate

An exact import allowlist protects the intended dependency direction:

- public data packages do not import parsers;
- layout consumes display, geometry, IR, and source;
- the root facade composes internals;
- the example module can use Untie's public API but no internal package.

See
[`internal/quality/architecture_test.go`](../internal/quality/architecture_test.go).

## Fuzzing and Hardening

The repository includes fuzz targets for public parsing, display lists, tagged
line graphs, alternative text, TeX, Typst, font GSUB, and display alignment.
Regular tests also cover cycles, typed nils, traversal budgets, invalid ranges,
output limits, cancellation, and malformed font data.

## How to Read Green Tests

Green tests prove the documented contracts, corpus classifications, and
architecture gates. They do not prove support for arbitrary LaTeX packages,
the full Typst language, or every font.

Use the supported-syntax pages together with the baseline policies:

- [Supported TeX](tex-support.md)
- [Supported Typst](typst-support.md)

The higher-level rule is simple: a feature is user-documented only when it is
present in public code and backed by a test, a tracked fixture, or an explicit
reviewed limitation.
