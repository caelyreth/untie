# MyGo Integration

The [`untiemygo`](../contrib/mygo/README.md) adapter renders Untie formulas as
native MyGo elements, baseline-aware Markdown math, or painter commands.

```go
fonts, err := untie.LoadMathFont(fontBytes)
if err != nil {
	return err
}
math, err := untiemygo.New(fonts, untiemygo.Options{
	Dialect:      untie.TeX,
	CacheEntries: 128,
	Draw: untiemygo.DrawOptions{
		Padding: 8,
		Align:   render.AlignCenter,
	},
})
if err != nil {
	return err
}
```

`Dialect` selects the input language. `BaseSize` controls fallback inline size,
and `DisplaySize` controls display math. If either size is omitted, suitable
defaults are used.

## Direct Elements

Use `Element` when a formula should be a normal MyGo drawable:

```go
ui.Column(c).Padding(16).Children(func() {
	math.Element(c, `\sum_{k=1}^{n} k^2`, untie.Display).FillWidth()
})
```

The returned `ui.Element` is ready to place, size, align, or draw like any
other MyGo element. It starts at the formula's natural size and scales down
when the available box is smaller. Set `DrawOptions.AllowUpscale` if a host
wants formulas to grow into larger spaces.

`untie.Inline` selects inline math size. A standalone inline element still
participates in normal MyGo box layout; use `InlineMath` when baseline
placement inside text is required.

Missing glyphs recover visibly by default: the element draws an error-colored
placeholder while preserving the rest of the formula. If parsing or
non-recoverable layout fails, `Element` returns selectable text in the theme's
danger color. Call `Renderer.Render` directly when the application also needs
the structured diagnostics.

## Markdown

`BindMarkdown` installs both Markdown math callbacks and enables single-dollar
inline math:

```go
var options markdown.Options
math.BindMarkdown(&options)
markdown.View(c, source, &options)
```

It supports `$...$`, `$$...$$`, `\(...\)`, and `\[...\]`, while retaining
existing non-math callbacks and parser extensions.

`InlineMath` maps the complete `Box.Bounds`, including certified overflow, into
MyGo's baseline-relative `Width`, `Height`, and `Depth`. `DisplayMath` builds a
full-width display element that scales down when the available width is
insufficient.

## Existing Boxes

`Renderer.Render` is the lower-level adapter entry point. It returns an
`*untie.RenderResult` containing the box, diagnostics, and alternative text
when requested:

```go
result, err := math.Render(ctx, source, untie.Display, untie.Style{})
if err != nil {
	return err
}
math.Draw(painter, result.Box, viewport)
```

`Draw` fits `Box.Bounds` into the viewport, applies configured padding and
alignment, and resolves semantic paints through the active theme.

## Cache And Ownership

The adapter caches laid-out boxes by source, dialect, mode, and full layout
style. The cache is a concurrency-safe LRU:

- `CacheEntries: 0` uses 128 entries;
- a negative value disables caching;
- `MaxCacheEntries` is 4096.

`CacheStats` reports entries, capacity, hits, misses, and evictions. Returned
and cached boxes are immutable. A caller may read a box returned by
`Renderer.Render`, but must not modify it or its slices while it is shared by
cache users, concurrent workers, or draw callbacks.

Paint roles remain semantic in the cache, so light and dark themes reuse the
same geometry and resolve ink, error, and placeholder colors at draw time.
