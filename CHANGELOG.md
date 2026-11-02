# Changelog

## Unreleased

### Changed

- `display.Break.Penalty` and `ir.Break.Penalty` were removed, and
  `ir.NewBreak` no longer accepts a penalty argument. Break metadata now carries
  only source position and legal-cut kind. Constrained display wrapping remains
  width-greedy and deterministically chooses the earliest source cut when
  widths are equal. Callers that previously treated penalties as costs must
  implement their own policy without Untie fabricating one.
- Links are text-only. TeX `\href`/`\url` and Typst `link` preserve visible body
  text and source spans, but destinations are intentionally discarded with an
  unsupported warning. There is no neutral link metadata.
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
