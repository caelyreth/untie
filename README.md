### Untie

Untie is a pure-Go math renderer for TeX and Typst math. It parses both
languages into one language-neutral IR, lays formulas out with an OpenType MATH
font supplied by the host, and returns a UI-independent display list of glyph
paths and filled rules.

See the [documentation](docs/README.md) for integration, rendering, font,
parsing, accessibility, and compatibility guidance.

### Properties

- `CGO_ENABLED=0`
- no UI, DOM, GPU, platform text, or system-font dependency
- deterministic, byte-stable canonical test output
- recoverable parsing with source diagnostics
- UTF-8 byte spans
- fixed-point internal geometry
- OpenType MATH metrics and glyph outlines
- reusable caller-owned layout engines with bounded convenience caching
- source-linked hit rectangles emitted from placed layout fragments
- deterministic inline break metadata and constrained display line boxes
- TeX display equation-tag metadata and separately selectable tag ranges
- text-only link bodies with explicit unsupported destination diagnostics
- deterministic text/speech alternative output with bounded fallbacks
- a tracked daily-STEM acceptance corpus
- no upstream renderer is compiled or executed by the test suite

### Use

```go
import (
	"context"
	"os"

	"github.com/caelyreth/untie"
)

fontBytes, err := os.ReadFile("NewCMMath-Book.otf")
if err != nil {
	return err
}
fonts, err := untie.LoadMathFont(fontBytes)
if err != nil {
	return err
}
renderer, err := untie.NewRenderer(fonts, untie.RendererOptions{
	DefaultStyle: untie.Style{Size: 16},
	CacheEntries: 128,
})
if err != nil {
	return err
}

result, err := renderer.Render(context.Background(), `x^2+\frac{1}{2}`, untie.RenderRequest{
	Mode: untie.Display,
})
if err != nil {
	return err
}
// Draw result.Box through render.Walk or result.Box.Items.
```

`NewRenderer` is the recommended parse-and-layout facade. It prepares one
`Engine`, applies bounded parsed-expression caching when requested, and never
caches returned display boxes. Use `NewEngine` directly when the host needs the
lower-level lifecycle boundary.

Display layout with a positive `Style.MaxWidth` returns `box.Lines` inside a
`MaxWidth`-wide alignment frame. `box.Items` and `box.Hits` are the single
drawable storage; each line carries ranges into them plus baseline, alignment,
and overflow metadata. `Style.Align` positions each line in the frame with
start, center, or end alignment. Inline layout ignores `MaxWidth` and `Align`.

The package-level `Layout` and `LayoutContext` helpers use a bounded
process-wide engine cache that can be released with `ClearLayoutEngineCache`.

### Development

```sh
nix develop
go test ./...
go vet ./...
```

Pure-Go verification:

```sh
CGO_ENABLED=0 go test ./...
```

The repository-wide corpus and compatibility gates live under
`internal/quality`:

```sh
go test ./internal/quality
```

A two-pane native editor example is in
[examples/mygo](examples/mygo).

A separately versioned MyGo Markdown adapter is in
[contrib/mygo](contrib/mygo). Its package name is `untiemygo`, so imports need
no alias. It provides inline and display math hooks, a bounded immutable layout
cache, theme-role drawing, and `ui.Tester` coverage without adding MyGo to the
root module's dependencies.

### License

Copyright 2026 Yu (Caelyreth).

Untie is dual-licensed under either:

- the [MIT License](LICENSE-MIT);
- the [Apache License, Version 2.0](LICENSE-APACHE).

You may choose either license at your option.

The SPDX expression for original project material is `MIT OR Apache-2.0`.

Third-party material remains under its own license.
