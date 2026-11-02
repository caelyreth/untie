# Untie

Untie is a pure-Go math renderer for TeX and Typst math. It parses both
languages into one language-neutral IR, lays formulas out with an OpenType MATH
font supplied by the host, and returns a UI-independent display list of glyph
paths and filled rules.

The project is under active implementation. The current repository contains the
public contracts, canonical IR, geometry, deterministic display-list types, and
the first frontend/font/layout work in progress.

## Use

```go
fontBytes, err := os.ReadFile("NewCMMath-Book.otf")
if err != nil {
	return err
}
fonts, err := untie.LoadFonts(untie.Fonts{
	Math: untie.FontData{Bytes: fontBytes, ID: "newcm-math"},
})
if err != nil {
	return err
}
engine, err := untie.NewEngine(fonts, untie.Limits{})
if err != nil {
	return err
}

expr := untie.ParseTeX(`x^2+\frac{1}{2}`, untie.Display, untie.ParseOptions{})
box, err := engine.Layout(expr, untie.Style{Size: 16})
if err != nil {
	return err
}
// Draw box.Items as filled paths/rules and use box.Hits for source mapping.
```

`NewEngine` is the caller-owned lifecycle API for repeated layouts. The
package-level `Layout` and `LayoutContext` helpers use a bounded process-wide
engine cache that can be released with `ClearLayoutEngineCache`.

## Properties

- `CGO_ENABLED=0`
- no UI, DOM, GPU, platform text, or system-font dependency
- deterministic, byte-stable canonical test output
- recoverable parsing with source diagnostics
- UTF-8 byte spans
- fixed-point internal geometry
- OpenType MATH metrics and glyph outlines
- reusable caller-owned layout engines with bounded convenience caching
- source-linked hit rectangles emitted from placed layout fragments
- no reference project is compiled or executed by the test suite

## Development

```sh
nix develop
go test ./...
go vet ./...
```

Pure-Go verification:

```sh
CGO_ENABLED=0 go test ./...
```

## Status

The public API and IR are still pre-1.0 and may change while the renderer's
vertical slice is completed. Unsupported behavior must produce a visible
diagnostic; silent approximation is not an accepted fallback.

