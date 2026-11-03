# Core Concepts and Ownership

Untie has three deliberate boundaries: source, canonical IR, and display
output. Keeping those boundaries separate is what lets one renderer accept
both TeX and Typst while leaving drawing policy to the host.

## Pipeline

```text
TeX or Typst bytes
        |
        v
  dialect frontend
  parse + recover
        |
        v
canonical IR (ir.Formula)
  neutral nodes + metadata
        |
        v
  Renderer.Render
  parse facade + cache
        |
        v
  Engine.Layout
  font metrics + shapes + layout policy
        |
        v
display.Box
  flattened items, hits, lines, tags, alt
```

Source text is never a runtime dependency of layout. Parser-produced nodes
carry UTF-8 byte ranges back into the source for diagnostics and hit testing,
but the layout algorithm consumes only the neutral tree.

## The Main Values

### `FontFace`, `FontSetConfig`, and `FontSet`

`untie.FontSetConfig` contains a primary math `untie.FontFace`, an optional text
face, and an ordered fallback list. Empty face IDs are derived from role,
metadata, selected collection index, and bytes. `untie.LoadFontSet` copies the
input bytes, validates every selected face and glyph outline, and returns an
immutable `*FontSet`. `untie.LoadMathFont` is the one-face convenience path.

The `FontSet` is safe for concurrent readers and is retained by an `Engine`.
It has no `Close` method.

See [Fonts and fallback](fonts.md).

### `Expr`

`Parse`, `ParseTeX`, and `ParseTypst` return an immutable `*Expr`. It owns:

- the canonical IR root and any formula-level tag metadata;
- the normalized parse diagnostics;
- the effective parse limits;
- the original source, dialect, and root mode;
- a convenience cache key.

Use `RootSnapshot()` when you need an independent IR snapshot. It returns the
clone error explicitly; there is no silent snapshot accessor.

`Expr.Diagnostics()` returns an independent copy. Changing that slice cannot
change the expression. `Expr.HasErrors()` provides the common
error-severity check without allocating a diagnostic copy.

### `Renderer`

`NewRenderer(fonts, options)` is the main application entry point. It prepares
one engine, applies dialect and style defaults, parses each source string, and
optionally caches immutable parsed expressions. Its `Render` method returns a
result containing the box, parse diagnostics, and any requested alternative
text.

`Renderer` is safe for concurrent renders. It never caches returned boxes, so
each successful call produces independent layout output.

### `Engine`

`NewEngine(fonts, limits)` prepares reusable layout state for one font set and
one effective limit configuration. An `Engine` is safe for concurrent layouts.
It retains its font set and is garbage-collected; there is no shutdown call.

`Engine` is the advanced entry point for hosts that need explicit layout
control, canonical-IR layout, or a custom parse step. The package-level `Layout`
and `LayoutContext` functions use a separate bounded cache of prepared engines.

### `Box`

A `Box` is the complete UI-independent result of layout:

- nominal width, ascent, descent, italic correction, axis height, and overflow;
- a flattened display list in host units;
- source-linked hit rectangles;
- inline break opportunities;
- optional display-line boxes and equation tags;
- alternative text when `Style.GenerateAlt` is requested.

Every successful layout returns an independent `Box`. Reusing an engine does
not reuse box storage. `Box.Clone()` produces a deep, independent copy of the
public box contract.

## Ownership Rules

| Value | Owner after the call | Important rule |
| --- | --- | --- |
| Font bytes passed to `LoadMathFont` or `LoadFontSet` | Caller | The loader copies them; later caller mutation has no effect. |
| `*FontSet` | Caller | Immutable and safe for concurrent readers. |
| `*Renderer` | Caller | Reusable and concurrency-safe; returned boxes remain caller-owned. |
| `*Engine` | Caller | Reusable and concurrency-safe; GC-managed. |
| `ParseOptions.Macros` | Caller | Copied into the expression's parse calculation. |
| `Style.Features` | Caller | Snapshot per layout call. |
| `*Expr` | Caller | Immutable; create independent IR snapshots for mutation. |
| `*Box` | Caller | Independent per layout; do not mutate while sharing. |
| `*Formula` passed to `LayoutFormulaContext` | Caller | Must stay immutable for the duration of the call. |

The public fields on IR nodes and display items are intentionally inspectable.
That does not make shared values safe for concurrent mutation. Treat parsed
expressions, formulas, and boxes as immutable after they are shared.

## Root Modes

`untie.Inline` and `untie.Display` are root math styles:

- `Inline` uses text style and is intended for placement in a host text line.
- `Display` uses display style and supports constrained line boxes when
  `Style.MaxWidth` is positive.

The mode changes math style, operator limits behavior, glyph scale, and
available layout metadata. It does not make Untie a block or paragraph layout
engine.

`Style.ScriptLevel` adapts the starting size:

- `0`: text style for inline, display style for display;
- `1`: script style;
- `2` or greater: script-script style.

Negative script levels are invalid.

## API Layers

Most hosts only need these packages:

- `github.com/caelyreth/untie`
- `github.com/caelyreth/untie/display`
- `github.com/caelyreth/untie/font`
- `github.com/caelyreth/untie/geometry`
- `github.com/caelyreth/untie/ir`
- `github.com/caelyreth/untie/render`
- `github.com/caelyreth/untie/encode/svg`
- `github.com/caelyreth/untie/encode/raster`
- `github.com/caelyreth/untie/source`

The [render](rendering.md) package provides the framework-neutral drawing
protocol. [SVG and raster](rendering.md#svg-output) packages are optional output
adapters. MyGo applications can use the [MyGo adapter](mygo.md) for direct
`ui.Element` rendering and Markdown math.

## Content Identity

Two identities are exposed for different jobs:

- `Expr.Key()` is a stable 64-bit convenience key over library version,
  dialect, mode, source, parse-affecting limits, and TeX host macros. It is
  suitable for host caches, not for cryptographic identity.
- `FontSet.KeySHA256()` is the collision-resistant identity of the complete
  font set, including face IDs and selected file indexes. `FontSet.Key()` is
  the first 64 bits for simpler host maps.

The package-level engine cache uses the full SHA-256 font identity, effective
limits, and `Version()`. It never caches a rendered `Box`.

## What Untie Does Not Own

- Font discovery or downloading
- Theme colors. The host resolves `RoleInk`, `RoleError`, and
  `RolePlaceholder`, or honors an explicit `RGBA`.
- Paragraph layout for inline formulas
- Document counters, references, or bibliography resolution
- Screen-reader behavior beyond deterministic alternative text
- GPU tessellation or glyph-atlas ownership
- Validation of the upstream license terms of host-supplied fonts
