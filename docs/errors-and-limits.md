# Errors and Resource Limits

Untie separates recoverable source diagnostics from fatal layout errors.
Understanding that split prevents a host from hiding valid previews or
misreporting a font/configuration problem as a syntax error.

## Error Classification

Layout functions return Go errors. Use `errors.Is`:

| Error | Meaning |
| --- | --- |
| `ErrInvalidInput` | Nil formula/root or other malformed layout input. |
| `ErrMissingGlyph` | The selected faces do not cover a required glyph. |
| `ErrUnsupported` | A recognized construct has no supported representation in this release. |
| `ErrInsufficientStretch` | The font cannot stretch a delimiter or accent enough. |
| `ErrLimit` | A resource limit was exceeded during layout or output construction. |
| `ErrInvalidStyle` | `Style` failed validation. |
| `ErrInvalidFonts` | Font preparation failed in `LoadFonts`, or `NewEngine` received an unusable font set. |
| `ErrInvalidEngine` | A nil or unusable engine was called. |
| `ErrInvalidContext` | A nil context was supplied. |
| `ErrInvalidExpr` | A nil expression was supplied. |
| `ErrAltInvalidRoot` | Alternative-text generation received malformed input. |
| `ErrAltLimitExceeded` | Alternative-text traversal/output exceeded a limit. |
| `ErrAltUnsupportedNode` | Alternative text cannot explain an IR node. |

Layout failures that carry one of the five layout sentinels are also returned
as `*untie.LayoutError`:

```go
type LayoutError struct {
	Kind  LayoutErrorKind
	Range source.Range
	Err   error
}
```

`Kind` (also available through `Code()`) is the stable machine-readable
classification:

| Kind | Sentinel |
| --- | --- |
| `LayoutErrorInvalidInput` | `ErrInvalidInput` |
| `LayoutErrorMissingGlyph` | `ErrMissingGlyph` |
| `LayoutErrorUnsupported` | `ErrUnsupported` |
| `LayoutErrorInsufficientStretch` | `ErrInsufficientStretch` |
| `LayoutErrorLimit` | `ErrLimit` |

`LayoutError.Unwrap` returns the underlying failure, so both `errors.Is` and
`errors.As` continue to work through host wrapping:

```go
box, err := engine.Layout(expr, style)
if err != nil {
	var layoutErr *untie.LayoutError
	if errors.As(err, &layoutErr) {
		switch layoutErr.Kind {
		case untie.LayoutErrorMissingGlyph:
			showFontProblem(layoutErr.Range)
		case untie.LayoutErrorLimit:
			showResourceProblem(layoutErr.Err)
		}
		if !layoutErr.Range.IsDetached() {
			underlineSource(layoutErr.Range)
		}
	}
	if errors.Is(err, untie.ErrMissingGlyph) {
		// Existing sentinel-based handling remains supported.
	}
}
```

When a node-local failure can be attributed to canonical IR, `Range` is that
node's `Origin` (falling back to `Span`). It is a half-open UTF-8 byte range in
the original source. A failure in final output validation or another operation
without a single owning node uses `source.Detached()`.

An error may satisfy more than one sentinel. Insufficient stretch is also
reported as unsupported for compatibility; `LayoutError.Kind` selects the more
specific `LayoutErrorInsufficientStretch`.

`context.Canceled` and `context.DeadlineExceeded` are returned directly when a
call observes cancellation. Cancellation is never classified as a
`LayoutError`, and a canceled `LayoutContext` does not return a partial box.

## Parse Diagnostics Versus Layout Errors

`Parse` returns an expression even for malformed source. Diagnostics describe
source problems:

```go
expr := untie.ParseTeX(`\frac{1}`, untie.Inline, untie.ParseOptions{})
if expr.HasErrors() {
	reportEditorState(expr.Diagnostics())
}
for _, d := range expr.Diagnostics() {
	if d.Severity == untie.SeverityError {
		reportEditorProblem(d)
	}
}
```

The recovery tree can still be laid out. A later layout error is a different
event, possibly unrelated to syntax:

- the font lacks a glyph;
- the font cannot build a required stretchy delimiter;
- an output or path-segment budget is too small;
- a parsed node shape is unsupported by the current layout phase.

Show both categories. Do not translate every diagnostic into a fatal layout
failure.

## Defaults

`Limits{}` is not unlimited. `WithDefaults` replaces zero and negative fields
with these values:

| Limit | Default | Primary enforcement |
| --- | ---: | --- |
| `MaxSourceBytes` | 1,048,576 | Parse boundary and IR validation |
| `MaxTokens` | 250,000 | TeX/Typst lexers and parsers |
| `MaxNesting` | 256 | Parser, IR traversal, and layout |
| `MaxNodes` | 100,000 | Parser boundary, IR validation, and layout |
| `MaxMacroExpansions` | 100,000 | TeX macro expander |
| `MaxExpandedTokens` | 1,000,000 | TeX macro expander |
| `MaxCallDepth` | 80 | Typst restricted evaluator and TeX call paths |
| `MaxTableCells` | 10,000 | TeX/Typst table parsing and layout checks |
| `MaxAssemblyRepeats` | 1,024 | Font assembly adapter |
| `MaxTextBytes` | 100,000 | IR validation, layout text, and Alt |
| `MaxOutputItems` | 250,000 | Display-list flattening |
| `MaxPathSegments` | 2,000,000 | Glyph/path flattening |
| `MaxDimension` | 1,099,511,627,776 | Geometry and length validation |

`MaxSourceBytes`, `MaxTextBytes`, and `MaxDimension` are byte/dimension values;
the others are counts.

## Which Limits Apply Where

### `ParseOptions.Limits`

These limits govern parsing and the parse-boundary validation of the returned
tree. The normalized values are retained by `Expr` and participate in
`Expr.Key()` when they can affect parsing or diagnostics. Layout-only limits
such as `MaxOutputItems` and `MaxPathSegments` do not change the parse key.

### `NewEngine(fonts, limits)`

These limits govern layout with that engine:

- structural node/nesting traversal;
- text and dimension checks;
- table-cell checks;
- output item and path-segment accounting;
- alternative text when `Style.GenerateAlt` requests it.

`Engine.Limits()` returns the normalized effective values.

### Package-Level `Layout`

The convenience API prepares or reuses an engine keyed by the expression's
effective parse limits. It still does not cache the resulting `Box`.

### Alternative Text

When `Style.GenerateAlt` is true, `Engine.LayoutContext` generates `Box.Alt`
under the per-field stricter intersection of engine and expression limits.
This means an engine can reduce an expression's text budget even when the
geometry itself is valid. `Expr.Alt()` remains available independently.

## Parser and Frontend Limits

The TeX frontend has hard checks for:

- lexer token count;
- macro expansion count;
- expanded token count;
- call depth;
- table-cell count.

The Typst restricted evaluator has analogous call, table, node, and nesting
checks. A limit diagnostic is recoverable where possible; it never asks a host
to parse an unbounded continuation.

## Layout and Output Limits

Layout counts structural work independently from source bytes. The key budgets
are:

- node visits;
- nesting depth;
- table cells;
- output items;
- path segments;
- text bytes;
- dimensions and lengths;
- MATH assembly repeats.

Output limits cover the generated display list, not the source size. A short
input can still exceed an output budget through repetition or large constructs.

`ItemGlyphRef` and `ItemGlyphPath` both consume `MaxOutputItems`. Setting
`Style.GlyphRefs` increases output count for exact outlined glyphs.

## Font and Layout Failures

`ErrMissingGlyph` is returned for an unmapped rune, missing glyph metrics, or a
missing outline. Add a capable fallback face; do not silently substitute a
different character.

`ErrInsufficientStretch` means the font has no variant or assembly that reaches
the required size. Check the primary MATH font and the requested construct.

`ErrUnsupported` is a capability boundary, not a corrupt input. Examples
include unsupported Typst code constructs and Phase 1 equation-tag shapes that
the presenter cannot place unambiguously.

## Context and Cancellation

All engine layout entry points check the context before work and at bounded
intervals. A canceled context returns the context error. A nil context is an
invalid host call and returns `ErrInvalidContext`.

```go
ctx, cancel := context.WithTimeout(parent, 100*time.Millisecond)
defer cancel()

box, err := engine.LayoutContext(ctx, expr, style)
if errors.Is(err, context.DeadlineExceeded) {
	// Abandon this preview; do not treat it as a source syntax failure.
}
```

## Robust Host Pattern

```go
expr := untie.Parse(src, dialect, mode, untie.ParseOptions{Limits: parseLimits})
diagnostics := expr.Diagnostics()

box, err := engine.LayoutContext(ctx, expr, style)
if err != nil {
	switch {
	case errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded):
		return err // stale request
	}
	var layoutErr *untie.LayoutError
	if !errors.As(err, &layoutErr) {
		showUnexpectedLayoutProblem(err)
		return err
	}
	switch layoutErr.Kind {
	case untie.LayoutErrorMissingGlyph:
		showFontProblem(layoutErr.Range)
	case untie.LayoutErrorLimit:
		showResourceProblem(layoutErr.Err)
	case untie.LayoutErrorUnsupported, untie.LayoutErrorInsufficientStretch:
		showCapabilityProblem(layoutErr.Err)
	default:
		showUnexpectedLayoutProblem(err)
	}
	return err
}

draw(diagnostics, box)
```

## Evidence

- Public errors: [`errors.go`](../errors.go), [`layout.go`](../layout.go)
- Sentinels used by layout: [`internal/layout/arithmetic.go`](../internal/layout/arithmetic.go)
- Internal source attribution:
  [`internal/layout/source_error.go`](../internal/layout/source_error.go)
- Defaults and layer notes: [`source/limits.go`](../source/limits.go)
- Limit propagation tests: [`api_test.go`](../api_test.go)
- Engine validation: [`internal/engine/engine.go`](../internal/engine/engine.go)
- Layout accounting: [`internal/layout/renderer.go`](../internal/layout/renderer.go),
  [`internal/layout/flatten.go`](../internal/layout/flatten.go)
