# Untie for MyGo

`github.com/caelyreth/untie/contrib/mygo` renders TeX and Typst math as native
[MyGo](https://mygo.egoist.dev/) UI content. The package name is `untiemygo`.

```go
fonts, err := untie.LoadMathFont(fontBytes)
if err != nil {
	return err
}

math, err := untiemygo.New(fonts, untiemygo.Options{
	Dialect:      untie.TeX,
	BaseSize:     16,
	DisplaySize:  24,
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

## Direct Elements

`Renderer.Element` turns a formula string directly into a MyGo `ui.Element`:

```go
ui.Column(c).Children(func() {
	math.Element(c, `\int_0^1 x^2\,dx`, untie.Display).FillWidth()
})
```

The element uses the formula's natural size plus the configured padding. It
scales down when its layout box is smaller and resolves semantic paint roles
from the active MyGo theme. If parsing or layout fails, it shows the source as
selectable danger-colored text.

Use `untie.Inline` for a standalone inline-size element. Use `InlineMath` or
Markdown binding when the formula must share a text baseline with surrounding
content.

## Markdown

`BindMarkdown` enables `$...$`, `$$...$$`, `\(...\)`, and `\[...\]` math while
retaining the non-math callbacks and parser extensions already configured:

```go
var options markdown.Options
math.BindMarkdown(&options)
markdown.View(c, source, &options)
```

Inline math maps the complete `Box.Bounds`, including overflow, into MyGo's
baseline-relative width, height, and depth. Display math paints at native size
and scales down when its host is narrower.

## Existing Boxes

`Renderer.Render` returns the underlying Untie box and diagnostics when an
application needs them:

```go
result, err := math.Render(ctx, source, untie.Display, untie.Style{})
```

`Renderer.Draw(painter, box, viewport)` paints such a box with the adapter's
padding, alignment, scale policy, and theme colors.

## Cache And Ownership

The adapter caches laid-out boxes by source, dialect, mode, and complete layout
style. `CacheEntries: 0` uses 128 entries, a negative value disables caching,
and the maximum is 4096. `CacheStats` reports size, capacity, hits, misses, and
evictions.

Cached and returned boxes are immutable. Callers may read them but must not
modify their items, paths, hits, lines, or alternative text while they may be
used by another goroutine or draw callback.

Semantic paint roles are resolved from the active theme on every draw, so one
cached box works in light and dark themes.
