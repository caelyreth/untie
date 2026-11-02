# font package

`font` is Untie's immutable OpenType substrate. It accepts host-owned SFNT or
TTC bytes and exposes design-unit metrics, cubic/quadratic outlines, MATH
constants and constructions, Unicode variation mapping, and the `ssty`/`dtls`
GSUB features needed by math layout.

## Integration

```go
set, err := font.Load(font.Fonts{
    Math: font.FontData{
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
```

`FontSet` is safe for concurrent readers. `Outline` returns a defensive copy;
the internal cache is immutable. `KeySHA256` is the collision-resistant font
set identity. `Key` is the first 64 bits for host maps.

Coordinates and advances are whole font design units. CFF 16.16 operands are
converted once to this design-unit grid; they are never rounded through a host
or canvas pixel size. Bounds are control bounds in y-down design units, matching
the Typst/NewCM fixture used by the tests.

## Intentional boundaries

- The host supplies bytes and IDs. There is no system font lookup, family
  matching, or environment-dependent fallback.
- The primary math face must contain OpenType MATH constants.
- `ssty` and `dtls` are parsed locally because `ssty` requires alternate-feature
  values. Lookup flags on those math features are rejected at load rather than
  silently ignored.
- Text shaping is outside this package. `MapRune` and glyph advances are
  deterministic primitives for a later text shaper.
- The two pinned parser dependencies are pure Go:
  `github.com/go-opentype/opentype` at
  `2c8922cb87c69d7555957e90370c19f890758b36`, and
  `github.com/go-text/typesetting` at `v0.3.5`.

## Conformance data

`testdata/NewCMMath` tracks the unmodified NewCM Book font and Typlet's complete
NewCM JSONL fixture. The tests compare all 8,598 glyph advances, control bounds,
italic corrections, accent attachments, extended-shape membership, kerns,
variants, and assemblies, plus every cmap character, variation sequence, and
`ssty`/`dtls` shaping result.

Four private-use glyph bounds differ by one design unit because CFF source
coordinates near an integer are quantized differently by the upstream
fixture. Those IDs are listed explicitly in `fixture_test.go`. The fixture's
U+FFFF entry maps to `.notdef`; public rune lookup intentionally reports that
as unmapped. The fixture also omits one spec-defined left-corner kern; Untie
retains it for prescript layout. No other differences are accepted.

