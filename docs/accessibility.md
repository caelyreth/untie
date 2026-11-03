# Accessibility Text

`display.Alt` supplies two deterministic text forms derived from canonical IR:

```go
type Alt struct {
	Speech string
	Text   string
}
```

- `Text` is a compact linear notation suitable for copying, fallback display,
  or an accessible name.
- `Speech` is an English spoken interpretation suitable for a screen reader or
  a host's accessibility API.

Alternative text is not a transcription of source syntax. It is generated from
semantic nodes, so equivalent TeX and Typst formulas can produce the same
result. For example, both `x_i^2` inputs produce:

```text
Text:   x_i²
Speech: x sub i squared
```

`mathml.Marshal` uses the same source-independent generator for the root
MathML `alttext` attribute and exposes the compact form through
`data-untie-alt-text` when it differs. Import regenerates accessibility text
from the canonical IR instead of trusting arbitrary annotation markup.

## Three Ways to Obtain Alt

### `Expr.Alt()`

Returns the generated value and ignores generation errors:

```go
alt := expr.Alt()
```

This is the simplest path when a host wants best-effort accessible text even if
the expression contains unsupported recovery nodes.

### `Expr.AltContext(ctx)`

Returns both the value and the error:

```go
alt, err := expr.AltContext(ctx)
if errors.Is(err, untie.ErrAltUnsupportedNode) {
	// The returned Alt contains visible unsupported markers.
}
```

Use this when accessibility diagnostics must be logged or presented to the
application.

Errors include:

- `ErrAltInvalidRoot`;
- `ErrAltLimitExceeded`;
- `ErrAltUnsupportedNode`;
- `context.Canceled`;
- `context.DeadlineExceeded`;
- `ErrInvalidContext`;
- `ErrInvalidExpr`.

### `Box.Alt`

Set `Style.GenerateAlt` when a layout call should populate `Box.Alt`. The
default geometry-only path skips accessibility traversal. Requested Alt is
generated under the stricter intersection of engine and expression
text/traversal limits.

This means `Box.Alt` can differ from `Expr.Alt()` when the engine has a lower
limit. It may be a bounded, validated fallback even though geometry succeeded.
An accessibility failure does not invalidate correct geometry.

For parsed expressions, `Expr.Alt()` remains available without layout, and
`Expr.AltContext()` provides precise accessibility error reporting.

## Semantics, Not Geometry

The generator never reads:

- source text;
- glyph outlines;
- font metrics;
- placed display items;
- source spans.

It reads the neutral node tree plus the root mode. As a result:

- grouping syntax that does not change meaning does not change Alt;
- equivalent TeX and Typst trees can share output;
- layout scaling and font choice do not change Alt;
- dead source formatting does not leak into the accessible name.

## Semantic Roles

The IR's semantic roles improve the output:

- `FractionRoleBinomial` says "binomial coefficient" rather than reading a
  generic fraction;
- `ScriptRoleLimits` says "from" and "to" even when the source requested side
  placement;
- `TableRoleMatrix`, array, cases, and vector use distinct wording;
- `CancelRoleNegation` can speak a recognized relation as "not equal to"
  instead of "equals canceled";
- brace labels, overlays, roots, fences, and accents use their semantic form.

The exact vocabularies are deterministic and tested. They are English-centric;
the library does not currently expose localization hooks.

## Recoverable Errors Are Visible

An error node contributes a visible marker such as:

```text
[error: definitelyunknown]
```

A missing node can produce:

```text
[missing: reason]
```

An unsupported node contributes an `[unsupported ...]` marker and also causes
`ErrAltUnsupportedNode` from `AltContext`.

The goal is never to silently invent valid-sounding content for a recovery
node. A screen-reader user should learn that part of the formula failed.

## Bounded Fallback

The generator counts visited nodes and nesting depth and limits both output
fields by `MaxTextBytes`.

On a non-context failure after generation starts, it returns a visible,
validated fallback along with the error. On cancellation before generation, it
returns a zero Alt and the context error.

`Alt.Validate()` checks:

- both fields are valid UTF-8;
- neither field contains NUL.

Use only validated output for a host accessibility API.

## Tags

For a rendered formula-level tag, Alt appends text of the form:

```text
equation (1)
```

Suppression-only metadata does not add spoken text.

## Host Recommendations

- Use `Alt.Speech` for a screen-reader label or an accessible description.
- Keep `Alt.Text` available for copy, search, and visual fallback.
- Surface visible parse diagnostics separately; Alt is not an error channel.
- Do not derive accessibility text by stripping TeX or Typst source.
- Do not omit text for a formula only because it has parse diagnostics. The
  recovery tree still describes the valid portions and marks the failures.
- Call `box.Alt.Validate()` if a host accepts boxes from outside the normal
  engine path.

The MyGo adapter applies this policy to standalone formula elements by default:
`DescribeFormula` selects the label, and the element receives the exact source
as its accessibility value. MyGo v0.7.4 does not expose per-inline-object
accessibility metadata, so this wiring is limited to normal elements rather
than `markdown.InlineObject`.

## Example

```go
expr := untie.ParseTeX(
	`\sum_{i=1}^{n} i^2`,
	untie.Display,
	untie.ParseOptions{},
)

alt, err := expr.AltContext(ctx)
if err != nil && !errors.Is(err, untie.ErrAltUnsupportedNode) {
	return err
}

accessibilityLabel := alt.Speech
copyText := alt.Text
```
