# Migrating To 0.3

Untie 0.3 turns the renderer into a complete editor-facing integration surface.
The runtime version is `0.3.0+layout.2`.

## Visible Recovery Is The Default

`RendererOptions.Recovery` and `LayoutOptions.Recovery` now default to
`RecoveryVisible`.

A missing glyph or shaped text cluster:

- remains in a non-nil `Box`;
- is drawn as an error-painted placeholder;
- leaves the rest of the formula drawable;
- appears in `RenderResult.Diagnostics` and `Box.Diagnostics` with
  `CodeMissingGlyph`.

Use `RecoveryStrict` to restore the previous fail-closed layout behavior.
Strict failures still return a structured `*LayoutError` and now also include a
normalized diagnostic in `RenderResult.Diagnostics`.

## General Text Uses Real Shaping

Text-mode runs now use pure-Go OpenType shaping through
`github.com/go-text/typesetting`, including bidi ordering, script segmentation,
kerning, ligatures, mark positioning, and per-cluster fallback.

Math symbols and named operators remain on the OpenType MATH/GSUB path. The new
shaping behavior can change text metrics and glyph choices even when the source
is unchanged.

`FontSet.ShapeText` and the `TextShapeRequest`/`ShapedText` types expose the
lower-level shaping contract. Color-only emoji fonts and bitmap/COLR emoji are
not part of the 0.3 outline-only display contract; unsupported clusters use
visible recovery.

## Text Nodes Carry Source Clusters

`ir.Text.Clusters` may contain parser-authored `ir.TextClusterSource` values
that map decoded text bytes to absolute source bytes.

When present, layout emits `HitAtom` records per visual shaping cluster. When
the frontend cannot prove an ordered, complete source map, the slice is empty
and the previous whole-node text hit remains available.

## MyGo Drawing

The `contrib/mygo` module continues to require MyGo v0.7.4.

New editor-facing APIs:

- `DisplayMathWidth` wraps against a known width before drawing;
- `DrawBox` draws an existing `untie.Box` without a renderer;
- `PaintResolver` permits host-specific color policy;
- `ScaleFit` preserves compact preview behavior;
- `ScaleNatural` preserves wrapping and native glyph metrics.

Explicit formula colors remain authoritative by default. Use a custom
`PaintResolver` to intentionally map explicit or semantic paints.

## Cache Identity

The library version, layout revision, and Engine cache schema changed:

- `untie.Version()` is now `0.3.0+layout.2`;
- the process-wide Engine cache schema is `2`.

Drop persisted layout or display caches created by 0.2.
