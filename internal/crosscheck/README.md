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

Everything else remains strict, including atom class, math variant, paint, math
limits, explicit spacing, fraction padding/style/bar state, fence balance,
table lines, text font/tracking, decoration fields, and error/placeholder data.

## Layout Contract

Renderer geometry is already in DIP. Static Typlet frames are converted exactly
once from points. Comparison tolerance is Typlet's reviewed `0.0015pt`, or
`0.002` DIP after conversion.

The comparison checks box width/ascent/descent, TeX-versus-Typst box details,
glyph font identity, glyph ID, x/y position, and em size. Static top-anchored
frames are compared after converting renderer baselines with the renderer box
ascent. Paint order and glyph item count are significant. Paths and raster
pixels are intentionally outside this checker; glyph identity pins the expected
outline.

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

At the `codex/crosscheck` base commit:

```text
pass                         0
semantic-ir-mismatch        74
layout-mismatch              0
fixture-mismatch             5
documented-pair-difference   2
unexplained                  0
```

This is a conformance inventory, not a claim that the paired corpus currently
passes. The reasons are recorded per case in the baseline. All non-fixture cases
currently stop at a semantic or frontend-diagnostic difference before reaching
the cross-dialect layout comparison. The largest gaps are unresolved Typst
symbol/function lowering, fence and compound-atom metadata, math-alphabet atom
classes, style metadata, and static-frame metric drift.

Supplementary-plane spans and long repeated runs are tested separately from the
upstream 81-case count so the pinned corpus remains exact.
