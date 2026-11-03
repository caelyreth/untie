# Migrating To 0.4

Untie 0.4 adds core output policy, shaped-text provenance, and the complete
MyGo editor integration surface. The runtime version is `0.4.0+layout.3`, and
the process-wide Engine cache schema is `3`.

Most integrations compile unchanged because every new policy is disabled at its
zero value. Applications that persist boxes, canonical snapshots, or adapter
cache keys should update their application-owned version.

## Core Color Policy

`Style.ColorPolicy` now controls authored explicit colors before the display
list is returned:

```go
style := untie.Style{
	Size: 24,
	ColorPolicy: untie.ColorPolicy{
		Mode: untie.ColorAdjustContrast,
		Background: untie.RGBA{R: 0x10, G: 0x12, B: 0x16, A: 0xff},
		MinContrast: 4.5,
	},
}
```

The modes are:

- `ColorPreserve`: the zero value; explicit RGBA values remain byte-for-byte
  unchanged;
- `ColorMapToRoles`: maps explicit colors to the perceptually nearest ink,
  error, or placeholder role in a required `ColorRolePalette`;
- `ColorAdjustContrast`: preserves explicit alpha and minimally changes RGB
  against an opaque background until the requested WCAG contrast ratio is met
  when the alpha permits it.

Semantic role paints are never rewritten. Resolved output still contains only
serializable `display.Paint` values, so MyGo `PaintResolver` continues to work
without a core UI dependency.

## Structured Face Usage

`Box.FontUsage` reports shaped text provenance:

- `Faces` contains source-ordered, merged `FaceUse` spans with resolved face IDs
  and a `Fallback` marker;
- `Missing` contains source-ordered, deduplicated `MissingFallback` records with
  missing text and the ordered face IDs that were tried.

`FontUsage.UsesFallback()` and `FontUsage.NeedsFallback()` provide quick policy
checks. These records do not replace `CodeMissingGlyph` diagnostics; diagnostics
remain authoritative for editor ranges and localized presentation.

The report covers text shaping. Math atoms selected directly through OpenType
MATH tables remain represented by their existing glyph items.

## Pixel-Grid Snapping

`Style.PixelGrid` opts into conservative snapping for thin rules and stems:

```go
style.PixelGrid = untie.PixelGrid{
	Scale:      2,   // host units to output pixels
	PixelRatio: 2,   // output pixels to device pixels
}
```

Only horizontal or vertical rules between 0.25 and 3 physical pixels thick are
adjusted, and only on their minor axis. Glyph outlines, paths, and thick rules
are unchanged. The zero grid is a no-op.

`Item.Snap` and `render.Command.Snap` expose the candidate axis even when
snapping is disabled, allowing hosts to implement an equivalent draw-time
policy without re-inferring geometry.

## Semantic Accessibility

Standalone MyGo elements now request semantic alternative text and expose:

- `Alt.Speech` as the label, with `Alt.Text` and source as fallbacks;
- exact TeX or Typst source as the value;
- a stable non-empty name for empty formulas.

Set `Options.Accessibility = untiemygo.AccessibilityDisabled` to retain
source-only labels without the alternative-text traversal. Use
`DescribeFormula` when an application needs the same deterministic fallback
policy outside an element.

MyGo v0.7.4's `markdown.InlineObject` has no per-object accessibility metadata.
Inline formulas continue to participate in the paragraph's source-copy and
selection behavior, but cannot independently attach spoken alternative text
through the pinned adapter API.

## MyGo Editor Elements

The new options-bearing element methods add:

- `OnHit` hover, press, and click callbacks resolved through `Box.At`;
- source-range diagnostic underlines and whole-element tooltips;
- plain-text and dialect-source clipboard context menu actions.

`HitAt` exposes the inverse draw transform for applications that do not use
MyGo input callbacks. `Options.Inline.Metrics` can report nominal baseline
metrics with overflow ink kept drawable through the surrounding text clip.

`FormulaClipboardData` and the context-menu option expose plain text and custom
TeX/Typst source MIME formats. MathML is not offered until Untie has a real
MathML serializer.

## Display And Cache Compatibility

- `display.Box` gained `FontUsage`;
- `display.Item` gained `Snap`;
- canonical box JSON may include `fontUsage` and `snap`;
- MyGo layout keys now include `ColorPolicy` and `PixelGrid`;
- `untie.Version()` is now `0.4.0+layout.3`;
- the Engine cache schema is now `3`.

Drop persisted prepared-engine, layout, or canonical-output caches from 0.3.
