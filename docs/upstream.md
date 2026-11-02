# Upstream Source Map

This document maps the pinned sources used for behavior, metadata, fixtures, and
fonts. It is a provenance record, not a claim that an upstream feature is
implemented.

Reference projects are inspected statically. Untie never compiles or runs
KaTeX, Typst, Typlet, or RaTeX as part of its build or test suite. CI consumes
only tracked files under `testdata/upstream` and checked-in generated data.

## Authority

When sources conflict, apply this order:

1. Accepted Untie decisions and the current public contract.
2. Product requirements and compatibility scope.
3. OpenType MATH, Unicode, MathML Core when relevant, and TeX rules.
4. Pinned KaTeX behavior for TeX math.
5. Pinned Typst behavior for Typst math.
6. Pre-generated upstream fixtures and tests.
7. RaTeX and Typlet as implementation and protocol evidence.
8. Other external projects as feasibility evidence only.

A lower-priority source cannot silently override a higher one. A divergence
requires a test, a documented compatibility exception, or an architecture
decision.

## Pinned Sources

| Source | Pin | Role | License | Local status |
|---|---|---|---|---|
| KaTeX 0.19.1 | `65d1e725206dbbd278704df641491cf877413b6b` | TeX syntax, commands, macro behavior, symbols, parser/error tests | MIT | Checkout HEAD confirmed |
| Typst 0.15.1 | `84cca406aafe7f830ed3f37f1cc6f0c8b6b65a03` | Typst syntax, evaluator behavior, math IR, layout, tests | Apache-2.0 | Checkout HEAD confirmed |
| Typst fixture source | `v0.15.1`, `9dfd3a08500b7896045f907433cf7b4b02434fad` | Typst revision recorded as the source of Typlet fixtures | Apache-2.0 | Not locally confirmed |
| Typlet 0.2.0 | `c3ebcac2db54ee123faaab8c437c83e7f66ec0a5` | Compact Typst pipeline, static formula and frame fixtures | MIT; generated Typst output carries Apache-2.0 notices | Checkout HEAD confirmed |
| RaTeX 0.1.15 | `f0ef2cf7c15d85c41743d4bd2df19759cf0dfee3` | TeX case inventory, display-list evidence | MIT | Checkout HEAD confirmed |
| `typst-assets` | `94dcb990edb36311ef85162baa26e4a8205e0bf0` | Exact source of tracked `NewCMMath-Book.otf` | Asset-specific; NewCM uses GUST 1.0 | Checkout absent; artifact hash confirmed |
| codex 0.3.0 | Typst revision `2e43ab9` | Transitive source of Typst symbol data captured by Typlet | Apache-2.0 | Not locally confirmed |
| unicode-math-class 0.1.0 | MathClass.txt revision 15 | Transitive Unicode math-class source captured by Typlet | MIT or Apache-2.0; recorded use is MIT | Not locally confirmed |

External candidates under evaluation, not dependencies and not imported:

| Candidate | Observed pin | Status |
|---|---|---|
| `go-opentype/opentype` | `2c8922cb87c69d7555957e90370c19f890758b36` | Unverified; no local checkout |
| `go-tex/math` | `2e01a93dc9749b2b23d3503fdb33117cace6a91b` | Unverified; no local checkout |

Adopting either candidate requires a pinned dependency review, license review,
performance and correctness tests, and an architecture decision. Neither is a
source of Untie compatibility.

## Confirmation Legend

- **Confirmed checkout**: the local ignored checkout's `git rev-parse HEAD`
  matched the recorded pin on 2026-11-02.
- **Confirmed artifact**: the tracked artifact exists and its SHA-256 or count
  is checked by repository tests or manifests.
- **Partially confirmed**: the main project pin is confirmed, but a transitive
  generation source or checkout is not available locally.
- **Unverified**: only a provenance record exists in this repository.

## KaTeX

Pinned source:

- Project: KaTeX 0.19.1
- Commit: `65d1e725206dbbd278704df641491cf877413b6b`
- License text: `testdata/upstream/licenses/KaTeX-MIT.txt`
- Duplicate metadata notice: `internal/mathdata/licenses/MIT-KaTeX.txt`

Consulted areas:

- parser, macro expander, command handlers, atoms, styles, spacing, and
  environments;
- command support inventory and parser/error cases;
- symbol definitions and zero-argument operator registrations;
- upstream screenshot inputs were not imported and are not display-list truth.

Tracked imports and generated use:

- `testdata/upstream/katex/cases.jsonl`: 36 hand-transcribed, normalized cases;
- `internal/mathdata/tex_generated.go` and
  `internal/mathdata/operator_generated.go`: generated metadata from
  `src/symbols.ts` and `src/functions/op.ts`.

Confirmed:

- local checkout HEAD matches the pin;
- generated-input SHA-256 values match `internal/mathdata/PROVENANCE.md`;
- tracked case count and file hash are enforced by fixture tests.

Limits:

- the KaTeX test selection is curated and hand-transcribed, not regenerated;
- KaTeX DOM/CSS layout and screenshots are not display-list truth;
- the full KaTeX command inventory is not implemented.

## Typst

Pinned source:

- Project: Typst 0.15.1
- Commit: `84cca406aafe7f830ed3f37f1cc6f0c8b6b65a03`
- License: `testdata/upstream/licenses/Typst-Apache-2.0.txt`
- Notice: `testdata/upstream/licenses/Typst-NOTICE.txt`
- Duplicate generated-data notice:
  `internal/mathdata/licenses/NOTICE-Typst.txt`

Consulted areas:

- math syntax and parser behavior;
- scoped evaluator behavior needed to produce math content;
- math IR, processing, spacing, style, and layout;
- OpenType MATH usage and math test cases;
- operator and syntax registrations used by generated metadata.

Tracked imports and generated use:

- generated class, operator, and symbol metadata in `internal/mathdata`;
- Typst-generated frame data imported through Typlet under
  `testdata/upstream/typlet-0.2.0/fixtures/typst-0.15.1/`;
- no Typst compiler or runtime is invoked by Untie.

Confirmed:

- local checkout HEAD matches the project pin;
- generated-input SHA-256 values for `typst-syntax/src/ast.rs` and
  `typst-library/src/math/op.rs` match provenance;
- Apache-2.0 and NOTICE files are tracked.

Partially confirmed:

- Typlet records `9dfd3a08500b7896045f907433cf7b4b02434fad` as the exact Typst
  revision used to generate its frame fixtures. That revision is not available
  as a local checkout, so the fixtures are trusted as immutable upstream
  artifacts but cannot be regenerated or independently verified here.

## Typlet

Pinned source:

- Project: Typlet 0.2.0
- Commit: `c3ebcac2db54ee123faaab8c437c83e7f66ec0a5`
- License: `testdata/upstream/licenses/Typlet-MIT.txt`
- Generated Typst output notices:
  `testdata/upstream/licenses/Typst-Apache-2.0.txt` and
  `testdata/upstream/licenses/Typst-NOTICE.txt`
- Duplicate metadata notice: `internal/mathdata/licenses/MIT-Typlet.txt`

Tracked imports:

| Path | Records | Static frames |
|---|---:|---:|
| `testdata/upstream/typlet-0.2.0/corpus/paired.jsonl` | 81 | 81 |
| `testdata/upstream/typlet-0.2.0/corpus/features.selected.jsonl` | 29 | 28 |
| `testdata/upstream/typlet-0.2.0/corpus/code.selected.jsonl` | 16 | 14 |
| `testdata/upstream/typlet-0.2.0/corpus/symbols.selected.jsonl` | 30 | 30 |
| `testdata/upstream/typlet-0.2.0/corpus/syntax.selected.jsonl` | 15 | 0 |
| `testdata/upstream/typlet-0.2.0/fixtures/typst-0.15.1/font.jsonl` | 8,606 | n/a |

The frame paths mirror the corpus paths below
`testdata/upstream/typlet-0.2.0/fixtures/typst-0.15.1/`.

`paired.jsonl` and its frame fixture are byte-for-byte copies. Selected records
retain original order and value; whitespace was normalized only by JSON Lines
selection. Selection IDs are recorded in
`testdata/upstream/manifests/typlet-selection.json`.

Confirmed:

- local checkout HEAD and tag identify Typlet 0.2.0;
- corpus counts, IDs, file hashes, and source/frame pairing are checked by
  `internal/fixture`;
- the font metric dump and NewCM frame fixtures refer to the tracked
  `NewCMMath-Book.otf`.

Limits:

- only a representative subset of feature, code, symbol, and syntax corpora is
  imported;
- the full 1,703-formula corpus is not tracked;
- fixture generation is not rerun.

## RaTeX

Pinned source:

- Project: RaTeX 0.1.15
- Commit: `f0ef2cf7c15d85c41743d4bd2df19759cf0dfee3`
- License: `testdata/upstream/licenses/RaTeX-MIT.txt`
- Imported file: `testdata/upstream/ratex-0.1.15/core-test-cases.txt`

The core case file is a byte-for-byte copy of RaTeX's
`tests/golden/test_cases.txt`. It contains 1,075 core TeX formulas. RaTeX
chemistry, proof-tree, PNG goldens, and bundled fonts are not imported.

Confirmed:

- local checkout HEAD matches the pin;
- line count and file hash are checked by fixture tests.

Limit:

- the file is a case inventory, not an expected IR or layout frame.

## New Computer Modern Math

Tracked font:

- File: `testdata/upstream/fonts/NewCMMath-Book.otf`
- SHA-256:
  `2ea09ebc9167b1e1a66f31390dc917f2d4004ecfca72d51b28010e4ad6becd95`
- Upstream: `typst-assets` revision
  `94dcb990edb36311ef85162baa26e4a8205e0bf0`,
  `files/fonts/NewCMMath-Book.otf`
- Author attribution: Antonis Tsolomitis
- License: GUST Font License 1.0, based on LPPL 1.3c or later
- Full tracked notice and font license section:
  `testdata/upstream/licenses/typst-assets-NOTICE.txt`

Confirmed:

- the tracked font SHA-256 matches `testdata/upstream/manifest.sha256`;
- `testdata/upstream/ATTRIBUTION.md` records the exact upstream path;
- the Typlet frame fixtures name `NewCMMath-Book`.

Unverified:

- the `typst-assets` checkout is not present locally, so its commit and the
  artifact's remote byte identity were not independently confirmed here.

Limit:

- the font is test data and is not yet loaded by the production font backend.

## Generated Metadata

`internal/mathdata/PROVENANCE.md` is the detailed record for checked-in generated
tables. Its direct inputs and SHA-256 values are:

| Input | SHA-256 |
|---|---|
| `KaTeX/src/symbols.ts` | `88bf41a79569d9c860292049e897cbd12019dd43f7e3e0044e39ca5deb356b84` |
| `KaTeX/src/functions/op.ts` | `a04557b2f14cccd7927b8ea60cdf3f7df649abe52ed3b81748145538deb25a9d` |
| `typst/crates/typst-syntax/src/ast.rs` | `595cea7702ab8dcc1272776f07564398527411157c8f2ebdd77b6b01c7b2b92e` |
| `typst/crates/typst-library/src/math/op.rs` | `8b4ee3ebcb085a4810b164d03f1eb0fbb4f2ce9b01224cd32ef4352d3631a1d3` |
| `Typlet/src/generated/math-class.ts` | `81697cfe65460d76fffcb834f87327c7e9f614aa7efff61effc318aab9f26bca` |
| `Typlet/src/utils/math-class.ts` | `c185f1bba6adc673ceff27ae5417b942767d2e2cd537a7a4e409870e2463c2e1` |
| `Typlet/src/utils/styling.ts` | `14fd8750bceb34b1b68d192b9e767e7e0af8db981e891b6fb15ce1548c971163` |
| `Typlet/src/generated/symbols.ts` | `229f1cb2b5892076ed14104be42adccbae055d72421c466063e150b0ee8a2d9d` |

The generator produces deterministic Go arrays with sorted records. Generated
headers identify the source files and hashes. Tests verify table semantics,
ordering, lookup behavior, and deterministic regeneration when the ignored
sources are available.

Partially confirmed:

- codex 0.3.0 at Typst revision `2e43ab9` and unicode-math-class 0.1.0
  MathClass.txt revision 15 are transitive sources recorded by Typlet and
  provenance, but their source repositories and complete license texts are not
  tracked here.

## Fixture Manifests

- `testdata/upstream/manifest.sha256` covers every imported file except itself.
- `testdata/upstream/manifests/corpora.json` declares every loadable corpus,
  file, expected count, and expected frame count.
- `testdata/upstream/ATTRIBUTION.md` records project, pin, source path, license,
  and transformation notes.

Changes to any of these files are data changes, not test fixes. A review must
confirm the upstream pin, license, transformation, expected counts, and hashes.
Never regenerate a golden from the implementation being tested.

## Update Procedure

When updating an upstream source:

1. Record the new tag and full commit.
2. Copy only the required static inputs.
3. Update attribution and license files.
4. Update the corpus selection or frame conversion documentation.
5. Regenerate manifests and generated metadata only from the pinned source.
6. Run `CGO_ENABLED=0 go test ./...`.
7. Review the fixture and metadata diff semantically.
8. Update `docs/status.md` and this map.

Do not partially update behavior, fixtures, and metadata. A mixed-version
update is not reproducible.
