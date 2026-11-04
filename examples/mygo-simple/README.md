# Untie With MyGo: Simple

A native desktop example built with [MyGo](https://mygo.egoist.dev/). The
window has two areas:

- a source editor with TeX/Typst switching;
- a renderer that displays a direct `untiemygo` formula element;
- source-hit feedback from pointer hover and clicks;
- diagnostic underlines, tooltips, accessible formula labels, and copy actions.

The two dialects keep separate source buffers, so switching syntax does not
discard the formula being edited.

## Run

```sh
cd examples/mygo-simple
go tool mygo dev
```

The New Computer Modern Math font is embedded from `assets` and remains under
the GUST Font License in `assets/GUST-FONT-LICENSE.txt`.
