# Architecture

This document describes the package boundaries and dependency rules for Untie.
It describes the implementation at the documentation snapshot, not only the
target design. See [status.md](status.md) for the current implementation gaps.

## Product Boundary

Untie accepts TeX or Typst math source and returns a UI-independent display
list. The intended pipeline is:

```text
TeX source   -> internal/tex   --\
                                 +-> ir.Formula --+-> internal/layout -> display.Box
Typst source -> internal/typst --/                \-> internal/alt -> display.Alt
```

The layers have different responsibilities:

1. A dialect frontend owns lexing, macro or value evaluation, syntax recovery,
   and one lowering pass into the neutral IR.
2. `ir` owns the language-neutral formula tree and its invariants. It must not
   contain measured font geometry or know which dialect produced a node.
3. `internal/layout` owns style, spacing, font-metric use, construct layout, and
   flattening. It must not import a dialect frontend or branch on dialect.
4. `display` owns the immutable public output contract. It contains positioned
   glyph paths and rules in host units, not source semantics.
5. `internal/alt` derives deterministic alternative text from the neutral IR.
   It does not inspect source text, frontend ASTs, fonts, layout fragments, or
   rendered display paths.

Source spans are UTF-8 byte offsets with an exclusive end. Macro-expanded and
generated nodes retain an origin span where the frontend can identify one.

## Package Map

| Package | Responsibility | Public |
|---|---|---|
| `github.com/caelyreth/untie` | Stable facade, dialect selection, parse orchestration, reusable layout engines, bounded engine cache | Yes |
| `source` | Byte ranges, diagnostics, stable diagnostic codes, resource limits | Yes |
| `geometry` | Fixed-point em/DIP values and public path primitives | Yes |
| `ir` | Sealed neutral math node interface, constructors, validation, canonical test encoding | Yes |
| `font` | Host font inputs, immutable font-set ownership, OpenType MATH/GSUB/cmap/outline parsing | Yes |
| `display` | Flattened box, items, paint, paths, source hits, break metadata, alternative text, validation, canonical test encoding | Yes |
| `internal/alt` | Deterministic neutral-IR-to-text/speech projection with bounded fallback behavior | No |
| `internal/tex` | Bounded TeX lexer, macro expansion, parser, and AST-to-IR lowering | No |
| `internal/typst` | Bounded Typst lexer/parser/evaluator and AST-to-IR lowering | No |
| `internal/layout` | Fixed-point math layout and flattening behind narrow font ports | No |
| `internal/mathdata` | Generated symbol, class, variant, shorthand, and operator tables | No |
| `internal/fixture` | Decoders, corpus runners, unit and span adapters, golden helpers | No |
| `internal/testrender` | Test-only SVG inspection of display lists | No |
| `cmd/formula-check` | Developer CLI for diagnostics and canonical IR inspection | Command |

`internal/fixture` and `internal/testrender` are test infrastructure. They must
not become runtime dependencies of the renderer.

## Dependency Direction

The current import graph follows these rules:

```text
untie
  -> display, font, internal/alt, internal/engine, internal/tex, internal/typst, ir, source

internal/tex, internal/typst
  -> ir, source

internal/alt
  -> display, ir, source

internal/layout
  -> display, geometry, ir, source

internal/engine
  -> display, font, geometry, internal/layout, internal/mathdata, ir, source

internal/fixture
  -> geometry, source

internal/testrender
  -> display, geometry

ir
  -> source

display
  -> geometry, source

font, internal/mathdata
  -> standard library only

source, geometry
  -> standard library only
```

The forbidden edges are more important than the diagram:

- `source` and `geometry` must not import another Untie package.
- `ir` must not import a frontend, `font`, `layout`, or `display`.
- `font` must not import `ir`, `layout`, `display`, or a frontend.
- A frontend must not import `font`, `layout`, or `display`.
- `internal/alt` must not import a frontend, `font`, `layout`, `engine`, or
  rendered display-list construction code.
- `internal/layout` must not import `internal/tex` or `internal/typst`.
- `display` may import `geometry` and the low-level immutable `source` range
  types. It must not import `font`, a frontend, or an internal package.
- No core package may import a UI toolkit, DOM, GPU, platform text, image
  rasterizer, system font lookup, or native rendering library.

`architecture_test.go` enforces the package-specific edges above and rejects
known UI libraries. Rules not covered by that test remain review-enforced.

## Neutral IR

`ir.Node` is a sealed interface with one struct per node family. The sealed
interface prevents hosts from inventing nodes that layout cannot validate.
Current node families include rows, glyphs, text, operators, spaces, fractions,
radicals, scripts, paired fences, standalone explicitly sized delimiters,
accents, lines, cancel, tables, multiline groups, styles, colors, boxes,
phantoms, errors, placeholders, breaks, and alignment markers.

`ir.Base` carries source metadata and semantic state needed by layout:

- a contiguous source range and call-site origin;
- math class;
- requested math size and cramped state;
- logical alphabet variant;
- role or explicit paint;
- limits placement;
- display-only limits placement distinct from class-dependent automatic limits;
- explicit left and right spacing;
- whether the node is ignored for automatic spacing.

`Span` is always one contiguous source range, including for macro-expanded
content. `Origin` records the contiguous call site used for source hits.
Expansion-internal definition-site ranges are not exposed in neutral IR because
the current contract has no multi-span metadata and rendering must not attach a
hit to bytes outside the caller's formula text.

Neutral nodes also preserve source semantics that layout must not infer:
fraction/table/script/cancel roles, source accent stretch intent, and natural
accent width fitting. These fields are compared by crosscheck; layout may
consume them only through neutral rules, never dialect checks.

`DelimiterSizing` is the shared target policy for paired `Fenced` nodes and
standalone `SizedDelimiter` leaves:

- `Content` fits delimiters to the wrapped content and is the default for
  `Fenced`;
- `Relative` multiplies the content-derived target by a percentage length;
- `Absolute` supplies an explicit target length, with font-relative units
  resolved against the active or explicitly selected typographic style.

A standalone `SizedDelimiter` is not an ordinary delimiter atom. It retains its
delimiter text, opening/closing/relation/ordinary class, source evidence, and an
absolute or relative policy, but not a dialect tier name. Layout renders it in
the inherited math style. TeX nodes explicitly request `SizeText`, so their
em-valued targets remain fixed inside scripts. TeX's four `\big...` tiers stay
in the TeX adapter and lower to absolute 1.2em, 1.8em, 2.4em, and 3.0em
targets. Typst `lr(size:)` lowers default/100% to `Content`, other percentages
to `Relative`, and length values to `Absolute`; single-delimiter calls use a
standalone `SizedDelimiter` that inherits the surrounding script scale.
Non-length arguments are invalid-value errors rather than
unsupported/defaulted geometry.

`ir` deliberately does not contain glyph IDs, font advances, outline paths,
pixel sizes, or dialect AST types. Those values either belong to source
semantics or to measured layout, not both.

`ir.Validate` checks tree structure, source ranges, enum domains, required
children, lengths, and configured node/nesting limits. Frontends are expected to
produce IR that passes validation; a validation failure is an implementation
bug, not malformed user input.

## Source And Limits

`source.Limits` bounds source bytes, tokens, nesting, IR nodes, macro expansion,
expanded tokens, call depth, table cells, assembly repeats, text bytes, output
items, path segments, and physical dimensions. Zero and negative values receive
conservative defaults through `WithDefaults`.

Limits are part of the parse cache key so two calls with different resource
contracts cannot accidentally share cached state.

Diagnostics use stable codes and sorted, normalized ranges. Malformed formula
source normally produces a diagnostic plus recoverable IR rather than an
ordinary Go error. Public parser panic recovery exists as a final containment
boundary; it is not a substitute for local recovery.

## Geometry And Units

There are three distinct unit domains:

1. OpenType design units, converted from a font's units-per-em.
2. Internal fixed-point ems (`geometry.Em`, `1 << 16` subunits per em) used by
   layout arithmetic.
3. Host DIPs exposed as `float32` in `display`.

Layout must use fixed-point arithmetic and perform host conversion only during
flattening. A frontend must not compute scaled pixel sizes. Fixture adapters
convert Typlet points or other source units at test boundaries; product code
must not accept mixed units implicitly.

The public display path uses a baseline-relative origin. Flattening quantizes
to the documented display precision. Rasterization is not a core dependency.

## Font Boundary

`internal/layout` depends on narrow ports rather than the `font` package:

- `GlyphSource` returns shaped glyph metrics and outlines in local ems.
- `StyleFont` supplies MATH constants for a requested style and variant.
- `GlyphStretcher` optionally returns MATH size variants and assemblies.
- `TextSource` optionally shapes text runs deterministically.
- `TableLayouter` optionally measures matrix and table cells.

These ports keep font-file parsing, glyph selection, shaping, and layout
separable. The layout engine requires glyph and metric ports. Optional ports
must reject unsupported requests explicitly; they must not invent metrics.

The public `font` package parses SFNT/TTC faces, cmap and variation mappings,
MATH constants and constructions, math GSUB features, and CFF/TrueType outlines.
`internal/engine` adapts those design-unit results to the narrow layout ports,
including deterministic glyph selection, stretching, text shaping, and fallback
order. For moustache delimiters without a native vertical MATH construction,
the adapter assembles the KaTeX-defined U+23A7/U+23AA/U+23AD and
U+23AB/U+23AA/U+23A9 piece sequences within the selected math face. The same
adapter path serves standalone tiers and tall `\left...\right` moustaches.

## Display Contract

`display.Box` contains dimensions, baseline information, overflow, and a flat
ordered list of positioned items:

- `ItemGlyphPath`: a filled outline path plus optional font and glyph identity;
- `ItemRule`: a filled axis-aligned rectangle;
- `ItemGlyphRef`: a host glyph-cache reference with font ID, glyph ID, scale,
  and position.

It also owns the public source-hit, inline-break, alternative-text, and display
line types. A hit maps a UTF-8 byte range to one placed rectangle and carries
depth/kind for deterministic lookup. The layout engine emits hits from the
placed fragment tree rather than reconstructing geometry from a flattened item
list. Inline layout emits top-level relation, binary, and explicit break
metadata. Display layout with a positive `Style.MaxWidth` greedily partitions
the top-level placed fragment groups at those neutral boundaries and emits
`LineBox` values in a `MaxWidth`-wide alignment frame; it does not slice
flattened items or rescan source. Each line carries a nonnegative `OffsetX`,
baseline offset, and line-local box. `Style.Align` chooses start, center, or end
position within the frame. Public orchestration derives alternative text from
the neutral IR after geometry succeeds, so an accessibility failure never
changes formula layout.
Alternative output honors the per-field stricter intersection of Engine and
expression resource limits; `Box.Alt` may therefore be a bounded fallback that
differs from `Expr.Alt` when the Engine contract is tighter.
The greedy pass chooses the furthest cut that fits. Penalty is consulted only
when distinct cuts project to the same width; this is not TeX badness
optimization. If exact source topology cannot be proven, display wrapping
returns one complete overfull line instead of guessing from paint order.
Wrapped output charges the parent and line draw items plus `LineBox` records to
`MaxOutputItems`; parent and line path copies remain bounded by
`MaxPathSegments`; aggregated parent hits remain bounded by `MaxNodes`. Parent
`Items` and `Hits` are the horizontally and vertically translated aggregate in
frame coordinates. Hosts draw either parent items or per-line items at their
line offset and baseline, never both. Parent `Width` is the requested
`MaxWidth`, including for single-line and overfull results. An overfull
unbreakable line has zero `OffsetX` and honest positive overflow. Parent
`Overflow` is recomputed from the assembled aggregate bounds relative to the
frame; line-local overflow is not treated as the parent value.

Paint is either a semantic role (`ink`, `error`, `placeholder`) or an explicit
RGBA source color. The host resolves roles through its theme.

`display.Box.Clone` owns copied path slices. `display.Box.Italic` preserves the
signed OpenType MATH italic correction; it is not a nonnegative dimension.
`display.Validate` checks finite values, nonnegative dimensions other than the
italic correction, known item kinds and paint roles, valid path arity, and glyph
reference placement. Explicit colors must use the ink role. For wrapped boxes,
parent items and hits must be exact aggregates of their line boxes. `Validate`
also checks hit ranges/areas and break positions. `MarshalCanonical` exists for
deterministic tests and diagnostics; it is not yet a stable cross-language
protocol.

## Public Orchestration

The root package is a coordinator:

- `Parse`, `ParseTeX`, and `ParseTypst` validate inputs, apply defaults, invoke
  one frontend, and return an immutable expression.
- `Expr.Diagnostics` returns copies.
- `Expr.Key` hashes source, dialect, mode, limits, Typst preamble, and sorted TeX
  macro definitions. It is a convenience identity, not a font or layout key.
- `LoadFonts` delegates font-set ownership.
- `NewEngine` prepares a caller-owned, concurrency-safe layout environment.
- `Engine.Layout` and `Engine.LayoutContext` are the explicit lifecycle API.
- Package-level `Layout` and `LayoutContext` use a bounded eight-entry cache of
  prepared engines, keyed by full font identity, effective limits, layout
  revision, and cache schema. Layout results are not cached.
- `ClearLayoutEngineCache` releases engines retained by the convenience path.
- `Style.Align` positions constrained display lines within the requested
  `MaxWidth` frame. It has no effect when `MaxWidth` is zero or for inline
  layout.

The root package must not grow construct-specific algorithms or duplicate
frontend and layout logic.

## Concurrency And Determinism

Expressions and font sets are immutable after construction. Their accessors must
not expose mutable backing storage. A layout call is read-only with respect to
the expression and font set.

The intended concurrency contract is:

- parse calls own all per-call state;
- one immutable font set may be used by concurrent layout calls if its ports are
  concurrency-safe;
- map iteration never affects keys, diagnostics, generated tables, or output;
- cancellation is checked throughout macro expansion, parsing, layout, and
  flattening;
- identical inputs and versions produce identical canonical output.

Determinism does not promise cross-version output stability. The engine-cache
identity includes full font bytes, face IDs and indices, effective limits,
layout revision, and cache schema. Style and parse identity remain per-call
inputs because layout results are not cached.

## Architectural Regressions

Reject a change if it:

- adds a dialect switch to `internal/layout` or `display`;
- stores measured font geometry or glyph IDs in `ir`;
- makes a frontend import layout to measure itself;
- exposes a frontend AST as the public IR;
- adds a UI, DOM, GPU, native font, or image dependency to a core package;
- silently substitutes a fallback for unsupported syntax or missing metrics;
- mixes points, DIPs, ems, and design units without a named conversion boundary;
- puts generation-only or fixture-only code on a product import path;
- weakens a parser limit, cancellation check, or deterministic ordering to make
  a single test pass.

These are refactor triggers. A local workaround is not acceptable when a change
crosses one of these boundaries.
