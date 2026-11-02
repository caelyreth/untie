# Upstream Test Fixtures

This directory contains the external evidence used by Untie's tests. It is
tracked so the normal test suite does not depend on the ignored `references/`
checkout, Node, Rust, a browser, or network access.

The files are immutable inputs, not generated build products. Do not update a
golden to match a failing implementation. An upstream fixture change must be a
separate data change with updated provenance and hashes.

## Layout

```text
ATTRIBUTION.md
manifest.sha256
manifests/
  corpora.json
  typlet-selection.json
katex/
ratex-0.1.15/
typlet-0.2.0/
  corpus/
  fixtures/typst-0.15.1/
fonts/
licenses/
```

- `manifest.sha256` covers every file below this directory except itself.
- `manifests/corpora.json` declares loadable corpora and exact record counts.
- `manifests/typlet-selection.json` records the IDs retained from the larger
  Typlet corpora.
- `NewCMMath-Book.otf` is the unmodified font bundled by the pinned
  `typst-assets` revision. It is required for exact frame comparisons.

## Invariants

- Every imported record has an ID or a stable source-line identity.
- Every declared corpus count is exact.
- A source corpus and its frame fixture must have the same set of IDs.
- JSON Lines files have no blank records and no duplicate IDs.
- A test may classify a case as unsupported or a known difference, but it may
  not silently omit the case.
- The UTF-16 offsets in Typlet fixtures are converted in `internal/fixture`;
  product packages only see UTF-8 byte spans.
- Frame fixtures are in points. Product geometry is DIP. Conversion happens
  only through the fixture adapter.

Run the local checks with:

```sh
go test ./internal/fixture
```

No command in this directory regenerates fixtures from an upstream project.
