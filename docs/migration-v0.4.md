# Migrating To 0.4

Untie 0.4 adds core output policy, shaped-text provenance, the complete MyGo
editor integration surface, editor geometry, structured accessibility
descriptions, Presentation MathML interchange, formula labels and references,
and high-value TeX layout constructs. The runtime version is
`0.4.0+layout.7`, and the process-wide Engine cache schema is `7`.

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

- `Alt.Description` as a versioned localization tree;
- `Alt.Speech` as the English label, with `Alt.Text` and source as fallbacks;
- exact TeX or Typst source as the value;
- a stable non-empty name for empty formulas.

`Options.LocalizeLabel` can turn the structured description into a host-owned
accessible label. `FormulaSemantics.Description` exposes the same tree to
applications that manage their own accessibility model. The existing strings
remain valid compatibility fallbacks.

The structured schema is now version `2`. It covers `Style.SetFontSize` and
`Style.FontSize`, `Overlay` alignment, `Raise` direction, and formula- or
row-scoped tags. Existing accessibility strings remain unchanged.

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
Found hit events also include `Caret` and `HasCaret`, so click-to-edit can enter
the editor at a proven source boundary without recomputing pointer affinity.

`FormulaClipboardData` and the context-menu option expose plain text and custom
TeX/Typst source MIME formats. The standalone `mathml` package now provides
explicit Presentation MathML conversion; it does not silently add a MathML
clipboard MIME type to the MyGo adapter.

## Caret Stops

`Box.CaretStops` stores source-ordered caret geometry for semantic leaves and
shaped text clusters. Use:

```go
carets := box.Carets()
caret, ok := box.CaretAt(sourceByteOffset)
```

`Carets` returns an independent sorted snapshot. `CaretAt` prefers an exact
proven source boundary and otherwise chooses the nearest boundary.
`Box.NearestCaret(point)` resolves a pointer position to the vertically nearest
line and then the horizontally nearest proven boundary.

The stops are in final box coordinates, including constrained display line
baselines and alignment offsets. Editors should not recover caret positions by
guessing from `Hit.Area`, because hits represent occupied regions rather than
positions between atoms.

## Inline Splitting

An untagged inline box can be divided at one of its own breaks:

```go
left, right, err := box.Split(box.Breaks[index])
if errors.Is(err, untie.ErrCannotSplit) {
	// Keep the unsplit inline object.
}
```

The split operation validates break ownership, copies and remaps path storage,
clips hit rectangles, keeps boundary caret stops on both sides, translates the
right box to a zero origin, and preserves item and hit `NodeID` values. `Alt`
and `FontUsage` are cleared because neither whole-formula semantic metadata can
be divided soundly.

Do not split constrained display boxes or tagged boxes. Those contracts retain
line and tag ownership through `Style.MaxWidth`.

## Stable Node Identity

`ir.Base` now carries `NodeID`, also exposed as `untie.NodeID`. Zero means
unassigned. Nonzero IDs must be unique within a formula.

Identity is explicit:

- `CloneFormula` preserves assigned IDs;
- validation rejects duplicate IDs on different nodes;
- canonical JSON includes assigned IDs;
- layout copies the source node ID into every emitted `Hit` and `Item`;
- a host that edits source and rebuilds a tree must preserve IDs on the
  semantically corresponding nodes.

Untie does not infer IDs from source spans, tree indexes, or pointers. It does
not provide `Expr.Edit` or incremental parsing in 0.4. Automatic cross-edit
identity requires a syntax-preserving edit model that does not exist yet.

## Text Cluster Direction

`font.ShapedCluster` now includes an `RTL` flag. It records whether shaping
produced that cluster in a right-to-left run. The internal layout text-source
port carries the same information; external code should use
`FontSet.ShapeText`, which populates it.

## Font-Size Declarations

The KaTeX-compatible declarations `\tiny`, `\sixptsize`, `\scriptsize`,
`\footnotesize`, `\small`, `\normalsize`, `\large`, `\Large`, `\LARGE`,
`\huge`, and `\Huge` now lower to neutral IR.

`ir.Style` gained `SetFontSize` and percentage-valued `FontSize`. The frontend
uses the exact KaTeX multipliers from 50% through 248.8%. Size declarations
scope to the remainder of their source group, nested declarations replace the
active absolute size, and scripts continue to apply MATH script scaling.
Text-mode declarations use the same IR and layout path.

## Overlap And Raise Primitives

Two neutral node families replace the old explicit-unsupported behavior:

- `ir.Overlay` places content at a zero-width insertion point with start,
  center, or end alignment;
- `ir.Raise` shifts content vertically by a signed source length without
  changing horizontal advance.

`\mathrlap`/`\rlap`, `\mathclap`/`\clap`, and `\mathllap`/`\llap` lower to
overlays. The non-`math` forms parse text-mode bodies. `\raisebox{length}{body}`
lowers to `Raise` with a text-mode hbox body. Overlay ink outside the zero-width
box participates in `Box.Overflow`.

`\vcenter`, `\phase`, `\enclose`, and other constructs without a lossless
neutral representation remain explicit unsupported diagnostics.

## Multiline Row Tags

Direct row-scope `\tag`, `\tag*`, `\nonumber`, and `\notag` now populate
`ir.EquationTag.Line`. `Box.Tag` remains the formula-level tag; `Box.Tags`
contains row-scoped `TagBox` values with nonnegative source-row indexes.

Formula-level and row-level tags are mutually exclusive. Row events must be
unique and source ordered, and each row-tag item/hit range occupies a contiguous
parent-frame suffix. Constrained display reserves one right-aligned tag column
while keeping the multiline formula block as one unbreakable line.

Nested environments, tags in ordinary groups or syntactic arguments, inline
tags, and automatic document counters remain outside the contract.

## Formula Labels and Host References

TeX `\label{name}` now produces immutable, unique formula metadata in
`ir.Formula.Labels`, also available through `Expr.Labels()`. Labels have no
visible output and may appear in groups or scripts.

TeX `\eqref{name}` resolves only from the parse-time
`ParseOptions.References` map:

```go
expr := untie.ParseTeX(`a=\eqref{eq:a}`, untie.Inline, untie.ParseOptions{
	References: map[string]string{"eq:a": "3"},
})
```

The map value is opaque display text; `\eqref` wraps it as `(<value>)` in a
text-font IR leaf. The map is copied before parsing. Formula labels never
populate or override the map, and there is no global state, automatic counter,
citation resolution, or cross-formula lookup.

Duplicate labels retain the first occurrence and report `duplicate-label`.
Missing entries report `unresolved-reference`; malformed or oversized values
report `invalid-value` or `limit-exceeded`. Each `\eqref` failure renders a
visible placeholder without discarding the surrounding formula.

Expression keys and the renderer's bounded expression cache include TeX
reference map contents. Typst ignores references, matching its existing macro
behavior.

## Display And Cache Compatibility

- `display.Box` gained `FontUsage`;
- `display.Alt` gained the versioned `Description` tree, with stable semantic
  kinds, roles, parts, flags, tags, and canonical JSON;
- `display.Item` gained `Snap`;
- `display.Box` gained `CaretStops`;
- `display.Box` gained `Tags` for source-row tags;
- `display.Item` and `display.Hit` gained `NodeID`;
- canonical IR gained `Style.FontSize`, `Overlay`, and `Raise`;
- canonical IR gained formula-scoped `EquationLabel` metadata;
- canonical box JSON may include `fontUsage`, `snap`, `carets`, `tags`, and
  node IDs;
- expression and renderer parse-cache identity includes TeX references;
- MyGo layout keys now include `ColorPolicy` and `PixelGrid`;
- `untie.Version()` is now `0.4.0+layout.7`;
- the Engine cache schema is now `7`.

Drop persisted prepared-engine, layout, or canonical-output caches from 0.3.
