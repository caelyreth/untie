# Untie Documentation

Untie parses TeX or Typst math into one language-neutral IR and lays that IR
out with a host-supplied OpenType MATH font. The result is a UI-independent
display list of filled glyph paths, filled rules, source hits, break metadata,
and accessibility text.

These docs describe the public API at library version `0.4.0` and layout
revision `3`. They focus on the behavior a host or code assistant can rely on
when integrating Untie; they are not a TeX or Typst language tutorial.

## Start Here

- [Getting started](getting-started.md) shows the complete
  `LoadMathFont` -> `NewRenderer` -> `Render` -> draw flow, with the lower-level
  engine path documented separately.
- [Core concepts and ownership](concepts.md) explains the pipeline, the
  lifetime of each value, and the inline/display boundary.
- [Rendering the display list](rendering.md) defines coordinates, items,
  lines, tags, hits, overflow, bounds, drawing helpers, and optional SVG/PNG
  output.
- [Editor integration](editor-integration.md) defines recovery, source
  mapping, host shaping, color policy, and MyGo font ownership for live
  editors.

## Integration Guides

- [MyGo integration](mygo.md) provides the bounded adapter for MyGo Markdown
  math, direct UI elements, theme-role drawing, and existing-box drawing.
- [Fonts and fallback](fonts.md) covers the required math face, optional text
  face, ordered fallbacks, face IDs, and font identity.
- [Parsing, diagnostics, and recovery](parsing.md) covers TeX and Typst input,
  source ranges, diagnostics, links, and recoverable output.
- [Errors and resource limits](errors-and-limits.md) describes fatal layout
  errors, parse-time recovery, and configurable limits.
- [Canonical IR and `Engine.LayoutFormulaContext`](canonical-ir.md) is for
  hosts that inspect or cache the language-neutral formula tree.
- [Concurrency, lifecycle, and caching](concurrency.md) describes safe reuse,
  cancellation, package-level caching, and ownership under concurrency.
- [Accessibility text](accessibility.md) explains `Alt.Text`, `Alt.Speech`,
  fallback markers, and how accessibility errors differ from layout errors.

## Compatibility and Verification

- [Supported TeX](tex-support.md) summarizes the tested TeX math subset and
  its explicit exclusions.
- [Supported Typst](typst-support.md) summarizes the restricted Typst math
  frontend and its explicit exclusions.
- [API stability tiers](stability.md) separates the host API, advanced IR
  types, and diagnostic serialization.
- [Release process](releasing.md) records versioning, module tags, and the
  automated GitHub release gate.
- [Security policy](../SECURITY.md) explains supported versions and private
  vulnerability reporting.
- [Compatibility and verification](quality.md) summarizes the tracked corpora
  and reviewed coverage.
- [Performance](performance.md) explains the benchmark workload, cache
  strategies, and allocation metrics.
- [Migrating to 0.2](migration.md) maps the removed low-level font and drawing
  conveniences to the current host facade, render helpers, and output adapters.
- [Migrating to 0.3](migration-v0.3.md) records editor recovery, shaping,
  diagnostics, source maps, and MyGo drawing changes.
- [Migrating to 0.4](migration-v0.4.md) records TeX sizing, overlap/raise
  primitives, source-row tags, and the new cache identity.
- [`encode/svg`](../encode/svg/doc.go) emits deterministic standalone SVG
  through the same framework-neutral drawing protocol.
- [`encode/raster`](../encode/raster/doc.go) renders a display box to a
  deterministic antialiased RGBA image or PNG without cgo or a UI dependency.

## Recommended Reading Paths

For a normal application:

1. [Getting started](getting-started.md)
2. [Rendering the display list](rendering.md)
3. [Fonts and fallback](fonts.md)
4. [Errors and resource limits](errors-and-limits.md)

For an editor or document application:

1. [Parsing, diagnostics, and recovery](parsing.md)
2. [Concurrency, lifecycle, and caching](concurrency.md)
3. [Accessibility text](accessibility.md)
4. The supported-syntax page for the active input dialect

For a host that stores or transforms formula trees:

1. [Canonical IR](canonical-ir.md)
2. [API stability tiers](stability.md)
3. [Compatibility and verification](quality.md)
4. [Performance](performance.md)

## Scope

Untie does not provide a document engine, font discovery, system keyboard
layout, DOM, network access, or semantic TeX/Typst execution. The host owns
those concerns. Untie's contract ends at a validated `Box`, its accessibility
text, and its source metadata.

The bundled [`examples/mygo`](../examples/mygo) application demonstrates direct
formula elements in a native editor.

For MyGo integration, use [`contrib/mygo`](../contrib/mygo/README.md). It can
render a formula directly as a `ui.Element`, bind Markdown math, or draw an
existing Untie box.
