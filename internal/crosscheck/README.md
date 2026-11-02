# Cross-Dialect Conformance

This package verifies the 81 pinned `typlet-paired` formulas end to end:

1. Load the sources and static Typst frames through `internal/fixture`.
2. Parse Typst and TeX through the public `untie` facade.
3. Validate both IR trees, including UTF-8 byte source ranges.
4. Normalize only source locations and the documented dialect encodings below.
5. Lay out both trees with the exact pinned NewCM font through `internal/engine`.
6. Compare semantic IR, cross-dialect glyph geometry, and both outputs against
   the static Typst frame where a paired visual claim exists.

No test or command requires `references/`. The fixture root is the tracked
`testdata/upstream` tree.

## Commands

```sh
go test ./internal/crosscheck
go test -bench=. ./internal/crosscheck
go run ./cmd/crosscheck
go run ./cmd/crosscheck -json
```

The baseline is
`internal/crosscheck/testdata/paired-baseline.json`. It contains all 81 case
IDs and dispositions, not just failures. A behavior change fails the test until
the observed result is reviewed and the data file is intentionally updated.

## Normalization Contract

The semantic comparison removes source evidence only:

- `Base.Span` and `Base.Origin`.
- Inherited `MathSize` and `Cramped` state are projected to their effective
  values. This avoids treating TeX's explicit root text size and Typst's
  `SizeInherit` as different.
- A row that has default atom metadata is treated as a transparent grouping
  container. Its children are spliced into the parent sequence. Non-neutral rows
  and empty rows remain nodes.
- A nil fence endpoint and an empty zero-ink row endpoint are the same null
  delimiter. Empty rows inside ordinary content remain significant.
- A single Unicode symbol may be encoded as `Glyph` or `Operator`; both become
  the same `atom` kind. Multi-rune `Operator` text remains `operator-text`.
  For non-letter, non-digit symbols, `VariantAuto` and `VariantNormal` are
  equivalent because symbol shaping does not apply an italic alphabet
  transform. Letter and digit variants remain strict.
- `Top` and `TopRight`, and `Bottom` and `BottomRight`, are canonicalized only
  when the shared limit-placement rules select side scripts. When limits are
  selected, they remain distinct. This records the neutral field choice without
  erasing a limits decision.
- A scripts node's own class is derived from its base node, so the base class is
  compared once rather than a second time on the wrapper.
- Semantic roles on fractions, scripts, cancellation overlays, and tables are
  compared explicitly. Two otherwise-identical trees with different quotient,
  limit, negation, or table-role semantics are not equivalent.
- Display-only script placement is distinct from class-dependent `auto`.
- Accent source-stretch intent and natural width fitting are compared
  separately, so Typst and TeX policy differences are visible before layout.
- A standalone explicitly sized delimiter remains a leaf with strict text,
  class, and neutral sizing policy. A paired fence's content/relative/absolute
  sizing policy is also compared strictly. Neither form is normalized into an
  ordinary atom or a different sizing kind.
- Eight reviewed pairs preserve genuinely different source semantics, such as
  different Greek code points, short versus long arrows, differential spacing,
  preserved text spaces, and color scope. Their baseline reasons identify the
  source fact instead of implying a renderer normalization bug.

Everything else remains strict, including atom class, math variant, paint, math
limits, explicit spacing, fraction padding/style/bar state, fence balance,
table lines, text font/tracking, decoration fields, and error/placeholder data.

## Layout Contract

Renderer geometry is already in DIP. Static Typlet frames are converted exactly
once from points. Comparison tolerance is Typlet's reviewed `0.0015pt`, or
`0.002` DIP after conversion.

The cross-dialect comparison checks box width/ascent/descent, box details,
ordered item kinds, paint, bounds, glyph paths, rule rectangles, glyph font
identity, glyph ID, x/y position, and em size. Static top-anchored frames are
compared after converting renderer baselines with the renderer box ascent.
Fixture comparison currently checks glyph identity/placement and box geometry;
direct table tests additionally compare stroked fixture lines with rule
geometry. Raster pixels remain outside this checker.

## Reporting

Every case has exactly one kind:

- `pass`: all required comparisons pass.
- `semantic-ir-mismatch`: parse diagnostics or normalized semantic trees differ.
- `layout-mismatch`: semantic trees agree but one side cannot lay out or the
  cross-dialect geometry differs.
- `fixture-mismatch`: both renderer outputs agree but at least one differs from
  the static Typlet Typst frame.
- `documented-pair-difference`: the upstream pair intentionally has no TeX
  source.
- `unexplained`: the observed kind differs from the reviewed baseline or no
  reviewed reason exists. A validated report must contain zero of these.

The committed baseline is deliberately strict about `detail`. This makes a
changed failure path, not merely a changed pass/fail bit, visible in review.

## Current Baseline

Current reviewed baseline:

```text
pass                        22
semantic-ir-mismatch        55
layout-mismatch              0
fixture-mismatch             2
documented-pair-difference   2
unexplained                  0
```

This is a conformance inventory, not a claim that the paired corpus currently
passes. The reasons are recorded per case in the baseline. The remaining
semantic cases mostly stop at a frontend-diagnostic or compound IR difference
before cross-dialect layout comparison. The largest remaining gaps are
unrestricted Typst code, font-sizing commands, chemistry/commutative-diagram
pairs, compound atom structure, math alphabets, and the two remaining
static-frame differences. `big-sizes` remains a reviewed semantic difference
because TeX lowers discrete absolute tiers while the Typst source requests a
content-relative percentage.

Baseline details can be refreshed with an explicit review file:

```sh
go run ./cmd/crosscheck -refresh \
  -write internal/crosscheck/testdata/paired-baseline.json \
  -reasons reviewed-reasons.json
```

The refresh command reuses reasons for unchanged result kinds but refuses any
kind transition without a new reviewed reason.

Supplementary-plane spans and long repeated runs are tested separately from the
upstream 81-case count so the pinned corpus remains exact.
