# Untie Stream With MyGo

An editable, pseudo-streamed Markdown reply rendered with MyGo and Untie.
The editor fills the window; **Render stream** switches to a full-page preview
and reveals the source at the configurable byte rate while display math smoothly
crossfades and scales into each new layout. **Edit source** returns to the
editor.

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
