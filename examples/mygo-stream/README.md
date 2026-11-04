# Untie Stream With MyGo

An editable, pseudo-streamed Markdown reply rendered with MyGo and Untie.
The editor fills the window; **Render stream** switches to a full-page preview
and reveals the source at the configurable byte rate. Display-list glyphs keep
their source identity across layouts: existing paths move point-by-point while
rules such as fraction bars interpolate their bounds and scale. **Edit source**
returns to the editor.

Math keeps the completed formula's natural scale and stage height while partial
layouts animate inside it. Wide formulas pan horizontally instead of shrinking
or repeatedly reflowing the whole page as tokens arrive.

The example uses MyGo's incremental `markdown.Stream` parser and Untie's native
`stream.Session` through `untiemygo.StreamingMath`. Rapid source updates are
coalesced before layout, and the public display transition supplies both moved
and entering commands without mutating renderer internals.

## Run

```sh
cd examples/mygo-stream
go tool mygo dev
```

The New Computer Modern Math font is embedded from `assets` and remains under
the GUST Font License in `assets/GUST-FONT-LICENSE.txt`.
