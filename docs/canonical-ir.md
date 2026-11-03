# Canonical IR and `Engine.LayoutFormulaContext`

The canonical IR is the shared language between the TeX and Typst frontends
and the layout engine. Most hosts should parse an expression and pass the
`*Expr` directly to layout. This page is for hosts that inspect, store, or
produce validated canonical IR.

## Formula and Tags

```go
type Formula struct {
	Root Node
	Tags []EquationTag
}
```

`Root` is one sealed `ir.Node`. `Tags` is formula metadata, not an alternate
root or a hidden table:

```go
type EquationTag struct {
	Body       Node
	Span       source.Range
	Origin     source.Range
	Manual     bool
	Suppressed bool
	Starred    bool
	Line       int
}
```

For the current contract:

- rendered tags are manual;
- `Line` is `ir.FormulaTagLine` for a formula-level tag or a zero-based source
  row for a multiline tag;
- `Starred` removes the automatic parentheses;
- suppression events have `Suppressed == true` and a nil body;
- a rendered tag has a non-nil body and `Manual == true`;
- formula-level and row-level events cannot be mixed;
- each line has at most one event and row events are strictly increasing.

The TeX frontend exposes formula tags and direct tags in supported multiline
environments. Typst tags, nested-environment tags, and automatic counters are
outside the metadata contract.

## The Node Interface

```go
type Node interface {
	NodeBase() *Base
	Kind() NodeKind
	node()
}
```

The unexported method seals the interface to the `ir` package. The concrete
node pointer types are public and constructible, but all public fields are
mutable.

Treat a formula as immutable once it is validated or shared.
`Expr.RootSnapshot()` returns a deep snapshot precisely so a host can inspect
or modify a copy without changing the expression.

## Shared Base

Every concrete node embeds `Base`:

```go
type Base struct {
	Span   source.Range
	Origin source.Range

	Class   MathClass
	Size    MathSize
	Cramped bool
	Variant Variant
	Paint   Paint

	Limits        LimitsPlacement
	LeftSpacing   Length
	RightSpacing  Length
	IgnoreAsSpace bool
}
```

`Span` is the syntactic source range represented by the node. `Origin` records
the source site that caused the node when it differs, most notably for a macro
expansion. A detached origin permits hit generation to fall back to `Span`.

`Class` drives automatic math spacing and is also used by scripts and layout
policy. `Size` records an explicit math style, while `Cramped`, `Variant`, and
`Paint` describe semantic presentation.

`LeftSpacing` and `RightSpacing` carry explicit dialect-independent spacing
overrides. `IgnoreAsSpace` preserves source atoms that should not create an
ordinary inter-atom gap.

## Node Families

| Node             | Purpose                                                                       |
| ---------------- | ----------------------------------------------------------------------------- |
| `Row`            | Horizontal sequence and the common container.                                 |
| `Glyph`          | One semantic symbol or grapheme cluster.                                      |
| `Text`           | Shaped text-mode run, with optional font role, tracking, and source clusters. |
| `Operator`       | Named mathematical operator.                                                  |
| `Space`          | Explicit, interword, or ignored space.                                        |
| `Fraction`       | Vertical, horizontal, skewed, or barless fraction; also binomial/stack roles. |
| `Radical`        | Square or indexed root.                                                       |
| `Scripts`        | Base plus top/bottom/corner attachments with a semantic role.                 |
| `Fenced`         | Optional opening/closing delimiters around a body.                            |
| `SizedDelimiter` | One delimiter with an explicit sizing policy.                                 |
| `Accent`         | Above/below accent with stretch and natural-fit controls.                     |
| `Line`           | Overline or underline.                                                        |
| `Cancel`         | Strike or negation overlay.                                                   |
| `Table`          | Matrix, array, cases, vector, or aligned grid.                                |
| `Multiline`      | Source-ordered rows without a table grid.                                     |
| `Style`          | Explicit math size, font scale, and cramping wrapper.                         |
| `Overlay`        | Zero-width start/center/end-aligned content.                                  |
| `Raise`          | Signed vertical shift that preserves horizontal advance.                      |
| `Color`          | Paint wrapper.                                                                |
| `Box`            | Padding, fill, border, and radius decoration.                                 |
| `Phantom`        | Hidden width/height/depth while preserving selected extents.                  |
| `ErrorNode`      | Visible recoverable source fragment.                                          |
| `Placeholder`    | Visible missing-content marker.                                               |
| `Break`          | Explicit legal line-break opportunity.                                        |
| `AlignMarker`    | Alignment point inside multiline/table content.                               |

The frontends lower dialect syntax into these families. TeX `pmatrix` and
Typst `mat` can both become a `Table`; TeX `aligned` and Typst alignment rows
can both become `Multiline`; a Typst `frac(..., style: "skewed")` and TeX
`\genfrac` can share `Fraction`.

`Style.SetFontSize` and `Style.FontSize` carry an explicit percentage of the
root host font size without embedding a dialect-specific size tier. Script
style transitions still come from layout, not the frontend.

`Overlay` changes horizontal placement while preserving the body's vertical
metrics. `Raise` changes vertical placement while preserving horizontal
advance. Both retain child source hits and normal visible recovery.

### Text Source Clusters

`Text` may carry a parser-authored source map:

```go
type TextClusterSource struct {
	TextStart int
	TextEnd   int
	Source    source.Range
}
```

Each entry maps a half-open UTF-8 byte range in `Text` to the absolute source
range that produced it. A non-empty map is a complete, ordered partition of
`Text`; its source ranges are also non-overlapping and ordered in logical text
order. Layout may later place those clusters in a different visual order for
bidirectional text.

The map is empty when the frontend cannot prove those invariants. Manual or
synthetic IR built without a map remains valid and uses the node-level
`Text.Origin`/`Span` hit fallback. Frontends must derive maps from lexer or
parser evidence, not by guessing offsets from braces or quotes.

## Semantic Roles

Some roles avoid forcing layout policy into a frontend:

- `FractionRole`: fraction, binomial, stacked;
- `TableRole`: matrix, array, cases, vector;
- `ScriptRole`: ordinary, limits, overlay, brace label;
- `CancelRole`: strike, negation;
- `DelimiterSizing`: content, relative, absolute;
- `LimitsPlacement`: automatic, always, never, display-only.

Accessibility and future layout policies can use these roles without
re-parsing source. Visual placement can differ from semantic role; for example,
`\nolimits` and Typst `scripts(...)` can place operator bounds beside the base
while preserving "limits" semantics.

## Traversal and Validation

Use the graph helpers rather than ad hoc pointer recursion:

```go
if err := ir.ValidateSource(formula, source, limits); err != nil {
	return err
}

err := ir.WalkFormula(formula, limits, func(node ir.Node) bool {
	// Inspect node; return false to skip its children.
	return true
})
```

- `ir.Children(node)` returns the direct children in deterministic semantic
  order and clones returned slices.
- `ir.VisitChildren(node, fn)` exposes the same order without allocating a
  child slice; the callback must not retain or mutate the tree.
- `ir.Walk` is the best-effort convenience walker.
- `ir.WalkWithLimits` returns cycle and traversal errors.
- `ir.WalkFormula` traverses the root and every tag body under one shared
  node/nesting budget.
- `ir.CloneFormula` validates and returns a deep copy.

Validation catches cycles, typed nils, node/kind mismatch, invalid ranges,
invalid roles and lengths, oversized text, excessive depth, and resource-limit
violations. `ValidateSource` additionally proves every attached range lies
inside the provided source string.

`ir.ValidateWithStats` and `ir.ValidateSourceWithStats` return an immutable
`ir.Stats` resource envelope alongside the validation result. The envelope
records node/tag counts, maximum depth and text size, the largest dimension
value, table-cell count, and a conservative glyph-materialization hint.
`Stats.WithinLimits` can prove that a formula satisfies a stricter limit set
without another traversal; `false` means the caller must run the diagnostic
validation path. Stats describe the validated tree and become stale if the
formula is mutated.

## Canonical JSON

`ir.MarshalCanonical` and `display.MarshalCanonical` produce deterministic JSON
for diagnostics and local inspection. They are intentionally not stable
cross-language protocols. Do not persist them as an external interchange
format without versioning your own adapter.

## Parsing Without Re-Parsing

`Expr.RootSnapshot()` returns a validated deep snapshot. This is the normal
route to canonical IR:

```go
expr := untie.ParseTeX(source, untie.Display, untie.ParseOptions{})
formula, err := expr.RootSnapshot()
if err != nil {
	return err
}
```

The snapshot is independent of the expression and may be cached by the host.

## Laying Out Canonical IR

Use:

```go
box, err := engine.LayoutFormulaContext(
	ctx,
	formula,
	untie.Display,
	untie.Style{Size: 24, MaxWidth: 480, GenerateAlt: true},
)
```

This API:

- requires a non-nil engine, context, formula, and root;
- validates the canonical tree;
- applies the engine's configured resource limits;
- performs the same font, layout, tag, alternative-text, and box-validation
  work as expression layout;
- populates `Box.Alt` when `Style.GenerateAlt` is true;
- returns no box on layout failure.

The formula must remain immutable for the duration of the call and may be
shared after it returns. Unlike parser-only structural validation, this entry
point uses the engine limits and therefore can reject a formula that a parser
would have produced under larger limits.

`LayoutFormulaContext` validates ranges structurally. It does not have the
original source string, so call `ir.ValidateSource` first when proving that
ranges stay within a specific source is required.

## Parsed Expression Versus Formula

For a normal parsed expression, prefer:

```go
box, err := engine.LayoutContext(ctx, expr, style)
```

The expression path:

- reuses the parser-validated frozen tree;
- intersects expression and engine limits for generated alternative text;
- retains expression-specific context and metadata;
- avoids requiring the host to manage a separate formula snapshot.

Use `LayoutFormulaContext` when canonical IR is the actual stored boundary,
for example a document cache shared across syntax frontends.

## Constructing IR Manually

The constructors create deterministic defaults:

```go
glyph := ir.NewGlyph(source.NewRange(0, 1), "x")
row := ir.NewRow(source.NewRange(0, 1), glyph)
formula := &ir.Formula{Root: row}

if err := ir.ValidateSource(formula, "x", untie.Limits{}); err != nil {
	return err
}
```

Manual construction is an advanced API. You are responsible for:

- source-range ownership;
- semantic classes and roles;
- valid length units and dimensions;
- valid table/multiline structure;
- tag metadata consistency;
- preserving immutability after sharing.

Do not create unvalidated nodes and pass them directly to layout in a
production host.
