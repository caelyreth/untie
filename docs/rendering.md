# Rendering the Display List

`Box` is both the measured result of layout and the complete display contract.
It contains no canvas handle, DOM node, texture, or platform text run. A host
draws its paths and rules, resolves paints, optionally caches exact glyphs, and
uses source hits for interaction.

Display math is "block-style" in the mathematical sense, not a block-layout
element in the host document. Untie returns a box; the host decides where that
box sits in a page, paragraph, or editor pane. Inline math returns an inline
box but still leaves paragraph placement to the host.

## Coordinates and Units

All public geometry is in the layout's host units:

- x increases to the right;
- y increases downward;
- the box baseline origin is `(0, 0)`;
- `Box.Ascent` and `Box.Descent` are positive distances above and below the
  baseline;
- glyph paths use the same y-down coordinates as the rest of the box.

The top of the nominal box is `-Box.Ascent`. The bottom is `Box.Descent`.
`Box.Width` is the horizontal advance of the complete box; it is not derived
from the ink bounds.

`Box.Italic` is the signed OpenType MATH italic correction at the box's final
horizontal endpoint. It may be negative. Do not clamp it to zero or use it as a
hit-test width.

## What a Box Contains

```go
type Box struct {
	Width      float32
	Ascent     float32
	Descent    float32
	Italic     float32
	AxisHeight float32
	Overflow   float32

	Items  []Item
	Hits   []Hit
	Breaks []Break
	Alt    Alt
	Tag    *TagBox
	Lines  []LineBox
}
```

The slices are owned by the returned box. A new successful layout creates new
box storage.

## Drawing Items

`Item` is already positioned. Draw it at the host's chosen box origin; do not
re-run TeX spacing, font shaping, or script placement in the host.

### `ItemGlyphPath`

`box.Paths[item.PathIndex]` is a shared fillable path template with `MoveTo`,
`LineTo`, `QuadTo`, `CubeTo`, and `Close` segments. Template points are in
local em units. Resolve each point in box coordinates as:

```text
x = item.At.X + point.X*item.Em
y = item.At.Y + point.Y*item.Em
```

`item.Bound` is the already-placed conservative rectangular extent. Templates
may be shared by many items, so do not mutate `box.Paths` or apply item
placement to the template itself.

The path may contain multiple contours. When a glyph uses a hole, the contours
are wound so that either nonzero or even-odd filling produces the intended
hole.

### `ItemRule`

`item.Rect` is a filled rectangle in box coordinates. Rules are used for
fraction bars, radicals, overlines, table boundaries, boxes, and other
constructed geometry.

### `ItemGlyphRef`

`ItemGlyphRef` identifies an exact font glyph that the host may resolve through
its own glyph atlas:

- `FontID`: the host-provided face ID;
- `Glyph`: the OpenType glyph index;
- `At`: the glyph origin in box coordinates;
- `Em`: the glyph's em size;
- `Bound`: the placed glyph bounds;
- `Paint`: the requested paint.

When `Style.GlyphRefs` is true, an outlined glyph produces both its path and a
reference. The reference is cache metadata, not a second visible shape. A glyph
with no outline can produce a reference without a path.

Draw only one representation of the same glyph. The simplest host draws
`ItemGlyphPath` and `ItemRule`, ignores references, and treats a reference-only
item as non-visible geometry. An optimized host can use exact references to
cache glyphs.

### Paint

`Paint` is either semantic or explicit:

- `HasExplicit == true`: use `Explicit` RGBA.
- otherwise: resolve `RoleInk`, `RoleError`, or `RolePlaceholder` through the
  host theme.

Error and placeholder roles are intentional visible fallback states. Do not
silently paint them as normal ink unless the host has a better accessibility
policy.

## Inline Layout

`Inline` layout returns a box with:

- no `Lines`;
- no use of `Style.MaxWidth` or `Style.Align`;
- inline break opportunities in `Box.Breaks`;
- the complete formula as `Box.Items`.

The host owns placement in a paragraph or text baseline. Untie does not choose
a surrounding paragraph's ascent, descent, or line height. The inline box's
ascent and descent are already suitable for the font and formula.

## Unconstrained Display Layout

`Display` layout with `Style.MaxWidth == 0` produces a free-standing display
box:

- `Lines` is empty;
- the complete formula is in `Items`;
- `Width` is the natural formula width;
- `Align` is irrelevant.

## Constrained Display Layout

`Display` layout with a positive `Style.MaxWidth` produces a constrained frame:

- `Box.Width` equals the requested `MaxWidth`, including when the formula fits;
- `Box.Lines` has at least one line;
- `Box.Items` and `Box.Hits` are the only stored item and hit slices and use
  parent-frame coordinates;
- each `LineBox.Items` and `LineBox.Hits` is a contiguous range into those
  parent slices;
- each line has explicit `Width`, `Ascent`, `Descent`, `Overflow`, `Baseline`,
  and `OffsetX` metadata;
- `Box.Overflow` covers the assembled frame and logical width excesses.

Draw `Box.Items` once. Use a line's ranges when the host needs clipping,
line-local interaction, or per-line metadata; do not apply another coordinate
translation. This single-source contract prevents duplicate path storage and
double drawing.

### Line Baselines

- Every line's `Baseline` is its y offset from the parent baseline.
- The first baseline is zero.
- Adjacent baselines are separated by the previous line's descent plus the
  current line's ascent.
- `OffsetX` is nonnegative and places the line in the frame.
- Overfull lines receive `OffsetX == 0` and positive overflow.

`Style.Align` positions each line in the available frame:

- `AlignStart`;
- `AlignCenter`;
- `AlignEnd`.

### What Can Wrap

Display wrapping is width-greedy. Legal cuts come from relation, binary, and
explicit break opportunities. Equal-width candidates use the earliest source
cut. Some constructs are deliberately unbreakable, including a formula with no
safe cut or source topology that cannot be partitioned without changing
meaning. Those cases produce one overfull line rather than an unsafe split.

`Box.Breaks` is populated for inline output. Constrained display output uses
break opportunities internally and does not expose them as parent
`Box.Breaks`.

### Alternative Text on Lines

Only the parent `Box.Alt` can be populated, and only when
`Style.GenerateAlt` is true. Line metadata has no independent alternative
text.

## Equation Tags

The current public contract exposes formula-level tags for TeX display math.
Typst input does not produce equation-tag metadata in this release.

An unconstrained tagged display box has:

- `Box.Tag != nil`;
- tag path and hit records in the suffix of `Box.Items` and `Box.Hits` selected
  by `Box.Tag.Items` and `Box.Tag.Hits`;
- explicit tag width, ascent, descent, italic correction, and parent-frame
  `X`/`Y`;
- no `Box.Lines`.

A constrained tagged box uses one complete line so the tag has unambiguous
ownership:

- `len(Box.Lines) == 1`;
- both `Box.Tag` and `Box.Lines[0].Tag` are present;
- the line is aligned in the reduced formula frame
  `MaxWidth - Tag.Width`;
- if the line is overfull, it begins at zero offset and the tag follows it;
- logical or ink excess contributes to `Box.Overflow`.

The tag body is already positioned in parent-frame coordinates. Draw the tag
range once as part of `Box.Items`; use `TagBox.Items` and `TagBox.Hits` only to
identify that subset. A `\tag*` value has no automatic parentheses.

Tag source metadata is available through `TagBox.Source` or the `Tags` field
of a formula returned by `Expr.RootSnapshot()`.
`Box.Tag.Manual` is true for the rendered formula-level tag. See the TeX
support page for the exact Phase 1 tag limitations.

## Source Hits

`Hit` links a source range to a placed rectangular area:

```go
type Hit struct {
	Source source.Range
	Area   geometry.Rect
	Depth  int
	Kind   HitKind
}
```

Source ranges are half-open UTF-8 byte offsets. They are not rune indexes and
not line/column pairs. Convert at the editor boundary if the UI uses runes.

Hit kinds identify semantic regions:

- atom;
- group;
- script;
- fraction numerator or denominator;
- radical;
- fence;
- table cell;
- row;
- other.

Rows and larger groups produce enclosing hits. Atoms and semantic children
usually produce deeper hits. Macro-derived nodes can map to the macro
invocation origin while retaining their expanded source spans.

### Point Hit Testing

`box.At(point)` returns one semantic hit containing the point:

1. deepest hit first;
2. then smallest area;
3. then earliest source range.

Hits use the same parent-frame coordinates as items. A line's hit range selects
the hits belonging to that line without changing their coordinates.

### Range Highlighting

`box.Covering(sourceRange)` returns placed areas intersecting the range. Areas
on the same line that overlap or touch are sorted and merged deterministically.
Use it for editor selection highlighting or source-linked overlays.

## Overflow and Bounds

`Box.Width`, `Ascent`, and `Descent` describe the nominal layout box. Ink can
extend outside it because of accents, italic overhangs, delimiters, tags,
negative spacing, or an overfull constrained line.

`Box.Overflow` is the nonnegative certified allowance around that nominal box.
Do not infer the drawable extent from the nominal fields alone.

```go
bounds := box.Bounds()
// Bounds is:
// X = -Overflow
// Y = -Ascent - Overflow
// Width = Width + 2*Overflow
// Height = Ascent + Descent + 2*Overflow
```

`Bounds()` is suitable for fitting the complete display list into a viewport
or allocating a temporary drawing surface. It is baseline-relative, not an
absolute screen rectangle.

For constrained display:

- each line records its own overflow;
- the parent overflow is the global allowance for the aligned shared list;
- a line's logical width excess can create overflow even when its ink fits.

## Drawing Checklist

- Apply the host's box translation once.
- Draw `Box.Items` once; line and tag ranges select subsets of that list.
- Fill glyph paths and rules with resolved paints.
- Treat `ItemGlyphRef` as either a cache substitute or metadata, not an extra
  shape.
- Use `Bounds()` for fitting.
- Use source hits in the same parent-frame coordinate space.
- Do not mutate a shared `Box`.

## Evidence

- Display types and validation: [`display/types.go`](../display/types.go),
  [`display/validate.go`](../display/validate.go)
- Bounds: [`display/bounds.go`](../display/bounds.go)
- Flattening and items: [`internal/layout/flatten.go`](../internal/layout/flatten.go)
- Constrained lines: [`internal/layout/lines.go`](../internal/layout/lines.go)
- Tags: [`internal/layout/tags.go`](../internal/layout/tags.go),
  [`internal/layout/tag_layout_test.go`](../internal/layout/tag_layout_test.go)
- Hits: [`internal/layout/hit_test.go`](../internal/layout/hit_test.go),
  [`display/types_test.go`](../display/types_test.go)
- Coordinate tests: [`internal/layout/coordinate_test.go`](../internal/layout/coordinate_test.go)
