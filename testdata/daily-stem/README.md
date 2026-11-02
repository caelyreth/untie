# Daily-STEM Acceptance Corpus

This corpus is the product-priority acceptance source for ordinary
undergraduate and professional STEM formula typing. It deliberately does not
try to maximize compatibility with every case in the pinned upstream projects.
When a rare reference case conflicts in priority with a supported daily
formula, the daily corpus wins; rare behavior must instead be documented as a
scope boundary.

## Exact Contents

`manifest.json` pins exact case IDs, counts, classifications, and the canonical
content digest. `cases.jsonl` contains 87 cases across 12 categories:

| Category | Cases |
|---|---:|
| algebra | 10 |
| calculus | 12 |
| delimiters | 6 |
| display-wrapping | 5 |
| linear-algebra | 10 |
| malformed-input | 4 |
| multiline-cases | 6 |
| physics-engineering | 10 |
| probability-statistics | 8 |
| resource-limits | 4 |
| sets-logic | 8 |
| text-and-fallback | 4 |

The corpus has 47 TeX and 40 Typst cases, with 35 inline and 52 display cases.
It contains 34 coverage groups and 19 explicitly source-specific cases. A
coverage group tests both dialects together; it does not assert semantic
equivalence.

## Case Schema

Each JSON Lines record has:

- a stable, sorted `id` and a `category`;
- exactly one classification: `coverageGroup` plus an `expectedDifference`, or
  a closed `sourceSpecific` reason;
- `dialect`, `mode`, and source text;
- optional parse/layout-limit and layout-style overrides;
- a closed `expect` object for parse, layout, Alt, root-fallback, and exact hit
  assertions.

`expectedDifference` is a review declaration, not a normalization rule. Its
closed values are `source-syntax`, `relation-spacing`, `text-preservation`,
`symbol-selection`, `layout-geometry`, and `alternative-text`.

`sourceSpecific` uses a closed taxonomy: `tex-delimiter-sizing`,
`typst-delimiter-sizing`, `tex-environment`, `typst-multiline`, `tex-recovery`,
`typst-recovery`, `tex-resource-limit`, `typst-resource-limit`,
`display-alignment`, `display-breaks`, `display-overflow`, and
`host-font-boundary`.

There are no catch-all skip or flaky dispositions. A case can expect clean
rendering, a recoverable set of diagnostic codes, or a named layout error such
as `missing-glyph`.

## Canonical Review

`manifest.json.contentSHA256` hashes sorted canonical case JSON, including
every source, classification, limit, style, expectation, and issue field, plus
all manifest invariants. An unreviewed edit fails before rendering.

After an intentional corpus review, update the manifest explicitly:

```sh
go run ./cmd/dailyreport -update-manifest
```

The resulting manifest and digest must be committed with the case change.

## Evaluation

The runner loads `testdata/upstream/fonts/NewCMMath-Book.otf`, then gives each
case a fresh public `Engine` with that case's limits. It validates the IR
snapshot, nonempty and optionally exact Alt text/speech, the flattened Box,
meaningful source-hit coverage, exact reviewed hit ranges/kinds, source-ordered
display lines, alignment offsets, overflow, and zero-width break behavior.
Zero-length hits are rejected for visible atom/script/group kinds; only
synthetic fence/row boundary hits may be zero-width, and they never count as
coverage for a meaningful leaf.

Every case has a two-second timeout and panic containment. A timed-out or
incompletely canceled worker poisons the runner because a process-local
goroutine cannot be force-killed; no subsequent case reuses that runner.

```sh
go run ./cmd/dailyreport
go run ./cmd/dailyreport -json
go test ./internal/daily ./cmd/dailyreport
```

## Known-Issue Policy

The current corpus contains no expected issue entries and passes with zero
known-issue failures.

Any future `issue` must be an explicit expected non-success and must also be
listed in `manifest.json.expectedIssues`. The default report and CLI treat
matched known issues as failures and exit nonzero. The top-level summary keeps
clean passes, known-issue failures, and unexpected failures separate, and lists
all non-pass IDs with details.

The only diagnostic mode is:

```sh
go run ./cmd/dailyreport -allow-known-issues
```

It permits a zero exit for matched known issues only; unexpected failures
remain fatal.

## Boundaries

- With only NewCM supplied, Chinese text-in-math fails deterministically with a
  missing-glyph error naming the unsupported character. A future host text
  face or explicit fallback-font policy may satisfy this boundary.
- Oversized input produces the reviewed limit diagnostic and a visible,
  bounded Alt fallback. It does not hang or panic.
- Node and output limits have exact fallback/error expectations.
- Typst decimal number runs now render as individual source-linked digits;
  `physics.units.typst` is a clean success with no tracked issue.
- Equation tags/numbering and Typst preamble-backed declarative code are not
  implemented, so this corpus does not claim them.
