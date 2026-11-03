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

Standalone elements expose semantic accessibility text by default:

- `Alt.Description` is available through `DescribeFormula`;
- `Alt.Speech` (or `Alt.Text`/source as a fallback) is the accessible label;
- the exact TeX or Typst source is the accessibility value;
- an empty formula still has a stable accessible name.

Hosts can replace the English label using the structured tree:

```go
math, err := untiemygo.New(fonts, untiemygo.Options{
	LocalizeLabel: func(description *untie.AltDescription) string {
		return catalog.Render(description)
	},
})
```

An empty callback result uses `Alt.Speech`. The callback receives a
source-independent semantic tree and must not retain or mutate it. See
[structured accessibility descriptions](../docs/accessibility-description.md).

Use `Options.Accessibility` to skip alternative-text generation when an
application does not need the richer spoken label. The source remains the
accessibility value in either mode.

`DisplayMath` is the compact preview path used by Markdown. When an editor or
another host knows the available width, call `DisplayMathWidth` instead:

```go
math.DisplayMathWidth(c, `a+b+c+d`, 480)
```

The width is the element's outer width. The adapter subtracts `DrawOptions.Padding`
and passes the remainder to Untie as `Style.MaxWidth`. Untie then emits
`Box.Lines` and the adapter draws at natural scale instead of shrinking the
complete formula uniformly.

Use `untie.Inline` for a standalone inline-size element. Use `InlineMath` or
Markdown binding when the formula must share a text baseline with surrounding
content.

## Editor Elements

`ElementWithOptions`, `DisplayMathWithOptions`, and
`DisplayMathWidthWithOptions` add editor interactions without changing the
basic element behavior:

```go
math.ElementWithOptions(c, source, untie.Display, untiemygo.ElementOptions{
	OnHit: func(event untiemygo.HitEvent) {
		if event.Found && event.Phase == untiemygo.HitClick {
			placeCaretAt(event.Hit.Source)
		}
	},
	Diagnostics: untiemygo.DiagnosticOptions{
		Mode: untiemygo.DiagnosticUnderline | untiemygo.DiagnosticTooltip,
	},
	Clipboard: untiemygo.ClipboardOptions{Enabled: true},
})
```

`OnHit` reports hover, primary press, and completed click phases. The hit comes
from `untie.Box.At` after inverting the same transform used to draw the element.
`HitAt` exposes that geometry directly for hosts with custom event models.
MyGo v0.7.4 has no dependable hover-exit callback, so an editor should treat a
new hover or press elsewhere as leaving the previous hit.

Diagnostic underlines map each diagnostic source range through `Box.Covering`.
`DiagnosticOptions.Paint` receives transformed marks when an application wants
to draw its own decoration. `DiagnosticOptions.Format` can localize tooltip
text. A diagnostic without covered geometry still contributes to the tooltip.

`BindMarkdownWithOptions` applies the same element options to display math
inside a Markdown view.

## Copy And Selection

`FormulaClipboardData`, `PlainTextClipboardData`, `SourceClipboardData`, and
`SourceFormat` expose copy data to applications with their own selection model.
The context-menu option writes:

- plain text from `Alt.Text`, falling back to source;
- `text/x-tex` or `text/x-typst` source.

Inline math in MyGo Markdown already copies as its source through
`markdown.InlineObject`. Standalone formula elements do not expose a text
selection model; enable `Clipboard` for explicit copy menu actions.

Set `ClipboardOptions.MathML` to add `application/mathml+xml` to the context
menu. `Renderer.MarshalMathML` and the clipboard helpers use the standalone
`mathml` package and the adapter's configured ParseOptions.

## Inline Baseline

By default, Markdown inline objects include certified overflow in their
baseline-relative width, height, and depth. Editors can keep line height stable
while retaining overflow ink inside the surrounding text clip:

```go
math, err := untiemygo.New(fonts, untiemygo.Options{
	Inline: untiemygo.InlineOptions{
		Metrics: untiemygo.InlineIgnoreOverflow,
	},
})
```

The nominal mode reports `Box.Width`, `Box.Ascent`, and `Box.Descent`. The draw
transform remains baseline-correct and uses the host paragraph clip for
certified overflow outside those nominal extents.

## Markdown

`BindMarkdown` enables `$...$`, `$$...$$`, `\(...\)`, and `\[...\]` math while
retaining the non-math callbacks and parser extensions already configured:

```go
var options markdown.Options
math.BindMarkdown(&options)
markdown.View(c, source, &options)
```

By default, inline math maps the complete `Box.Bounds`, including overflow, into
MyGo's baseline-relative width, height, and depth. `InlineIgnoreOverflow`
selects nominal metrics. Display math paints at native size and scales down when
its host is narrower.

## Existing Boxes

`Renderer.Render` returns the underlying Untie box and diagnostics when an
application needs them:

```go
result, err := math.Render(ctx, source, untie.Display, untie.Style{})
```

`Renderer.Draw(painter, box, viewport)` uses the adapter's configured draw
options. The package-level helper can draw the same box without a renderer:

```go
err := untiemygo.DrawBox(painter, box, rect, untiemygo.DrawOptions{
	Padding: 8,
	Align:   render.AlignCenter,
	Scale:   untiemygo.ScaleNatural,
	Paint: func(_ *ui.Painter, paint untie.Paint) ui.Color {
		if paint.HasExplicit {
			return myExplicitColorPolicy(paint.Explicit)
		}
		return myRoleColor(paint.Role)
	},
})
```

`ScaleFit` is the compact/preview behavior. `ScaleNatural` preserves wrapping
and glyph metrics at scale 1.

## Paint Policy

Explicit formula colors are authoritative by default. `\color`, `\textcolor`,
and Typst color calls keep their source color in light and dark MyGo themes.
Semantic ink, error, and placeholder roles resolve from the active theme on
every draw, so cached geometry follows theme changes.

A custom `PaintResolver` receives the complete `untie.Paint`, including
`HasExplicit`, `Explicit`, and `Role`. It can preserve explicit colors, reject
or adjust them against a host background, or map all semantic roles into a
non-MyGo theme. Resolvers should only inspect the painter and paint; draw
commands still belong to the adapter.

Hosts that want color policy to participate in layout cache identity can set
`untie.Style.ColorPolicy` before rendering. Preserve is the zero-value default;
map-to-role and contrast-adjusted results remain ordinary `untie.Paint` values,
so `ThemePaintResolver` and custom resolvers continue to consume them.

## Fonts

Untie and MyGo intentionally own different font inputs:

- Untie takes a prepared `*untie.FontSet` with OpenType MATH tables, variants,
  assemblies, and outline metrics.
- MyGo's `ui.RegisterFont` registry supplies families to the platform text
  engine and exposes shaped text glyphs, but not the MATH data or the complete
  font bytes Untie needs.

Load the math font once and share that `FontSet` across adapters. Do not
register it with MyGo merely to make the two registries look alike; doing so
would duplicate ownership without enabling MATH layout.

## Cache And Ownership

The adapter caches laid-out boxes by source, dialect, mode, and complete layout
style, including `MaxWidth`, `ColorPolicy`, and `PixelGrid`. Compact,
width-aware, and differently styled layouts therefore have distinct cache
entries. `CacheEntries: 0` uses 128 entries, a negative value disables caching,
and the maximum is 4096. `CacheStats` reports size, capacity, hits, misses, and
evictions. Cached boxes retain their immutable `FontUsage` report.

Cached and returned boxes are immutable. Callers may read them but must not
modify their items, paths, hits, lines, or alternative text while they may be
used by another goroutine or draw callback.

Semantic paint roles and custom paint policies are resolved on every draw, so
one cached box works across theme changes and host-provided color policies.
