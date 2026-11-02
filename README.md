### Untie

Untie is a pure-Go math renderer for TeX and Typst math. It parses both
languages into one language-neutral IR, lays formulas out with an OpenType MATH
font supplied by the host, and returns a UI-independent display list of glyph
paths and filled rules.

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
- TeX display equation-tag metadata and independently drawable tag boxes
- text-only link bodies with explicit unsupported destination diagnostics
- deterministic text/speech alternative output with bounded fallbacks
- a tracked daily-STEM acceptance corpus
- no upstream renderer is compiled or executed by the test suite

### Use

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

Display layout with a positive `Style.MaxWidth` returns `box.Lines` inside a
`MaxWidth`-wide alignment frame. Each line's items remain line-local and are
drawn at `(line.OffsetX, line.Baseline)`. The parent `box.Items`, `box.Hits`,
and `box.Overflow` are the translated aggregate, so a host must draw either the
parent items or the per-line items, never both. `Style.Align` positions each
line in the frame with start, center, or end alignment. Inline layout ignores
`MaxWidth` and `Align`.

`NewEngine` is the caller-owned lifecycle API for repeated layouts. The
package-level `Layout` and `LayoutContext` helpers use a bounded process-wide
engine cache that can be released with `ClearLayoutEngineCache`.

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

### License

Copyright 2026 Yu (Caelyreth).

Untie is dual-licensed under either:

- the [MIT License](LICENSE-MIT);
- the [Apache License, Version 2.0](LICENSE-APACHE).

You may choose either license at your option.

The SPDX expression for original project material is `MIT OR Apache-2.0`.

Third-party material remains under its own license.
