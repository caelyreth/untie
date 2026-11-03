## Untie

Untie is a pure-Go math renderer for TeX and Typst. It parses both input
languages into one language-neutral representation, lays formulas out with a
host-supplied OpenType MATH font, and returns a UI-independent display list of
glyph paths, rules, source hits, and accessibility text.

### Properties

- one renderer for TeX and Typst math;
- pure Go with `CGO_ENABLED=0`;
- no browser, DOM, GPU, platform text, or system-font dependency;
- deterministic layout and output;
- recoverable parsing with source diagnostics;
- source-mapped hit regions;
- OpenType MATH metrics, glyph outlines, variants, and assemblies;
- reusable rendering with bounded caches;
- inline and display layout, line breaking, equation tags, and overflow bounds;
- deterministic text and speech alternatives for accessibility;
- optional SVG and PNG output.

### Use

`NewRenderer` is the main API for applications. Load the font once, keep the
renderer, and call `Render` whenever the source or style changes.

```go
import (
	"context"
	"os"

	"github.com/caelyreth/untie"
	"github.com/caelyreth/untie/render"
)

fontBytes, err := os.ReadFile("NewCMMath-Book.otf")
if err != nil {
	return err
}
fonts, err := untie.LoadMathFont(fontBytes)
if err != nil {
	return err
}
math, err := untie.NewRenderer(fonts, untie.RendererOptions{
	DefaultStyle: untie.Style{Size: 24},
	CacheEntries: 128,
})
if err != nil {
	return err
}

result, err := math.Render(context.Background(), `x^2+\frac{1}{2}`, untie.RenderRequest{
	Mode: untie.Display,
})
if err != nil {
	// Recoverable parse diagnostics may still be present in result.
	return err
}

err = render.Walk(result.Box, func(command render.Command) error {
	switch command.Kind {
	case render.CommandPath:
		// Paint command.Path using command.At, command.Em, and command.Paint.
	case render.CommandRule:
		// Fill command.Rect with command.Paint.
	}
	return nil
})
```

The renderer caches immutable parsed expressions when `CacheEntries` is
positive. It never reuses returned boxes, so each successful `Render` produces
an independent display list.

For ready-made output, use
[`encode/svg`](encode/svg/doc.go) or [`encode/raster`](encode/raster/doc.go).
SVG supports deterministic vector export; the raster package provides
antialiased RGBA and PNG output without cgo.

### Lower-Level API

Use `NewEngine` when you need explicit engine ownership, direct canonical-IR
layout, or a custom parse/layout pipeline. It exposes the same layout core as
`Renderer`, without the convenience facade.

```go
engine, err := untie.NewEngine(fonts, untie.Limits{})
if err != nil {
	return err
}
expr := untie.ParseTeX(`\sum_{i=1}^{n} i^2`, untie.Display, untie.ParseOptions{})
box, err := engine.Layout(expr, untie.Style{Size: 24})
```

See [Getting Started](docs/getting-started.md) for the complete
integration pipeline and [Rendering](docs/rendering.md) for the
drawing contract.

### MyGo

The optional [`contrib/mygo`](contrib/mygo) adapter renders directly to MyGo
elements and plugs into Markdown math:

```go
math, err := untiemygo.New(fonts, untiemygo.Options{
	Dialect:      untie.TeX,
	CacheEntries: 128,
})
if err != nil {
	return err
}

ui.Column(c).Children(func() {
	math.Element(c, source, untie.Display).FillWidth()
})
```

`BindMarkdown` adds `$...$`, `$$...$$`, `\(...\)`, and `\[...\]` support to a
MyGo Markdown view. See the
[MyGo integration guide](docs/mygo.md) for direct elements,
Markdown binding, and existing-box drawing.

### Documentation

- [Runnable Examples](examples/README.md)
- [Website and Documentation Project](web/README.md)
- [Getting Started](docs/getting-started.md)
- [Core Concepts](docs/concepts.md)
- [Rendering](docs/rendering.md)
- [Fonts](docs/fonts.md)
- [Parsing](docs/parsing.md)
- [MyGo Integration](docs/mygo.md)
- [Supported TeX](docs/tex-support.md)
- [Supported Typst](docs/typst-support.md)
- [API Stability](docs/stability.md)
- [Full Documentation Index](docs/README.md)

### License

Copyright 2026 Yu (Caelyreth).

Untie is dual-licensed under either:

- the [MIT License](LICENSE-MIT);
- the [Apache License, Version 2.0](LICENSE-APACHE).

You may choose either license at your option.

The SPDX expression for original project material is `MIT OR Apache-2.0`.

Third-party material remains under its own license.
