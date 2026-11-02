# Untie

Untie is a pure-Go math renderer for TeX and Typst math. It parses both
languages into one language-neutral IR, lays formulas out with an OpenType MATH
font supplied by the host, and returns a UI-independent display list of glyph
paths and filled rules.

The project is under active implementation. The repository now contains a
working end-to-end vertical slice for both dialects, including canonical IR,
OpenType MATH layout, source hits, inline breaks, alternative text, and
width-constrained display lines.

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

Display layout with a positive `Style.MaxWidth` returns baseline-offset
`box.Lines`; draw each line's independent items at that line baseline. The
parent `box.Items` are the already-translated aggregate, so a host must draw
either the parent items or the per-line items, never both. Inline layout ignores
`MaxWidth`.

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
- deterministic inline break metadata and constrained display line boxes
- deterministic text/speech alternative output with bounded fallbacks
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

