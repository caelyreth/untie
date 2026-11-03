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

Standalone elements use `Alt.Speech` as their accessibility label, with
`Alt.Text` and source as fallbacks, and the exact dialect source as their
accessibility value. `Options.Accessibility` can disable the extra
alternative-text traversal.

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

When the host knows the available width, use `DisplayMathWidth` so Untie can
wrap the display formula before drawing:

```go
math.DisplayMathWidth(c, source, availableWidth)
```

The width is the outer MyGo element width. The adapter subtracts configured
draw padding and passes the remaining frame as `Style.MaxWidth`. The resulting
`Box.Lines` are drawn at natural scale instead of uniformly shrinking the
whole formula.

## Editor Integration

Use `ElementWithOptions`, `DisplayMathWithOptions`, or
`DisplayMathWidthWithOptions` for editor behavior:

```go
element := math.DisplayMathWidthWithOptions(
	c,
	source,
	width,
	untiemygo.ElementOptions{
		OnHit: func(event untiemygo.HitEvent) {
			if event.Found && event.Phase == untiemygo.HitClick {
				editAt(event.Hit.Source)
			}
		},
		Diagnostics: untiemygo.DiagnosticOptions{
			Mode: untiemygo.DiagnosticUnderline |
				untiemygo.DiagnosticTooltip,
		},
		Clipboard: untiemygo.ClipboardOptions{Enabled: true},
	},
)
```

`OnHit` emits hover, press, and click events from MyGo pointer input. `HitAt`
performs the same inverse viewport transform for applications with their own
event dispatch. MyGo v0.7.4 does not expose a reliable pointer-leave event, so
hosts must clear hover state when another hit begins.

Diagnostics can paint source-range underlines through `Box.Covering` and attach
a whole-element tooltip. `DiagnosticOptions.Paint` and `DiagnosticOptions.Format`
are the adapter-side extension points for custom drawing and localized text.

`FormulaClipboardData` and related helpers offer plain text plus
`text/x-tex`/`text/x-typst` source. The context-menu option writes those formats
directly. Inline Markdown math already copies as source. Untie has no MathML
serializer, so the adapter intentionally exposes no MathML clipboard format.

For baseline-stable inline math, set:

```go
Options{Inline: untiemygo.InlineOptions{
	Metrics: untiemygo.InlineIgnoreOverflow,
}}
```

This reports nominal width/ascent/descent while preserving certified overflow
as drawable ink inside the host text clip.

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

For a framework-neutral MyGo draw path that does not require an adapter
instance, use:

```go
err := untiemygo.DrawBox(painter, box, viewport, untiemygo.DrawOptions{
	Padding: 8,
	Align:   render.AlignCenter,
	Scale:   untiemygo.ScaleNatural,
	Paint: func(_ *ui.Painter, paint untie.Paint) ui.Color {
		return myColorPolicy(paint)
	},
})
```

`ScaleFit` is the compact preview behavior. `ScaleNatural` preserves wrapping
and glyph metrics at scale 1. A custom `PaintResolver` receives both semantic
role and explicit source color, so a host can preserve, reject, or adapt either
without changing cached geometry.

## Font Ownership

Untie and MyGo intentionally own different font inputs:

- Untie needs the host-supplied bytes for a MATH face, optional text face, and
  ordered fallback faces;
- MyGo's `ui.RegisterFont` registry supplies families to its platform text
  engine, but does not expose the MATH tables or complete font data Untie needs.

Load and share the Untie `*FontSet` directly. Do not register the math font with
MyGo merely to make the registries look alike. A future host-text port may let
MyGo shape `\text{}` while Untie continues to own math-font parsing, MATH
layout, and deterministic outline output; that mode must remain explicit and
cache-isolated because platform text is not byte-deterministic.

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
