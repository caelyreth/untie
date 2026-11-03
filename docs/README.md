# Untie Documentation

Untie parses TeX or Typst math into one language-neutral IR and lays that IR
out with a host-supplied OpenType MATH font. The result is a UI-independent
display list of filled glyph paths, filled rules, source hits, break metadata,
and accessibility text.

These docs describe the public API at library version `0.1.0` and layout
revision `1`. They focus on the behavior a host or code assistant can rely on
when integrating Untie; they are not a TeX or Typst language tutorial.

## Start Here

- [Getting started](getting-started.md) shows the complete
  `LoadMathFont` -> `NewEngine` -> `Parse` -> `Layout` -> draw flow.
- [Core concepts and ownership](concepts.md) explains the pipeline, the
  lifetime of each value, and the inline/display boundary.
- [Rendering the display list](rendering.md) defines coordinates, items,
  lines, tags, hits, overflow, and bounds.

## Integration Guides

- [MyGo integration](mygo.md) provides the bounded adapter for MyGo Markdown
  math, theme-role drawing, and framework-neutral box drawing.
- [Fonts and fallback](fonts.md) covers the required math face, optional text
  face, ordered fallbacks, face IDs, and font identity.
- [Parsing, diagnostics, and recovery](parsing.md) covers TeX and Typst input,
  source ranges, diagnostics, links, and recoverable output.
- [Errors and resource limits](errors-and-limits.md) describes fatal layout
  errors, parse-time recovery, every limit, and the layer that enforces it.
- [Canonical IR and `Engine.LayoutFormulaContext`](canonical-ir.md) is for
  hosts that inspect or cache the language-neutral formula tree.
- [Concurrency, lifecycle, and caching](concurrency.md) describes safe reuse,
  cancellation, package-level caching, and ownership under concurrency.
- [Accessibility text](accessibility.md) explains `Alt.Text`, `Alt.Speech`,
  fallback markers, and how accessibility errors differ from layout errors.

## Compatibility and Evidence

- [Supported TeX](tex-support.md) summarizes the tested TeX math subset and
  its explicit exclusions.
- [Supported Typst](typst-support.md) summarizes the restricted Typst math
  frontend and its explicit exclusions.
- [API stability tiers](stability.md) separates the host API, advanced IR
  types, diagnostic serialization, and internal packages.
- [Compatibility evidence](quality.md) lists the tracked corpora and the
  commands that verify parser, layout, font, and corpus contracts.
- [Performance](performance.md) explains the tracked corpus benchmarks,
  cache strategies, allocation metrics, and profiling commands.

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
3. [Compatibility evidence](quality.md)
4. [Performance](performance.md)

## Scope

Untie does not provide a document engine, font discovery, system keyboard
layout, DOM, network access, or semantic TeX/Typst execution. The host owns
those concerns. Untie's contract ends at a validated `Box`, its accessibility
text, and its source metadata.

The bundled `examples/mygo` application is a complete drawing integration for
the public display list. The example module is separate from the library; the
library itself has no UI dependency.

For plug-and-play Markdown math, use the separately versioned
[`contrib/mygo`](../contrib/mygo/README.md) adapter. Importing the root module
never imports MyGo.
