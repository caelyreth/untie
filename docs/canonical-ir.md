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

For the current Phase 1 contract:

- rendered tags are manual;
- `Line` is `ir.FormulaTagLine`;
- `Starred` removes the automatic parentheses;
- suppression events have `Suppressed == true` and a nil body;
- a rendered tag has a non-nil body and `Manual == true`;
- multiple manual tags for one line are invalid.

The TeX frontend exposes top-level display tags. Typst and nested/multiline
tagging are outside the Phase 1 metadata contract.

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

Treat a formula as immutable once it is validated or shared. `Expr.Root()`
returns a deep snapshot precisely so a host can inspect or modify a copy without
changing the expression.

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

| Node | Purpose |
| --- | --- |
| `Row` | Horizontal sequence and the common container. |
| `Glyph` | One semantic symbol or grapheme cluster. |
| `Text` | Shaped text-mode run, with optional font role and tracking. |
| `Operator` | Named mathematical operator. |
| `Space` | Explicit, interword, or ignored space. |
| `Fraction` | Vertical, horizontal, skewed, or barless fraction; also binomial/stack roles. |
| `Radical` | Square or indexed root. |
| `Scripts` | Base plus top/bottom/corner attachments with a semantic role. |
| `Fenced` | Optional opening/closing delimiters around a body. |
| `SizedDelimiter` | One delimiter with an explicit sizing policy. |
| `Accent` | Above/below accent with stretch and natural-fit controls. |
| `Line` | Overline or underline. |
| `Cancel` | Strike or negation overlay. |
| `Table` | Matrix, array, cases, vector, or aligned grid. |
| `Multiline` | Source-ordered rows without a table grid. |
| `Style` | Explicit math size/cramping wrapper. |
| `Color` | Paint wrapper. |
| `Box` | Padding, fill, border, and radius decoration. |
| `Phantom` | Hidden width/height/depth while preserving selected extents. |
| `ErrorNode` | Visible recoverable source fragment. |
| `Placeholder` | Visible missing-content marker. |
| `Break` | Explicit legal line-break opportunity. |
| `AlignMarker` | Alignment point inside multiline/table content. |

The frontends lower dialect syntax into these families. TeX `pmatrix` and
Typst `mat` can both become a `Table`; TeX `aligned` and Typst alignment rows
can both become `Multiline`; a Typst `frac(..., style: "skewed")` and TeX
`\genfrac` can share `Fraction`.

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
- `ir.Walk` is the best-effort convenience walker.
- `ir.WalkWithLimits` returns cycle and traversal errors.
- `ir.WalkFormula` traverses the root and every tag body under one shared
  node/nesting budget.
- `ir.CloneFormula` validates and returns a deep copy.

Validation catches cycles, typed nils, node/kind mismatch, invalid ranges,
invalid roles and lengths, oversized text, excessive depth, and resource-limit
violations. `ValidateSource` additionally proves every attached range lies
inside the provided source string.

## Canonical JSON

`ir.MarshalCanonical` and `display.MarshalCanonical` produce deterministic JSON
for golden tests and diagnostics. They are intentionally not stable
cross-language protocols. Do not persist them as an external interchange
format without versioning your own adapter.

## Parsing Without Re-Parsing

`Expr.Root()` returns a validated deep snapshot. This is the normal route to
canonical IR:

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
	untie.Style{Size: 24, MaxWidth: 480},
)
```

This API:

- requires a non-nil engine, context, formula, and root;
- validates the canonical tree;
- applies the engine's configured resource limits;
- performs the same font, layout, tag, alternative-text, and box-validation
  work as expression layout;
- populates `Box.Alt`;
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

## Evidence

- Base and node contract: [`ir/base.go`](../ir/base.go)
- Concrete nodes and tags: [`ir/nodes.go`](../ir/nodes.go)
- Validation: [`ir/validate.go`](../ir/validate.go)
- Traversal and cloning: [`ir/walk.go`](../ir/walk.go),
  [`ir/clone.go`](../ir/clone.go)
- Canonical serialization: [`ir/canonical.go`](../ir/canonical.go)
- Formula layout facade: [`layout.go`](../layout.go)
- Canonical IR layout test: [`api_test.go`](../api_test.go)
- Tag contracts: [`ir/tags_test.go`](../ir/tags_test.go),
  [`internal/tex/tags_test.go`](../internal/tex/tags_test.go)

