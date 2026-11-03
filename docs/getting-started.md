# Getting Started

This guide uses only the root `untie` package. The root package re-exports the
public display, font, source, and geometry types needed by a host.

## Requirements

- A pure-Go build. No cgo, browser, DOM, GPU, or system-font lookup is used.
- A host-supplied OpenType font with MATH data for the primary math face.
- A positive layout size in host units. The bundled example uses DIP-like
  units.

Untie validates and copies the supplied font bytes during `LoadMathFont` or
`LoadFontSet`. The caller may release or reuse its byte slices after that call.

## Load, Parse, and Lay Out

```go
package main

import (
	"errors"
	"fmt"
	"os"

	"github.com/caelyreth/untie"
)

func renderFormula() (*untie.Box, []untie.Diagnostic, error) {
	fontBytes, err := os.ReadFile("NewCMMath-Book.otf")
	if err != nil {
		return nil, nil, err
	}

	fonts, err := untie.LoadMathFont(fontBytes)
	if err != nil {
		return nil, nil, err
	}

	engine, err := untie.NewEngine(fonts, untie.Limits{})
	if err != nil {
		return nil, nil, err
	}

	expr := untie.ParseTeX(
		`x^2 + \frac{1}{2}`,
		untie.Display,
		untie.ParseOptions{},
	)

	box, err := engine.Layout(expr, untie.Style{Size: 24})
	if err != nil {
		return nil, expr.Diagnostics(), err
	}
	return box, expr.Diagnostics(), nil
}

func classifyLayoutError(err error) string {
	switch {
	case errors.Is(err, untie.ErrMissingGlyph):
		return "the selected fonts do not cover the formula"
	case errors.Is(err, untie.ErrLimit):
		return "a configured resource limit was exceeded"
	case errors.Is(err, untie.ErrUnsupported):
		return "the parsed construct is not representable"
	case errors.Is(err, untie.ErrInsufficientStretch):
		return "the font cannot stretch a delimiter or accent enough"
	default:
		return fmt.Sprint(err)
	}
}
```

The order is intentional:

1. Load and validate fonts once.
2. Prepare an `Engine` once for repeated layouts with that font set.
3. Parse each source string into an immutable `Expr`.
4. Lay out the expression with a positive `Style.Size`.
5. Draw the returned `Box.Items`.

## High-Level Renderer

Applications that want one reusable parse-and-layout entry point can prepare
the engine internally with `NewRenderer`:

```go
renderer, err := untie.NewRenderer(fonts, untie.RendererOptions{
	Limits:         untie.Limits{},
	DefaultDialect: untie.TeX,
	DefaultStyle:   untie.Style{Size: 24},
	CacheEntries:   128,
})
if err != nil {
	return err
}

result, err := renderer.Render(ctx, source, untie.RenderRequest{
	Dialect: untie.Typst,
	Mode:    untie.Display,
	Style:   untie.Style{Size: 24, MaxWidth: 640},
})
if err != nil {
	// result may still contain recoverable parse diagnostics.
	return err
}
draw(result.Box)
```

A zero request dialect uses `DefaultDialect`; a zero dialect there means TeX.
A zero mode means `Inline`. A zero style uses `DefaultStyle`, whose zero size
is replaced with 16 host units. Request parse limits are intersected with
`RendererOptions.Limits`, so a renderer can enforce an application-wide safety
ceiling.

`CacheEntries` bounds a concurrency-safe LRU of immutable parsed expressions.
Zero or a negative value disables that cache. Display boxes are never cached:
every successful `Render` performs layout and returns an independent box.
When layout fails after parsing, `Render` returns a result containing the parse
diagnostics together with the layout error.

## Which Parse Function?

- `Parse(source, dialect, mode, options)` accepts a runtime `Dialect`.
- `ParseContext(ctx, source, dialect, mode, options)` adds boundary
  cancellation.
- `ParseTeX(source, mode, options)` is the same path with `TeX` selected.
- `ParseTypst(source, mode, options)` is the same path with `Typst` selected.

`Parse` does not return an ordinary Go error. Syntax and semantic problems are
reported in `expr.Diagnostics()`, and the expression still contains a validated,
drawable recovery tree. See [Parsing, diagnostics, and
recovery](parsing.md).

## Which Layout Function?

For an application, prefer an explicitly owned `Engine`:

```go
engine, err := untie.NewEngine(fonts, untie.Limits{})
if err != nil {
	return err
}
box, err := engine.LayoutContext(ctx, expr, untie.Style{Size: 20})
```

`Engine.Layout` uses a background context. `Engine.LayoutContext` accepts a
context for cancellation. The engine is safe for concurrent layouts, retains
its `FontSet`, and has no `Close` method.

The package-level convenience API keeps a bounded process-wide engine cache:

```go
box, err := untie.LayoutContext(ctx, expr, fonts, untie.Style{Size: 20})
```

It caches prepared engines, not layout results. Use `NewEngine` when the host
needs explicit lifecycle control. See [Concurrency, lifecycle, and
caching](concurrency.md).

## Draw the Result

The baseline origin is `(0, 0)`. Coordinates increase to the right and down.
For every item:

- `ItemGlyphPath`: fill `box.Paths[item.PathIndex]` with `item.Paint` after
  placing each template point as `item.At + point*item.Em`.
- `ItemRule`: fill `item.Rect` with `item.Paint`.
- `ItemGlyphRef`: optional host-cache metadata for the same placed glyph. It is
  not an additional visible shape. Ignore it when drawing the path list.

```go
func paint(painter Painter, box *untie.Box, colorFor func(untie.Paint) Color) {
	for _, item := range box.Items {
		switch item.Kind {
		case untie.ItemGlyphPath:
			template := box.Paths[item.PathIndex]
			painter.FillPath(placePath(template, item.At, item.Em), colorFor(item.Paint))
		case untie.ItemRule:
			painter.FillRect(item.Rect, colorFor(item.Paint))
		case untie.ItemGlyphRef:
			// Skip when ItemGlyphPath is already being drawn. A host may instead
			// resolve an exact ref through its own glyph cache.
		}
	}
}
```

`Painter`, `Color`, `FillPath`, and `placePath` above are placeholders for the host's
graphics API. The real MyGo implementation is in
[`examples/mygo/mathview.go`](../examples/mygo/mathview.go).

Do not draw `ItemGlyphPath` and its accompanying `ItemGlyphRef` as two shapes.
When `Style.GlyphRefs` is true, an exact glyph can produce both an outline path
and a semantic glyph reference.

## Fit and Center

Use `box.Bounds()` to fit the complete display list. The rectangle is relative
to the baseline origin and already includes the box's certified overflow.

```go
bounds := box.Bounds()
originX := viewport.X + (viewport.Width-bounds.Width)/2 - bounds.X
originY := viewport.Y + (viewport.Height-bounds.Height)/2 - bounds.Y
```

Items are already positioned relative to the baseline origin, so draw them
with the same translation. See [Rendering the display list](rendering.md) for
the full coordinate contract.

## Inline and Display

`untie.Display` selects display-style math and can produce constrained line
boxes when `Style.MaxWidth` is positive. `untie.Inline` selects text-style
math; it ignores `MaxWidth` and `Align` and does not return `Box.Lines`.

Untie is not a paragraph layout engine. An inline caller remains responsible
for placing the returned box in a text line. A display caller can use the
returned `MaxWidth` frame and line ranges for clipping or interaction. Draw
`Box.Items` once in both constrained and unconstrained display.

## Style Essentials

```go
style := untie.Style{
	Size:        24,          // required, finite and positive
	MaxWidth:    480,         // display-only wrapping frame
	Align:       untie.AlignCenter,
	ScriptLevel: 0,           // root style; 1=script, 2+=script-script
	Features:    []string{"dtls=1"},
	GlyphRefs:   false,
	GenerateAlt: false,       // opt in to Box.Alt accessibility generation
}
```

`Style.Features` is copied for the duration of the call. A caller may safely
reuse or modify its slice after `LayoutContext` returns.

`Expr.Alt()` is always available for parsed expressions. Set
`Style.GenerateAlt` when the returned `Box` should also carry accessibility
text under the engine's effective limits.

## Common First Integration Mistakes

- Drawing a path template without applying `item.At + point*item.Em`, or
  applying the host viewport transform twice. Templates are in local em units;
  `item.Bound` is already in box coordinates.
- Treating a parse diagnostic as a reason not to draw. Recoverable parse
  results are intended to remain drawable.
- Treating a nil/zero `Box` as a successful layout. Layout errors return no
  box.
- Re-translating a `LineBox.Items` range by `OffsetX`/`Baseline`; range items
  are already in parent-frame coordinates.
- Supplying a font without a valid OpenType MATH table as `FontSetConfig.Math`.

## Evidence

- Public facade: [`api.go`](../api.go), [`layout.go`](../layout.go),
  [`doc.go`](../doc.go)
- End-to-end examples: [`example_test.go`](../example_test.go)
- Lifecycle and style tests: [`layout_lifecycle_test.go`](../layout_lifecycle_test.go)
- Drawing example: [`examples/mygo/mathview.go`](../examples/mygo/mathview.go)
