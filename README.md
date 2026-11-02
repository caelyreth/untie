# Untie

Untie is a pure-Go math renderer for TeX and Typst math. It parses both
languages into one language-neutral IR, lays formulas out with an OpenType MATH
font supplied by the host, and returns a UI-independent display list of glyph
paths and filled rules.

The project is under active implementation. The current repository contains the
public contracts, canonical IR, geometry, deterministic display-list types, and
the first frontend/font/layout work in progress.

## Properties

- `CGO_ENABLED=0`
- no UI, DOM, GPU, platform text, or system-font dependency
- deterministic, byte-stable canonical test output
- recoverable parsing with source diagnostics
- UTF-8 byte spans
- fixed-point internal geometry
- OpenType MATH metrics and glyph outlines
- reusable caller-owned layout engines with bounded convenience caching
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

