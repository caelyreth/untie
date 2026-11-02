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

### `Fonts` and `FontSet`

`untie.Fonts` is an input value containing the primary math face, an optional
text face, and an ordered fallback list. `untie.LoadFonts` copies the input
bytes, validates every selected face and glyph outline, and returns an
immutable `*FontSet`.

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

### `Engine`

`NewEngine(fonts, limits)` prepares reusable layout state for one font set and
one effective limit configuration. An `Engine` is safe for concurrent layouts.
It retains its font set and is garbage-collected; there is no shutdown call.

Use `Engine.Layout` or `Engine.LayoutContext` for expressions produced by the
public parser. The package-level `Layout` and `LayoutContext` functions use a
separate bounded cache of prepared engines.

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
| Font bytes passed to `LoadFonts` | Caller | `LoadFonts` copies them; later caller mutation has no effect. |
| `*FontSet` | Caller | Immutable and safe for concurrent readers. |
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

## Internal and Public Boundaries

The root package composes two internal frontends and an internal layout engine:

- `internal/tex`
- `internal/typst`
- `internal/engine`
- `internal/layout`
- `internal/alt`

Those packages are implementation details. Hosts and code assistants should
use the root facade and the public support packages:

- `github.com/caelyreth/untie`
- `github.com/caelyreth/untie/display`
- `github.com/caelyreth/untie/font`
- `github.com/caelyreth/untie/geometry`
- `github.com/caelyreth/untie/ir`
- `github.com/caelyreth/untie/source`

The repository's architecture test enforces the import direction and rejects
imports of Untie internals from the separate example module. See
[`internal/quality/architecture_test.go`](../internal/quality/architecture_test.go).

## Content Identity

Two identities are exposed for different jobs:

- `Expr.Key()` is a stable 64-bit convenience key over library version,
  dialect, mode, source, effective limits, and TeX host macros. It is suitable
  for host caches, not for cryptographic identity.
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

## Evidence

- Public type aliases and ownership: [`api.go`](../api.go)
- Engine and cache: [`layout.go`](../layout.go),
  [`layout_cache.go`](../layout_cache.go)
- Display contract: [`display/types.go`](../display/types.go)
- Canonical IR: [`ir/base.go`](../ir/base.go),
  [`ir/nodes.go`](../ir/nodes.go)
- Import boundaries:
  [`internal/quality/architecture_test.go`](../internal/quality/architecture_test.go)
