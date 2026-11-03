# Parsing, Diagnostics, and Recovery

Parsing turns source text into canonical IR. It deliberately does not use a Go
`error` return for source mistakes: a document editor must still receive a
valid tree to highlight and display while the user is typing.

## Public Entry Points

```go
expr := untie.Parse(source, untie.TeX, untie.Inline, untie.ParseOptions{})
expr, err := untie.ParseContext(ctx, source, untie.TeX, untie.Inline, untie.ParseOptions{})
expr := untie.ParseTeX(source, untie.Display, untie.ParseOptions{})
expr := untie.ParseTypst(source, untie.Inline, untie.ParseOptions{})
```

All of these share the same recovery and validation boundary. `ParseTeX` and
`ParseTypst` are convenience wrappers. `ParseContext` checks cancellation
before parser dispatch and again before returning; parser work that has already
started is bounded by `ParseOptions.Limits` rather than interrupted at
arbitrary points. A canceled call returns no expression and `ctx.Err()`.

`ParseOptions` contains:

```go
type ParseOptions struct {
	Macros     map[string]untie.Macro
	References map[string]string
	Limits     untie.Limits
}
```

The TeX frontend consumes macros and references. The Typst frontend ignores
both maps. `References` values are opaque, host-supplied reference text: they
are not parsed as TeX or Typst and cannot introduce commands, macros, or labels.

## What Parsing Produces

The returned `*Expr` contains:

- source and dialect;
- the requested root mode;
- a validated canonical `ir.Formula`;
- sorted diagnostics;
- effective limits;
- a convenience cache key.

`Parse` is the boundary that checks the frontend's output with
`ir.ValidateSource`. A frontend failure becomes a visible diagnostic and a
drawable fallback rather than exposing an invalid tree.

## Diagnostics

```go
type Diagnostic struct {
	Range    untie.Range
	Severity untie.Severity
	Code     untie.Code
	Message  string
	Args     []string
	Hints    []string
}
```

`Range` is a half-open UTF-8 byte range. `Diagnostic.Error()` returns
`code: message`.

Severities are:

- `SeverityError`;
- `SeverityWarning`;
- `SeverityInfo`.

Codes currently include:

| Code                  | Meaning                                                |
| --------------------- | ------------------------------------------------------ |
| `syntax`              | General syntax error.                                  |
| `unknown-command`     | Unrecognized TeX control sequence.                     |
| `unknown-symbol`      | A referenced symbol cannot be resolved.                |
| `missing-argument`    | A construct requires an argument that is absent.       |
| `unclosed-group`      | A group, fence, or environment is not closed.          |
| `unexpected-token`    | A token is not legal at that position.                 |
| `unsupported`         | Recognized syntax is outside the current renderer.     |
| `invalid-value`       | An argument is present but invalid.                    |
| `invalid-environment` | Unknown or mismatched environment.                     |
| `unknown-variable`    | Typst name is not defined in the restricted evaluator. |
| `duplicate-label`     | A formula-scoped label name was repeated.              |
| `unresolved-reference` | No host reference entry matched a `\eqref` name.      |
| `missing-glyph`       | A parse-stage glyph requirement is absent.             |
| `text-unsupported`    | Text syntax is outside the supported text subset.      |
| `limit-exceeded`      | A configured resource limit was reached.               |
| `canceled`            | Work was canceled.                                     |
| `internal`            | A parser invariant or implementation failure.          |
| `deprecated`          | Reserved for a recognized deprecated spelling.         |

The project treats existing code meanings as stable identifiers, while allowing
new codes in future releases.

### Normalization

`Expr.Diagnostics()` returns a defensive copy. The result is normalized:

- detached ranges are retained;
- invalid or out-of-source ranges are removed;
- ordering is by start, end, severity, code, then message.

The normalization is deterministic. Do not use slice position as a substitute
for the range and code when correlating diagnostics to a document.

`Expr.HasErrors()` is a convenience check for at least one error-severity
diagnostic. The root package also exports the stable diagnostic code
vocabulary as `untie.Code` and `untie.Code*` constants, so ordinary hosts do
not need to import `source` just to classify a diagnostic.

## Recoverable Fallback

Malformed input produces diagnostics plus a valid, drawable tree. Examples:

- missing `\frac` arguments;
- an unclosed Typst `sqrt(`;
- an unmatched TeX fence;
- an unknown variable in Typst;
- exceeding a parser resource limit.

The parse boundary also converts these non-source failures into the same
contract:

- source larger than `MaxSourceBytes`;
- an unknown `Dialect` value;
- a recovered frontend panic.

A fallback is not a claim that the source is correct. The caller should display
the diagnostics and inspect `Expr.Alt()` or request `Box.Alt` through
`Style.GenerateAlt` for accessible error markers.

The fallback text is bounded by `MaxTextBytes`. Under an exact one-node budget,
the parser emits a one-node placeholder/error rather than constructing an
invalid two-node tree.

## Host TeX Macros

`ParseOptions.Macros` supplies simple host-defined macros:

```go
expr := untie.ParseTeX(`\norm{x}`, untie.Inline, untie.ParseOptions{
	Macros: map[string]untie.Macro{
		"norm": {Body: `\left\lVert #1 \right\rVert`, Args: 1},
	},
})
```

The body uses `#1`, `#2`, and so on. Use zero through nine undelimited
arguments. The macro map is not mutated, and the expression's key includes a
deterministic serialization of its names, bodies, and arities.

TeX source definitions such as `\def`, `\newcommand`, `\renewcommand`, and
`\providecommand` are also handled by the frontend's bounded macro expander.
This is not a complete TeX execution engine: document packages, conditionals,
counters, file inclusion, and arbitrary external macros are outside the
frontend.

When a macro body produces nodes, source ranges preserve both the expanded
body and the invocation origin where they can be mapped. Layout hits can
therefore point back to the host source invocation.

## Links Are Text-Only

The neutral IR has no clickable destination node. Both frontends therefore
preserve a link's visible body and discard its destination:

- TeX `\href{...}{body}` and `\url{...}` render visible text/body and report an
  `unsupported` warning after the arguments validate.
- Typst `#link("...", $body$)` renders the visible body and reports an
  `unsupported` warning. An omitted Typst body renders the destination text.

Malformed link arguments produce their own `missing-argument`, `invalid-value`,
or `syntax` diagnostics. Formula labels and references use the explicit
contract below; document-level link resolution remains outside this API.

Do not assume a link warning means the body was discarded. It was not.

## Formula Labels and References

TeX `\label{name}` is formula metadata, not visible IR. `Expr.Labels()` and
`ir.Formula.Labels` expose the decoded, unique, source-ordered names:

```go
type EquationLabel struct {
	Name   string
	Span   source.Range
	Origin source.Range
}
```

The first occurrence of a duplicate name is retained and later occurrences
emit `duplicate-label`. Labels inside groups or scripts remain formula-scoped.
For macro-expanded labels, `Origin` and usually `Span` point to the invocation
site rather than joining definition and call-site bytes.

TeX `\eqref{name}` resolves only from the `ParseOptions.References` snapshot:

```go
expr := untie.ParseTeX(`a=\eqref{eq:a}`, untie.Inline, untie.ParseOptions{
	References: map[string]string{"eq:a": "3"},
})
```

The resolved output is the text-font leaf `(3)`. The map value must already be
display-ready text without parentheses; Untie does not parse it as math.
Missing entries emit `unresolved-reference`, while empty, invalid UTF-8,
control-containing, or oversized values emit `invalid-value` or
`limit-exceeded`. Those failures render a visible placeholder and preserve the
surrounding formula.

Resolution precedence is deliberately small:

1. Label names are decoded from TeX escapes such as `\_` and `\%`.
2. Map lookup is an exact match against that decoded name.
3. A formula's own `Labels` never populate or override `References`.
4. There is no global registry, automatic numbering, citation lookup, or
   lookup across another formula or parse call.

Both maps are copied when parsing begins. Mutating the caller's map afterward
cannot change the expression, its metadata, its key, or a cached render.

## Chemistry Is A Bounded Frontend

The TeX frontend recognizes bounded `\ce{...}` and `\pu{...}` syntax and
lowers it to ordinary neutral IR. Chemistry is therefore subject to the same
source, diagnostic, canonical-validation, layout, and accessibility rules as
other math. See [Bounded TeX chemistry](chemistry.md) before relying on a
specialist mhchem construct.

## Parser Limits

`ParseOptions.Limits` is normalized with `Limits.WithDefaults`. Zero and
negative values do not mean "unlimited"; they select the documented default.

The parser enforces source, token, macro-expansion, expanded-token, call-depth,
and table-cell limits. The parse boundary also enforces source, node, nesting,
text, and dimension limits while validating the produced tree. See
[Errors and resource limits](errors-and-limits.md).

## Recommended Editor Flow

```go
result, err := renderer.Render(ctx, source, untie.RenderRequest{
	Dialect: dialect,
	Mode:    untie.Display,
	Style:   untie.Style{Size: 24},
	Parse:   untie.ParseOptions{Limits: parseLimits},
})

if result != nil {
	for _, diagnostic := range result.Diagnostics {
		span := diagnostic.Range
		// Map UTF-8 byte offsets to the editor's native coordinates.
		showDiagnostic(span, diagnostic.Severity, diagnostic.Code, diagnostic.Message)
	}
}

if err != nil {
	// Keep the parse diagnostics visible and show a layout-level message.
	return err
}
draw(result.Box)
```

A recoverable parse diagnostic should not by itself disable the preview.
Layout failure should be shown separately because it can be caused by fonts,
limits, unsupported layout shapes, or an invalid host call.
