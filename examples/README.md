# Examples

- [`mygo-simple/`](mygo-simple/) shows Untie's editor-oriented MyGo element and
  Markdown integration in a native desktop window.
- [`mygo-stream/`](mygo-stream/) streams editable Markdown with morphing math
  into a MyGo view.
- [`mathml/`](mathml/) converts TeX or Typst to deterministic Presentation MathML.
- [`svg/`](svg/) exports a formula as a deterministic standalone SVG.
- [`png/`](png/) rasterizes a formula to an antialiased PNG.

## MathML

Write Presentation MathML to standard output:

```sh
go run ./examples/mathml \
  -source '\frac{-b \pm \sqrt{b^2-4ac}}{2a}'
```

Use `-dialect typst` for Typst input, `-mode inline` for inline math, or
`-out formula.xml` to write a file. This example needs no font.

## SVG

Write an SVG to a file:

```sh
go run ./examples/svg \
  -source '\frac{-b \pm \sqrt{b^2-4ac}}{2a}' \
  -out formula.svg
```

Omit `-out` to write the SVG document to standard output. Use
`-dialect typst` to change the input language, or `-mode inline` for inline
layout.

## PNG

Write a two-pixel-per-unit PNG with a white background:

```sh
go run ./examples/png \
  -source '\sum_{k=1}^{n} k^2' \
  -scale 2 \
  -out formula.png
```

Omit `-out` to write the PNG stream to standard output. Pass `-transparent` to
keep the background transparent. Both examples use
`testdata/upstream/fonts/NewCMMath-Book.otf` by default; override it with
`-font`.
