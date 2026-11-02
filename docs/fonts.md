# Fonts and Fallback

Untie does not discover fonts. The host supplies validated OpenType bytes and a
stable ID for every face in the set.

## Font Roles

```go
type Fonts struct {
	Math      FontData
	Text      *FontData
	Fallbacks []FontData
}

type FontData struct {
	Bytes []byte
	Index int
	ID    string
}
```

`Fonts.Math` is required. It must contain valid OpenType MATH constants,
including axis height, and valid vertical metrics. The engine also requires at
least one glyph in the primary math face.

`Fonts.Text` is optional. It is tried first for text-role runs. If it is
absent, text-role runs begin with the math face.

`Fonts.Fallbacks` are ordered. They are tried after the role-specific primary
face or faces.

`FontData.Index` selects a face in a TrueType/OpenType collection. A non-zero
index is invalid for a single-face SFNT.

## Face IDs

Every supplied face must have a nonblank, unique ID across math, text, and
fallback roles. The ID becomes:

- `font.FaceInfo.ID`;
- the `FontID` on exact `ItemGlyphRef` records;
- part of the font-set identity used by the engine cache.

Use an ID that is stable for the host's glyph cache, for example a font name
plus revision. Do not reuse one ID for different font bytes.

## Fallback Order

Fallback is deterministic and role-specific.

For math-role glyphs:

1. primary math face;
2. fallback 0;
3. fallback 1;
4. and so on.

For text-role glyphs:

1. optional text face;
2. primary math face;
3. fallback 0;
4. fallback 1;
5. and so on.

The text face does not become the source of math symbols. The math face is
still required for operators, MATH constants, script variants, stretchy
constructions, and math italics.

Fallback selection is per shaped unit, not a global font switch. A CJK text
run can use a fallback face while surrounding math uses the primary math face.
The host can add a CJK face to `Fallbacks` without changing math symbols.

If no face maps the required rune or variation sequence, layout returns an
error wrapping `ErrMissingGlyph`. There is no automatic visible placeholder
during layout.

## Unicode and Variation Selectors

The adapter tries a direct rune mapping, then a rune plus variation selector
mapping when one is present. It also applies the font's supported math GSUB
features before drawing. A fallback face must contain the needed variation
sequence to satisfy a source that requests it.

## What `LoadFonts` Validates

`LoadFonts` fails cleanly, without a partial `FontSet`, when:

- the math face is absent or empty;
- a face ID is blank or duplicated;
- an SFNT/TTC face cannot be parsed;
- the primary math face lacks usable MATH constants;
- a face has invalid units per em, glyph count, metrics, or outlines;
- there are more fallback faces than the public `FaceRef` space allows.

Malformed glyph outlines are rejected at load time so later layout does not
fail because of a malformed glyph program.

## Byte Ownership

`LoadFonts` copies every byte slice before parsing. Mutating the caller's
`FontData.Bytes` afterward cannot change the `FontSet` or its key.

`FontSet.Fonts()` returns an independent copy of the input font data. Mutating
the copy cannot affect the loaded set.

## Identity and Caching

`FontSet.KeySHA256()` is the canonical identity of the complete font set,
including face IDs and selected collection indexes. `FontSet.Key()` is its
first 64 bits for smaller host maps.

The package-level layout cache uses the full SHA-256 identity. This means two
sets with identical bytes but different IDs are distinct cache entries. It also
means replacing a fallback or text face invalidates the prepared engine.

## OpenType Substrate

The public `font` package exposes more than the root facade needs:

- face metadata and vertical metrics;
- Unicode cmap and variation-selector mapping;
- glyph advances, control bounds, and y-down outlines;
- MATH constants, kerns, top-accent attachment, variants, and assemblies;
- exact four-byte GSUB feature tags;
- one-to-one `Shape` and the `ShapeMath` `dtls`/`ssty` path.

Coordinates are whole font design units in the `font` package. CFF fractional
coordinates are quantized once by truncating toward zero. The layout engine
converts those design units into deterministic host-unit geometry.

See [`font/README.md`](../font/README.md) for the lower-level integration
contract and the fixture provenance.

## Layout Style Features

`Style.Features` accepts up to 64 strings in `tag` or `tag=value` form. Tags are
one to four ASCII letters or digits; values are decimal `uint16` values.
Style validation checks syntax, not whether a particular face contains the
feature.

Math layout gives special handling to:

- `dtls` and `flac` for flattening accents;
- `ssty` script alternates selected from the active script level.

Other valid tags are passed to the font's one-to-one substitution support.
Use `FontSet.HasFeature` and `FontSet.Features` when a host needs to inspect
available tags directly.

## Missing Glyphs

A missing glyph is a layout error, not a parse diagnostic:

- `errors.Is(err, untie.ErrMissingGlyph)` identifies it;
- the host can retry with an additional fallback face;
- the parse tree and its `Expr.Alt()` remain available even when layout fails.

The daily corpus intentionally includes text-only CJK cases that produce
`ErrMissingGlyph` with the New Computer Modern fixture. They document the
fallback boundary rather than pretending the font covers every script.

## Recommended Setup

```go
text := untie.FontData{
	Bytes: textBytes,
	ID:    "host-text-v1",
}

fonts, err := untie.LoadFonts(untie.Fonts{
	Math: untie.FontData{
		Bytes: mathBytes,
		ID:    "host-math-v1",
	},
	Text: &text,
	Fallbacks: []untie.FontData{
		{Bytes: broadUnicodeBytes, ID: "host-fallback-v1"},
	},
})
if err != nil {
	return err
}

engine, err := untie.NewEngine(fonts, untie.Limits{})
```

Keep the font set and engine for the lifetime of the configured graphics
environment. There is no font reload or close callback.

## Licensing Boundary

Untie does not determine whether a host may redistribute or embed a supplied
font. The host is responsible for the font's license and for preserving any
required notices. The repository's test font has separate provenance and
license material under `testdata/upstream`.

## Evidence

- Font input and loading: [`font/font.go`](../font/font.go)
- Public font types: [`font/types.go`](../font/types.go)
- Located font documentation: [`font/README.md`](../font/README.md)
- Input ownership and TTC tests: [`font/font_test.go`](../font/font_test.go)
- Fallback-order implementation: [`internal/engine/ports.go`](../internal/engine/ports.go)
- Shaping path: [`internal/engine/shaping.go`](../internal/engine/shaping.go)
- Missing-glyph corpus cases: [`testdata/daily-stem/cases.jsonl`](../testdata/daily-stem/cases.jsonl)

