# Migrating to 0.2

Untie `0.2.0` is a pre-1.0 API cleanup. The math pipeline, canonical IR,
layout behavior, and display-box ownership contract are unchanged. This guide
maps the removed convenience surfaces to their supported replacements.

## Recommended Host Entry Point

`NewRenderer` is now the primary parse-and-layout integration:

```go
renderer, err := untie.NewRenderer(fonts, untie.RendererOptions{
	DefaultStyle: untie.Style{Size: 24},
	CacheEntries: 128,
})
if err != nil {
	return err
}

result, err := renderer.Render(ctx, source, untie.RenderRequest{
	Dialect: untie.Typst,
	Mode:    untie.Display,
})
```

The renderer prepares and retains one engine. Its optional cache stores bounded
immutable parsed expressions, never layout results. A successful call returns a
new `Box` owned by the caller.

`NewEngine` remains supported for hosts that need explicit engine lifecycle or
the `Engine.LayoutFormulaContext` advanced boundary.

## Font Construction

The root `FontData`, `Fonts`, and `LoadFonts` aliases and the `font.FontData`,
`font.Fonts`, and `font.Load` types and function are removed.

| Removed surface | Replacement |
| --- | --- |
| One math-font byte slice passed through `Fonts` | `LoadMathFont` with `WithFontID` and `WithFontIndex` options |
| Separate math, text, and fallback byte containers | `LoadFontSet(FontSetConfig{...})` with `font.FontFace` values |
| Reading files separately and assembling host-specific structures | `fontsource` helpers over an `io/fs.FS` |
| Inspecting the old mutable font container | `FontSet.Config()` for immutable copied configuration |

Example:

```go
fonts, err := untie.LoadFontSet(untie.FontSetConfig{
	Math: untie.FontFace{ID: "math", Bytes: mathBytes},
	Text: &untie.FontFace{ID: "text", Bytes: textBytes},
	Fallbacks: []untie.FontFace{
		{ID: "symbols", Bytes: symbolBytes, Index: 0},
	},
})
```

Face IDs may be omitted. Untie derives a stable ID from the role, selected
index, metadata, and bytes. Font bytes are copied during loading, so the caller
may reuse its input slices after the call.

## Drawing and Fitting

The display-box contract is unchanged. The new `render` package removes common
per-host path placement code:

| Previous host responsibility | Current helper |
| --- | --- |
| Iterate items and skip glyph-reference metadata | `render.Walk` |
| Materialize `At + point*Em` into a new path | `render.ResolvePath` |
| Place paths while drawing without allocation | Resolve each borrowed `render.Command` segment at paint time |
| Compute a viewport transform from `Box.Bounds()` | `render.Fit` |
| Apply translation and scale to points or rectangles | `render.Transform.Apply` and `Transform.ApplyRect` |

`render.Walk` preserves visible item order. For path commands it borrows the
box's path template; the callback must not mutate or retain that template.
Call `render.ResolvePath` only when a host API needs an independent
`geometry.Path`.

## SVG and PNG Output

Hosts that cannot draw the display list directly can use the optional adapters:

- `encode/svg.Marshal` and `encode/svg.Write`;
- `encode/raster.Encode` and `encode/raster.EncodePNG`.

Both adapters consume the same public display list through `render.Walk` and
include the box's certified overflow. SVG is an optional export format, not a
stable wire protocol. PNG output is deterministic for the same box and options.

## MyGo Integration

MyGo integration remains in the separately versioned nested module:

```go
import "github.com/caelyreth/untie/contrib/mygo"
```

The package itself is named `untiemygo`, so an explicit import alias is not
required. Importing the root `untie` module never adds a MyGo dependency.

`untiemygo.New` and the `(*untiemygo.Renderer).Render`,
`(*untiemygo.Renderer).InlineMath`, `(*untiemygo.Renderer).DisplayMath`,
`(*untiemygo.Renderer).BindMarkdown`, `(*untiemygo.Renderer).Draw`, and
`(*untiemygo.Renderer).CacheStats` methods replace application-owned path
fitting, paint resolution, and MyGo drawing code. The adapter may cache
immutable boxes; callers must not mutate a box obtained from that cache.

## Ownership and Caching

- `FontSet` and `Engine` are immutable or concurrency-safe reusable resources.
- `Renderer` is safe for concurrent `Render` calls.
- Every successful `(*untie.Renderer).Render` or `(*untie.Engine).Layout` call
  returns independent mutable `Box` storage.
- The root renderer and package-level engine cache never share returned boxes.
- The MyGo adapter is the documented exception: cache hits return the same
  immutable box, which callers must not mutate.
- Path templates referenced by `render.Walk` are box-owned, callback-scoped
  borrows.
- Persisted caches should include `untie.Version()` and the relevant font or
  expression identity.

See [Concurrency, lifecycle, and caching](concurrency.md) and
[API stability tiers](stability.md) for the complete contract.
