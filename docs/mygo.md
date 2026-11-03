# MyGo Integration

`github.com/caelyreth/untie/contrib/mygo` is a separate Go module that adapts
Untie to MyGo without adding any UI dependency to the root library.
The module declares package `untiemygo`, so imports use that name without an
alias.

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

var options markdown.Options
math.BindMarkdown(&options)
markdown.View(c, source, &options)
```

`BindMarkdown` retains existing options and installs `Math`, `InlineMath`,
`ExtMath`, and `ExtDollarMath`. It supports `$...$`, `$$...$$`, `\(...\)`, and
`\[...\]`.

## Drawing Contract

`InlineMath` maps the complete `Box.Bounds`, including certified overflow, into
MyGo's baseline-relative `Width`, `Height`, and `Depth`. Its callback paints
through `render.Walk`; it does not materialize another display list.

`DisplayMath` builds a full-width MyGo element. It paints at native size and
scales down when the available width is insufficient.

`Renderer.Draw` is public for hosts that have a `*untie.Box` from the root API.
It fits `Box.Bounds` into a `ui.Rect`, applies the configured padding,
alignment, and upscale policy, and resolves semantic paint roles from the
active theme.

## Cache And Ownership

The adapter caches laid-out boxes by source, dialect, mode, and full layout
style. The cache is a concurrency-safe LRU:

- `CacheEntries: 0` uses 128 entries;
- a negative value disables caching;
- `MaxCacheEntries` is 4096.

`CacheStats` reports size, capacity, hits, misses, and evictions. Returned and
cached boxes are immutable. A caller may read a box returned by
`Renderer.Render`, but must not modify it or its slices while shared by cache
users, concurrent workers, or draw callbacks.

Paint roles remain semantic in the cache, so light and dark themes reuse the
same geometry and resolve ink, error, and placeholder colors at draw time.

## Module Boundary

The root `github.com/caelyreth/untie` module has no MyGo dependency. The nested
module path is `github.com/caelyreth/untie/contrib/mygo`.

The architecture gate allows only the public Untie packages and exact MyGo
packages used by the adapter. It rejects Untie internals, MyGo internals, cgo,
and undeclared framework dependencies.
