# Untie With MyGo

A native desktop example built with [MyGo](https://mygo.egoist.dev/). The
window has two areas:

- a source editor with TeX/Typst switching;
- a renderer that paints Untie's display list with `ui.Painter`.

The two dialects keep separate source buffers, so switching syntax does not
discard the formula being edited.

## Run

```sh
cd examples/mygo
go tool mygo dev
```

## Test

```sh
go test ./...
```

The New Computer Modern Math font is embedded from `assets` and remains under
the GUST Font License in `assets/GUST-FONT-LICENSE.txt`.
