# Untie for MyGo

This module connects Untie to [MyGo](https://mygo.egoist.dev/) with one
bounded, concurrency-safe adapter:
The module path is `github.com/caelyreth/untie/contrib/mygo`; its package name
is `untiemygo`, so no import alias is needed.

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

`BindMarkdown` enables `$...$`, `$$...$$`, `\(...\)`, and `\[...\]` math and
installs both Markdown callbacks. Inline objects map the complete
`Box.Bounds`, including certified overflow, into MyGo's baseline-relative
width, height, and depth. Display math paints at native size and scales down
when its host is narrower.

`Renderer.Draw(painter, box, viewport)` is also public for hosts that obtain a
`*untie.Box` from the root API and want the same fitting and theme-role
behavior.

## Cache And Ownership

The adapter caches laid-out boxes by source, dialect, mode, and complete layout
style. `CacheEntries: 0` uses 128 entries, a negative value disables caching,
and the maximum is 4096. The cache is concurrency-safe.

Cached and returned boxes are immutable. A caller may read a box from
`Renderer.Render` but must not modify it or its items, paths, hits, lines, or
alternative text while any goroutine or draw callback can use it.

Semantic paint roles are resolved from `ui.Painter.Theme()` on every draw, so
one cached box works in light and dark themes. Parse recovery remains visible
through Untie's `RoleError` and `RolePlaceholder` paints.

## Tests

The package uses `ui.Tester` for Markdown inline and display math, theme color
changes, and rendering without a window:

```sh
cd contrib/mygo
go test ./...
go test -race ./...
go vet ./...
```
