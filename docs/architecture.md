# Architecture

This document describes the package boundaries and dependency rules for Untie.
It describes the implementation at the documentation snapshot, not only the
target design. See [status.md](status.md) for the current implementation gaps.

## Product Boundary

Untie accepts TeX or Typst math source and returns a UI-independent display
list. The intended pipeline is:

```text
TeX source   -> internal/tex   --\
                                  +-> ir.Formula -> internal/layout -> display.Box
Typst source -> internal/typst --/
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

Source spans are UTF-8 byte offsets with an exclusive end. Macro-expanded and
generated nodes retain an origin span where the frontend can identify one.

## Package Map

| Package | Responsibility | Public |
|---|---|---|
| `github.com/caelyreth/untie` | Stable facade, dialect selection, parse orchestration, API cache keys | Yes |
| `source` | Byte ranges, diagnostics, stable diagnostic codes, resource limits | Yes |
| `geometry` | Fixed-point em/DIP values and public path primitives | Yes |
| `ir` | Sealed neutral math node interface, constructors, validation, canonical test encoding | Yes |
| `font` | Host font inputs and immutable font-set ownership; OpenType parsing is planned | Yes |
| `display` | Flattened box, items, paint, paths, validation, canonical test encoding | Yes |
| `internal/tex` | Bounded TeX lexer, macro expansion, parser, and AST-to-IR lowering | No |
| `internal/typst` | Reserved Typst frontend boundary; currently a diagnostic stub | No |
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
  -> display, font, ir, source, internal/tex, internal/typst

internal/tex, internal/typst
  -> ir, source

internal/layout
  -> display, geometry, ir, source

internal/fixture
  -> geometry, source

internal/testrender
  -> display, geometry

ir
  -> source

display
  -> geometry

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
- `internal/layout` must not import `internal/tex` or `internal/typst`.
- `display` must not import `font` or an internal package.
- No core package may import a UI toolkit, DOM, GPU, platform text, image
  rasterizer, system font lookup, or native rendering library.

`architecture_test.go` enforces the package-specific edges above and rejects
known UI libraries. Rules not covered by that test remain review-enforced.

## Neutral IR

`ir.Node` is a sealed interface with one struct per node family. The sealed
interface prevents hosts from inventing nodes that layout cannot validate.
Current node families include rows, glyphs, text, operators, spaces, fractions,
radicals, scripts, fences, accents, lines, cancel, tables, multiline groups,
styles, colors, boxes, phantoms, errors, placeholders, breaks, and alignment
markers.

`ir.Base` carries source metadata and semantic state needed by layout:

- source and origin ranges;
- math class;
- requested math size and cramped state;
- logical alphabet variant;
- role or explicit paint;
- limits placement;
- explicit left and right spacing;
- whether the node is ignored for automatic spacing.

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

The public `font` package currently provides immutable byte ownership and a
stable font-set key. It does not yet parse SFNT, CFF, GSUB, GPOS, cmap, or the
OpenType MATH table. The eventual adapter is responsible for converting design
units to fixed-point ems before passing data to layout.

## Display Contract

`display.Box` contains dimensions, baseline information, overflow, and a flat
ordered list of positioned items:

- `ItemGlyphPath`: a filled outline path plus optional font and glyph identity;
- `ItemRule`: a filled axis-aligned rectangle;
- `ItemGlyphRef`: a host glyph-cache reference with font ID, glyph ID, scale,
  and position.

Paint is either a semantic role (`ink`, `error`, `placeholder`) or an explicit
RGBA source color. The host resolves roles through its theme.

`display.Box.Clone` owns copied path slices. `display.Validate` checks finite
values, nonnegative dimensions, known item kinds, valid path arity, and glyph
reference placement. `MarshalCanonical` exists for deterministic tests and
diagnostics; it is not yet a stable cross-language protocol.

## Public Orchestration

The root package is a coordinator:

- `Parse`, `ParseTeX`, and `ParseTypst` validate inputs, apply defaults, invoke
  one frontend, and return an immutable expression.
- `Expr.Diagnostics` returns copies.
- `Expr.Key` hashes source, dialect, mode, limits, Typst preamble, and sorted TeX
  macro definitions. It is a convenience identity, not a font or layout key.
- `LoadFonts` delegates font-set ownership.
- `LayoutContext` is the future public layout boundary.

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

Determinism does not promise cross-version output stability. Font bytes, face
index, font-set identity, style, layout revision, and parse identity must all be
included in future layout cache keys.

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
