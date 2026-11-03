# font package

`font` is Untie's immutable OpenType substrate. It accepts host-owned SFNT or
TTC bytes and exposes design-unit metrics, cubic/quadratic outlines, MATH
constants and constructions, Unicode variation mapping, and the `ssty`/`dtls`
GSUB features needed by math layout.

## Integration

```go
set, err := font.LoadFontSet(font.FontSetConfig{
    Math: font.FontFace{
        Bytes: newCMMathBytes,
        Index: 0,
        ID:    "newcm-math-book-v4",
    },
})
if err != nil {
    // No partial FontSet is returned.
}

g, ok := set.MapRune(font.MathFace, '\U0001D465') // mathematical italic x
metrics, ok := set.GlyphMetrics(g)                // design units
outline, ok := set.Outline(g)                     // y-down design units
scaled, ok := set.ShapeMath(font.MathFace, []font.GlyphID{g.ID},
    font.ScriptVariant, false)

shaped, err := set.ShapeText(font.TextShapeRequest{
    Context: ctx,
    Text:    "a\u0301 सालाम 👩‍💻",
    Faces:   []font.FaceRef{font.TextFace, font.MathFace},
    MaxGlyphs: 4096,
})
```

`FontSet` is safe for concurrent readers. `Outline` returns a defensive copy;
the internal cache is immutable. `KeySHA256` is the collision-resistant font
set identity. `Key` is the first 64 bits for host maps.

Coordinates and advances are whole font design units. CFF 16.16 operands are
quantized once to this integer grid by truncating toward zero. They are never
rounded through a host or canvas pixel size. Sub-design-unit outline coordinates
are intentionally not exposed by this API. Bounds are control bounds in y-down
design units.

## Intentional boundaries

- The host supplies bytes and optional IDs. There is no system font lookup,
  family matching, or environment-dependent fallback.
- Empty IDs are derived from role, metadata, selected index, and bytes. Explicit
  IDs must be nonblank and unique across the math, text, and fallback faces.
- The primary math face must contain OpenType MATH constants.
- `ssty` and `dtls` are parsed locally because `ssty` requires alternate-feature
  values. Lookup flags on those math features are rejected at load rather than
  silently ignored.
- Public feature tags are exact, case-sensitive four-byte OpenType tags.
  Disabled and unsupported values are ignored. `Shape` preserves caller order;
  `ShapeMath` always applies `dtls` before `ssty`.
- `MathConstruction` exposes variants and the complete assembly recipe:
  assembly italic correction, minimum connector overlap, ordered parts, part
  glyph IDs, connector lengths, full advances, and extender flags. Variant
  selection remains a layout policy and is not implemented in `font`.
- Ordinary text runs use the pure-Go HarfBuzz shaper from `go-text/typesetting`
  after Unicode bidi and script segmentation. Fallback is resolved once per
  extended grapheme cluster, and shaped clusters retain byte and rune ranges for
  editor hit testing.
- When one face covers a complete bidi/script run, it is preferred for the
  whole run so joining and reordering stay intact. For non-Latin, non-Greek,
  and non-Cyrillic scripts, a supplied text or fallback face is preferred over
  the MATH face when both have nominal coverage.
- Math symbols, stretchy constructions, and named operators remain on the
  dedicated cmap/MATH/GSUB path. General text ligatures and kerning do not leak
  into formula-symbol layout.
- Font parsing and shaping support are pure Go and require no cgo toolchain.

## Compatibility

The package is validated against a pinned New Computer Modern Math Book
reference covering all 8,598 glyph advances, control bounds, italic
corrections, accent attachments, extended-shape membership, kerns, variants,
assemblies, cmap entries, variation sequences, and `ssty`/`dtls` shaping.

Four variation-sequence glyphs (`8465`, `8480`, `8481`, and `8483`) differ by
one design unit because `ttf-parser` composes their legacy CFF endchar-seac
components while Untie's outline parser does not. U+FFFF maps to `.notdef` and
public rune lookup reports it as unmapped. Untie also retains the one
spec-defined left-corner kern omitted by the reference data for prescript
layout.

