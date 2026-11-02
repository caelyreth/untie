# New Computer Modern Math test data

## Font

- File: `NewCMMath-Book.otf`
- SHA-256: `2ea09ebc9167b1e1a66f31390dc917f2d4004ecfca72d51b28010e4ad6becd95`
- Copyright: 2019-2026 Antonis Tsolomitis
- Source: `typst-assets` 0.15.1, `files/fonts/NewCMMath-Book.otf`
- License: GUST Font License 1.0; see `GUST-FONT-LICENSE.txt`
- Transformation: none

The file is bundled without modification for deterministic parser and metric
tests. The GUST license permits redistribution with this notice and license.

## Metrics and shaping fixture

- File: `Typlet/font.jsonl`
- SHA-256: `24e779f01cb0d19278fb6fb1bcd55171ff58adab6f05b406804a3f0ff1d432f5`
- Source: Typlet 0.2.0, `test/fixtures/typst-0.15.1/font.jsonl`
- License: MIT; see `Typlet/LICENSE`
- Transformation: none

The fixture was generated upstream by Typlet from Typst 0.15.1 observations of
the exact font above. Untie only decodes the static JSON lines; it never runs
Typst or Typlet.

Known fixture edge cases are documented in `font/README.md`: four private-use
glyph bounds have a one-design-unit CFF quantization difference, U+FFFF maps to
`.notdef`, and one left-corner MATH kern is omitted upstream. The conformance
test names each exception explicitly.

