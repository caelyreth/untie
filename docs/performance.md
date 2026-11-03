# Performance

Untie's primary performance target is the complete public geometry pipeline:
parse, validate, lay out, validate the output box, and return a drawable
display list. Accessibility generation is optional and excluded unless
`Style.GenerateAlt` is enabled. Font loading and corpus loading are setup work.

## Whole-Corpus Workload

The daily performance workload contains:

- 92 formulas;
- 51 TeX and 41 Typst;
- 2,252 source bytes;
- four effective limit configurations.

Each benchmark operation completes one pass over the corpus. Reported metrics
include complete corpus passes, attempted formulas, successful layouts, output
items, source hits, and retained path segments.

The `/case` and `/success` denominators are intentionally different:

- `/case` divides by every attempted formula;
- `/success` divides by layouts that produced a box.

This keeps work spent on expected missing-glyph and limit failures visible.

## Engine Strategies

The benchmark compares several reusable-engine strategies:

- parse and validation without layout;
- caller-owned engine prepared outside the measured operation;
- caller-owned engine with accessibility text enabled;
- warm package-level engine cache;
- cold package-level engine cache;
- a new engine for every formula.

Use the caller-owned-engine result for normal application throughput. The
package-cache rows measure the convenience API, and the no-cache row is a
worst-case comparison.

## Constrained Display

Constrained display has a separate storage-focused benchmark. It distinguishes:

- canonical item, hit, and path-segment references;
- line and tag ranges into that canonical storage;
- actual retained item and deduplicated path-template storage.

Timing alone does not reveal accidental duplicate retained memory, so storage
ratios are reported alongside time.

## Interpreting Allocations

`B/op` and `allocs/op` include parsing, validation, layout, and output
construction. A speed improvement is not comparable if it moves work outside
the measured operation or reuses output across iterations.

Every corpus pass constructs fresh expressions and boxes. Font-derived data and
bounded layout scratch may be reused, but individual expressions and returned
boxes are not.

Compare repeated runs on the same hardware, toolchain, and corpus revision.
When optimizing, treat lower allocation count as useful only if corpus time is
also unchanged or lower.
