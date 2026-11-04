# Untie Stream With MyGo

An editable, pseudo-streamed Markdown reply rendered with MyGo and Untie.
The editor fills the window; **Render stream** switches to a full-page preview
and reveals the source at the configurable byte rate. Display-list glyphs keep
their source identity across layouts: existing paths move point-by-point while
rules such as fraction bars interpolate their bounds and scale. **Edit source**
returns to the editor.

Math keeps its natural scale in a fixed-height, two-axis scroll viewport. Large
formulas pan inside that viewport instead of shrinking or repeatedly reflowing
the whole page as tokens arrive.

The example uses MyGo's incremental `markdown.Stream` parser and Untie's normal
renderer cache. The morph is drawn through the public display-list adapter, not
by mutating renderer internals.

## Run

```sh
cd examples/mygo-stream
go tool mygo dev
```

The New Computer Modern Math font is embedded from `assets` and remains under
the GUST Font License in `assets/GUST-FONT-LICENSE.txt`.
