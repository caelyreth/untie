# Changelog

## Unreleased

### Changed

- TeX display `\tag` and `\tag*` are now formula metadata instead of discarded
  diagnostics. Non-starred tags render display-time parentheses, starred tags
  render their literal body, `\nonumber`/`\notag` survive as suppression events,
  and display output exposes `Box.Tag` plus constrained `LineBox.Tag`.
  Formula-level tags use `Line=-1` (`FormulaTagLine`) consistently in neutral
  IR, display output, and public API. Constrained alignment reserves the tag
  width before positioning the formula, keeping a fitting formula and tag
  inside `MaxWidth`; overfull content follows the formula with honest overflow. Every
  syntactic argument boundary creates an independent metadata scope, so tags
  cannot be promoted from scripts, required/text/optional arguments, or
  macro-expanded equivalents. Nested Multiline/Table values no longer
  invalidate a top-level tag. Tagged output with `MaxWidth` uses one complete
  constrained line with honest tag logical overflow; parent `Italic` follows
  the rightmost tag run. Typst numbering, host counters, and row-scoped
  multiline tags remain deferred.
- `TableRow.Tag` and the table-provider tag fragment were removed. They had no
  producing frontend and incorrectly suggested that top-level equation tags
  belonged inside table rows. Use `ir.Formula.Tags` and `display.TagBox`.
- `display.Break.Penalty` and `ir.Break.Penalty` were removed, and
  `ir.NewBreak` no longer accepts a penalty argument. Break metadata now carries
  only source position and legal-cut kind. Constrained display wrapping remains
  width-greedy and deterministically chooses the earliest source cut when
  widths are equal. Callers that previously treated penalties as costs must
  implement their own policy without Untie fabricating one.
- Links are text-only. TeX `\href`/`\url` and Typst `link` preserve visible body
  text and source spans, but destinations are intentionally discarded with an
  unsupported warning after argument validation. Braced TeX URL groups preserve
  raw characters such as `%20`, `~`, `#`, whitespace, and balanced braces;
  `\url` also accepts one token, and unclosed groups report
  `unclosed-group`. Empty `\url{}` and `\href{}{body}` remain valid. Typst
  destinations in the bounded evaluator must be non-empty strings; labels,
  references, and document-level resolution are outside scope. There is no
  neutral link metadata.
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

### Unsupported

- `\nobreak` remains outside the neutral break contract and produces an
  unsupported diagnostic. Untie does not implement a cost-based line breaker.
- Full LaTeX `hyperref` behavior, Typst labels/references as destinations,
  destination metadata, and clickable output are outside the neutral formula
  IR.
- TeX URL catcodes are applied lexically to a literal braced group in source or
  a host macro body. Bare `\url%...` remains a normal TeX comment, and text
  assembled only from already-lexed macro arguments is not retroactively
  reparsed as a raw URL.
