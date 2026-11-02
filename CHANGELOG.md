# Changelog

## Unreleased

### Changed

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
