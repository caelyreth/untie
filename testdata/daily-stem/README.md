# Daily-STEM Acceptance Corpus

This corpus is the product-priority acceptance source for ordinary
undergraduate and professional STEM formula typing. It deliberately does not
try to maximize compatibility with every case in the pinned upstream projects.
When a rare reference case conflicts in priority with a supported daily
formula, the daily corpus wins; rare behavior must instead be documented as a
scope boundary.

## Exact Contents

`manifest.json` pins the exact case IDs and counts. `cases.jsonl` contains 81
cases across 12 categories:

| Category | Cases |
|---|---:|
| algebra | 10 |
| calculus | 12 |
| delimiters | 6 |
| display-wrapping | 1 |
| linear-algebra | 10 |
| malformed-input | 4 |
| multiline-cases | 6 |
| physics-engineering | 10 |
| probability-statistics | 8 |
| resource-limits | 2 |
| sets-logic | 8 |
| text-and-fallback | 4 |

The corpus has 41 TeX and 40 Typst cases, with 33 inline and 48 display cases.
It contains 34 equivalent TeX/Typst groups and 13 explicitly source-specific
cases.

## Case Schema

Each JSON Lines record has:

- a stable, sorted `id` and a `category`;
- exactly one classification: `equivalentGroup` for a genuinely equivalent
  TeX/Typst pair, or `sourceSpecific` for intentionally dialect-specific
  syntax;
- `dialect`, `mode`, and source text;
- optional parse-limit and layout-style overrides;
- a closed `expect` object for parse, layout, and alternative text.

There are no catch-all skip, flaky, or known-failure dispositions. A case can
expect only clean rendering, a recoverable set of diagnostic codes, or a named
layout error such as `missing-glyph`. Product bugs use an additional stable
`issue` identifier so they remain visible in reports.

## Evaluation

The runner loads `testdata/upstream/fonts/NewCMMath-Book.otf`, parses through
the public facade, validates an independent IR root, generates and validates
`Alt`, lays out through a reusable public `Engine`, and validates the resulting
`Box`, hits, and constrained display lines. Every case has a two-second
timeout and panic containment. A command run uses one process and one prepared
Engine for all cases.

```sh
go run ./cmd/dailyreport
go run ./cmd/dailyreport -json
go test ./internal/daily ./cmd/dailyreport
```

## Known Boundaries

- With only NewCM supplied, Chinese text-in-math fails deterministically with a
  missing-glyph error naming the unsupported character. A future host text
  face or explicit fallback-font policy may satisfy this boundary.
- Oversized input produces the reviewed limit diagnostic and a visible,
  bounded fallback. It does not hang or panic.
- `physics.units.typst` records product issue `typst-decimal-text-run`: Typst
  lowers `9.81` as a multi-character text run, and layout reports
  `missing glyph: "9.81"` instead of laying out the decimal as digits. The
  equivalent TeX formula renders. This is frozen as an expected issue, not
  hidden as success.
