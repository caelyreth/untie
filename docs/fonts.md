# Fonts and Fallback

Untie does not discover fonts. The host supplies validated OpenType bytes. Face
IDs are optional; when omitted, Untie derives stable IDs from the face role,
metadata, selected collection index, and bytes.

## Font Roles

```go
type FontFace struct {
	ID    string
	Bytes []byte
	Index int
}

type FontSetConfig struct {
	Math      FontFace
	Text      *FontFace
	Fallbacks []FontFace
}
```

`FontSetConfig.Math` is required. It must contain valid OpenType MATH constants,
including axis height, and valid vertical metrics. The engine also requires at
least one glyph in the primary math face.

`FontSetConfig.Text` is optional. It is tried first for text-role runs. If it is
absent, text-role runs begin with the math face.

`FontSetConfig.Fallbacks` are ordered. They are tried after the role-specific
primary face or faces.

`FontFace.Index` selects a face in a TrueType/OpenType collection. A non-zero
index is invalid for a single-face SFNT.

## Face IDs

Explicit IDs must be nonblank and unique across math, text, and fallback roles.
An empty ID is derived deterministically as a safe `<role>-<font-name>-<hash>`
identifier. The resolved ID becomes:

- `font.FaceInfo.ID`;
- the `FontID` on exact `ItemGlyphRef` records;
- part of the font-set identity used by the engine cache.

Derived IDs require no host bookkeeping and remain stable for identical input
bytes, selected index, and role. Supply an explicit ID when the host needs a
name tied to its own asset revision. Do not reuse one explicit ID for different
font bytes.

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

Fallback selection is per shaped unit, not a global font switch. Ordinary text
is segmented by Unicode bidi, script, and extended grapheme cluster before a
face is selected, so combining marks and emoji ZWJ sequences are not split
merely to change fonts. A CJK text run can use a fallback face while surrounding
math uses the primary math face. The host can add a CJK face to `Fallbacks`
without changing math symbols.

When a single face covers a complete bidi/script run, it is used for that whole
run to preserve contextual shaping. For complex text scripts, an explicit text
face or fallback is preferred over the primary MATH face when both nominally
cover the run. Latin, Greek, and Cyrillic continue to prefer the normal face
order.

If no face maps the required rune or variation sequence, the default
`RecoveryVisible` layout keeps the rest of the formula drawable. It substitutes
an error-painted placeholder box, records a `missing-glyph` diagnostic on both
the box and high-level render result, and continues with the surrounding row.
Use `RecoveryStrict` when a complete font match is required.

## Structured Face Usage

Layout records text-shaping provenance separately from diagnostics:

```go
box.FontUsage.Faces   // source span, resolved face ID, fallback marker
box.FontUsage.Missing // source span, missing text, ordered tried face IDs
```

`Faces` is sorted and merged in source order. Adjacent clusters that use the
same face and state share one `FaceUse`. `Fallback` means the selected face was
not the first face requested for that text role; the optional text face and
primary math face are therefore not automatically labelled fallback.

`Missing` is deduplicated and merges adjacent clusters that tried the same faces
in the same order. Its `Text` and `TriedFaces` fields tell a host what coverage
is still needed without parsing diagnostic messages. This is reporting data,
not a second error channel: `CodeMissingGlyph` diagnostics remain authoritative
for editor ranges and visible recovery.

The report covers shaped text clusters, not math atoms selected directly through
OpenType MATH tables. It is empty when a formula has no shaped text.

Color-only emoji formats such as sbix, CBDT, and COLR are not part of the 0.3
outline-only font and display contract. When such coverage is required, load a
monochrome outline font or let the missing cluster use visible recovery.

## Unicode and Variation Selectors

The adapter tries a direct rune mapping, then a rune plus variation selector
mapping when one is present. It also applies the font's supported math GSUB
features before drawing. A fallback face must contain the needed variation
sequence to satisfy a source that requests it.

## What `LoadFontSet` Validates

`LoadFontSet` fails cleanly, without a partial `FontSet`, when:

- the math face is absent or empty;
- duplicated explicit or resolved face IDs collide;
- an SFNT/TTC face cannot be parsed;
- the primary math face lacks usable MATH constants;
- a face has invalid units per em, glyph count, metrics, or outlines;
- there are more fallback faces than the public `FaceRef` space allows.

Every preparation failure matches `errors.Is(err, untie.ErrInvalidFonts)`
while preserving the underlying font parser error through `errors.Unwrap`.

Malformed glyph outlines are rejected at load time so later layout does not
fail because of a malformed glyph program.

## Byte Ownership

`LoadFontSet` and `LoadMathFont` copy every byte slice before parsing. Mutating
the caller's `FontFace.Bytes` afterward cannot change the `FontSet` or its key.

`FontSet.Config()` returns an independent copy of the input font
configuration. Mutating the copy cannot affect the loaded set.

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
contract.

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

A missing glyph is a recoverable layout problem by default:

- `RendererOptions.Recovery == RecoveryVisible` returns a non-nil box with an
  error-painted placeholder for the missing item;
- `RenderResult.Diagnostics` and `Box.Diagnostics` contain
  `untie.CodeMissingGlyph` with the attributed source range;
- surrounding symbols, fractions, scripts, and rows remain in the display list;
- hosts that require a complete font match can use `RecoveryStrict`, in which
  case `errors.Is(err, untie.ErrMissingGlyph)` and the structured
  `*untie.LayoutError` remain available.

The reference font set intentionally triggers visible CJK recovery because it
does not contain a CJK face. Install a fallback face to render the characters
instead of the placeholder.

## Recommended Setup

```go
text := untie.FontFace{Bytes: textBytes}
fonts, err := untie.LoadFontSet(untie.FontSetConfig{
	Math: untie.FontFace{Bytes: mathBytes},
	Text: &text,
	Fallbacks: []untie.FontFace{
		{Bytes: broadUnicodeBytes},
	},
})
if err != nil {
	return err
}

renderer, err := untie.NewRenderer(fonts, untie.RendererOptions{
	DefaultStyle: untie.Style{Size: 24},
	CacheEntries: 128,
})
```

For one math face, the shorter form also accepts TTC selection:

```go
fonts, err := untie.LoadMathFont(mathBytes,
	untie.WithFontID("host-math-v1"),
	untie.WithFontIndex(0),
)
```

Use `FontFace`, `FontSetConfig`, and `LoadFontSet` or `LoadMathFont` for all
font setup.

## Filesystem Helpers

The optional `github.com/caelyreth/untie/fontsource` package reads host-provided
files through `io/fs`. It works with `os.DirFS`, `embed.FS`, and test
filesystems; it performs no system-font discovery or network lookup.

```go
fonts, err := fontsource.LoadMath(os.DirFS("assets"), "NewCMMath-Book.otf")
```

Use `fontsource.LoadSet` for math, text, and ordered fallback files. Parsing and
byte ownership still happen in `font.LoadFontSet`.

Keep the font set and renderer for the lifetime of the configured graphics
environment. There is no font reload or close callback.

## Licensing Boundary

Untie does not determine whether a host may redistribute or embed a supplied
font. The host is responsible for the font's license and for preserving any
required notices.
