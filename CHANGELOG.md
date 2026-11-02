# Changelog

## Unreleased

### Changed

- TeX display `\tag` and `\tag*` are now formula metadata instead of discarded
  diagnostics. Non-starred tags render display-time parentheses, starred tags
  render their literal body, `\nonumber`/`\notag` survive as suppression events,
  and single-line display output exposes `Box.Tag` plus constrained
  `LineBox.Tag`. Typst numbering, host counters, and multiline row tags remain
  deferred.
- `TableRow.Tag` and the table-provider tag fragment were removed. They had no
  producing frontend and incorrectly suggested that top-level equation tags
  belonged inside table rows. Use `ir.Formula.Tags` and `display.TagBox`.
- Width-constrained display layout now uses the requested `Style.MaxWidth` as
  `Box.Width`, not the widest intrinsic line width. `AlignStart` with a positive
  `MaxWidth` therefore returns a frame-width box whose parent items and hits use
  frame coordinates. Hosts that need intrinsic content width should use each
  `LineBox.Box.Width`; hosts that draw `Box.Lines` must apply `LineBox.OffsetX`
  and `LineBox.Baseline`.
- The unused `Alt.MathML` field was removed. Alternative output currently
  contains only `Text` and `Speech`; MathML output requires a separately
  designed backend.
- `ParseOptions.Preamble` was removed. Typst preamble and formula-level code
  definitions are outside the bounded math renderer; hosts must evaluate or
  expand them before calling Untie and pass only formula source.
