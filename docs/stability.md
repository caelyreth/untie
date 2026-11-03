# API Stability Tiers

Untie is currently version `0.4.0` with layout revision `3`. This page defines
how these docs describe stability; it does not add a guarantee that is absent
from the code.

## Tier 1: Host Integration API

Primary packages:

- `github.com/caelyreth/untie`
- the host-facing types in `github.com/caelyreth/untie/display`
- `github.com/caelyreth/untie/font` for `FontFace`, `FontSetConfig`, and `FontSet`
- `github.com/caelyreth/untie/fontsource` for `io/fs` loading helpers
- `github.com/caelyreth/untie/render` for framework-neutral drawing, path
  resolution, and viewport fitting
- `github.com/caelyreth/untie/contrib/mygo` for MyGo elements, Markdown math,
  and painter integration

Primary surface:

- `LoadFontSet`, `LoadMathFont`, `WithFontID`, `WithFontIndex`, `NewEngine`,
  `Engine.Layout`, `Engine.LayoutContext`,
  `Engine.LayoutContextWithOptions`;
- `FontSet.ShapeText`, `TextShapeRequest`, `ShapedText`, `ShapedCluster`,
  `ShapedGlyph`;
- `NewRenderer`, `RendererOptions`, `RenderRequest`, `RenderResult`,
  `Renderer.Render`, `RecoveryMode`, `LayoutOptions`;
- `render.Walk`, `render.Command`, `render.ResolvePath`, `render.Fit`,
  `render.FitOptions`, and `render.Transform`;
- `(*untiemygo.Renderer).Element`, `BindMarkdown`, `InlineMath`, `DisplayMath`,
  `DisplayMathWidth`, `Draw`, `DrawBox`, `PaintResolver`, `ScaleMode`, and
  `CacheStats` in the optional MyGo adapter;
- `Parse`, `ParseContext`, `ParseTeX`, `ParseTypst`, `Expr`;
- `Layout`, `LayoutContext`, `ClearLayoutEngineCache`;
- `Style`, `Box`, `Item`, `Paint`, `Hit`, `Break`, `LineBox`, `TagBox`, `Alt`,
  and `Box.Diagnostics`;
- public error sentinels and diagnostic severities/codes.

Tier 1 is the recommended integration surface. Breaking changes are allowed
while the library is pre-1.0, but documented public behavior remains the
compatibility target.

## Tier 2: Advanced Data Contracts

Packages:

- `github.com/caelyreth/untie/ir`
- `github.com/caelyreth/untie/source`
- `github.com/caelyreth/untie/geometry`
- `github.com/caelyreth/untie/encode/raster`
- `github.com/caelyreth/untie/encode/svg`
- low-level introspection in `github.com/caelyreth/untie/font`

This tier supports:

- canonical IR inspection and caching;
- `RootSnapshot`, `ValidateSource`, `CloneFormula`, `WalkFormula`;
- `Engine.LayoutFormulaContext`;
- source ranges, diagnostic codes, and resource limits;
- optional SVG or PNG output;
- exact glyph metadata, feature tags, MATH tables, and font identity.

These are public and intended to be usable, but they expose more of the data
model. Changes are more likely to require host code changes than a Tier 1
drawing integration. SVG output is explicitly not a stable wire protocol.

## Tier 3: Diagnostic Serialization

Functions:

- `ir.MarshalCanonical`;
- `display.Box.MarshalCanonical`;
- `display.Alt.MarshalCanonical`;
- corpus and report schemas.

The canonical serializers produce deterministic JSON for diagnostics and local
inspection. They are not stable cross-language protocols or a public
interchange format.

Use them for local debugging and application-controlled snapshots.

Do not use them as an unversioned external wire protocol.

## Version and Cache Compatibility

`untie.Version()` combines a library version and layout revision:

```text
0.4.0+layout.3
```

The package-level engine cache includes that whole string. A layout behavior
change that invalidates prepared engine assumptions increments the layout
revision.

Font identity is separate and collision-resistant:

- `FontSet.KeySHA256()` for full identity;
- `FontSet.Key()` for a 64-bit convenience key.

Expression identity is convenience-only:

- `Expr.Key()` is 64-bit;
- it includes source, dialect, mode, version, parse-affecting limits, and TeX
  macros;
- it is not cryptographic.

## Diagnostic Codes

The source package documents diagnostic codes as stable semantic identifiers.
New codes may be added, but an existing code should not acquire a contradictory
meaning. Match with `Diagnostic.Code`, not message text.

Messages are human-readable and may improve. `Args` and `Hints` are supporting
metadata, not a stable localization format.

## Layout Error Kinds

`LayoutErrorKind` values are stable semantic identifiers. Match with
`LayoutError.Kind`, not the error message. New kinds may be added when a new
failure category is introduced.

`LayoutError.Range` is an attributable half-open source byte range or
`source.Detached()`. Its exact node may improve as source attribution becomes
more precise; hosts should treat it as editor guidance rather than a parser
serialization format.

## Font IDs

Resolved face IDs are part of the public cache and glyph-reference contract:

- empty IDs are derived from role, metadata, selected index, and bytes;
- explicit IDs must be nonblank and unique within a `FontSet`;
- derived and explicit IDs must not collide;
- `ItemGlyphRef.FontID` is the resolved ID;
- changing an ID changes the font-set SHA-256 identity.

Treat a face ID as a persisted cache identifier. Changing it intentionally
invalidates host caches.

## JSON and Report Schemas

Tracked corpus and report files have their own schema versions. Those versions
do not automatically define a public application protocol.

If a host persists canonical IR or reports, add its own application-level
version, migration, and validation layer.

## Compatibility Checklist

- Import only root and documented public packages.
- Use `errors.Is` for layout sentinels and `errors.As` for structured
  `LayoutError` details.
- Do not depend on diagnostic message spelling.
- Do not parse canonical JSON as a long-lived external format.
- Pin the Untie version used by persisted caches.
- Recompute font keys after changing bytes, IDs, indexes, or fallback order.
- Use `Version()` rather than assuming two binary versions share layout
  behavior.
