# Attribution And Provenance

All files below `testdata/upstream` are test data. They are not linked into the
Untie library. The upstream projects were inspected statically; none is
compiled or executed by this repository's tests.

## Typlet

- Project: Typlet 0.2.0
- Commit: `c3ebcac2db54ee123faaab8c437c83e7f66ec0a5`
- Source: <https://github.com/dbccccccc/Typlet>
- License: MIT; see `licenses/Typlet-MIT.txt`
- Imported paths:
  - `test/corpus/paired.jsonl`
  - `test/fixtures/typst-0.15.1/paired.jsonl`
  - `test/fixtures/typst-0.15.1/font.jsonl`
  - selected records from `test/corpus/{features,code,symbols,syntax}.jsonl`
  - matching selected records from
    `test/fixtures/typst-0.15.1/{features,code,symbols,syntax}.jsonl`
- Transformations:
  - The paired corpus and its frame fixture are byte-for-byte copies.
  - The NewCM metric dump is a byte-for-byte copy.
  - Selected datasets retain original record order and values. `jq -c` was
    used only to select the IDs recorded in `manifests/typlet-selection.json`;
    whitespace and key spelling were not semantically changed.

Typlet's fixture output was generated from Typst 0.15.1 and New Computer Modern
Math. Typlet's notices state that the fixture data is generated output from
Typst. Typst is Copyright The Typst Project Developers and is licensed under
Apache-2.0; see `licenses/Typst-Apache-2.0.txt` and
`licenses/Typst-NOTICE.txt`.

## RaTeX

- Project: RaTeX 0.1.15
- Commit: `f0ef2cf7c15d85c41743d4bd2df19759cf0dfee3`
- Source: <https://github.com/erweixin/RaTeX>
- License: MIT; see `licenses/RaTeX-MIT.txt`
- Imported path: `tests/golden/test_cases.txt`
- Transformation: byte-for-byte copy as `core-test-cases.txt`.

The 1,075-line core corpus excludes RaTeX's chemistry and proof-tree suites.

## KaTeX

- Project: KaTeX 0.19.1
- Commit: `65d1e725206dbbd278704df641491cf877413b6b`
- Source: <https://github.com/KaTeX/KaTeX>
- License: MIT; see `licenses/KaTeX-MIT.txt`
- Source files consulted:
  - `test/errors-spec.ts`
  - `test/katex-spec.ts`
- Transformation: `katex/cases.jsonl` is a hand-transcribed, normalized
  selection. It retains source text, expected parse/error behavior, diagnostic
  position when the upstream test asserts one, strictness where relevant, and
  the source file/line. KaTeX's terminal underline rendering and JavaScript
  harness are intentionally not copied.

## New Computer Modern Math

- Font: New Computer Modern Math Book, version 4.0
- Author: Antonis Tsolomitis
- Exact artifact: `typst-assets` revision
  `94dcb990edb36311ef85162baa26e4a8205e0bf0`,
  `files/fonts/NewCMMath-Book.otf`
- Source:
  <https://github.com/typst/typst-assets/blob/94dcb990edb36311ef85162baa26e4a8205e0bf0/files/fonts/NewCMMath-Book.otf>
- Transformation: none
- License: GUST Font License 1.0, based on LPPL 1.3c or later. The complete
  font licensing section and third-party notice distributed by `typst-assets`
  are in `licenses/typst-assets-NOTICE.txt`.

This is the exact font face named `NewCMMath-Book` by the Typlet frame fixtures.
Its SHA-256 is pinned in `manifest.sha256`.

## Noto shaping subsets

- Copyright: 2013 Google LLC
- License: SIL Open Font License 1.1; see `licenses/Noto-OFL-1.1.txt`
- Transformation: each source was reduced with `pyftsubset` 4.63.0 to the code
  points used by the complex-script shaping tests while retaining all layout
  features.

Imported sources and paths:

- Noto Emoji regular, `v65`:
  <https://fonts.gstatic.com/s/notoemoji/v65/bMrnmSyK7YY-MEu6aWjPDs-ar6uWaGWuob-r0jwv.ttf>
  to `fonts/NotoEmoji-ShapingSubset.ttf`
- Noto Naskh Arabic regular, `v44`:
  <https://fonts.gstatic.com/s/notonaskharabic/v44/RrQ5bpV-9Dd1b1OAGA6M9PkyDuVBePeKNaxcsss0Y7bwvc5krA.ttf>
  to `fonts/NotoNaskhArabic-ShapingSubset.ttf`
- Noto Sans Devanagari regular, `v33`:
  <https://fonts.gstatic.com/s/notosansdevanagari/v33/TuGoUUFzXI5FBtUq5a8bjKYTZjtRU6Sgv3NaV_SNmI0b8QQCQmHn6B2OHjbL_08AlXQly-A.ttf>
  to `fonts/NotoSansDevanagari-ShapingSubset.ttf`

Only the shaping tests load these test fonts. They are not selected by Untie
itself and do not imply system-font discovery or bundled production fonts.

## Variable-font fixtures

- Source Sans variable, from `go-text/typesetting-utils` revision
  `4ffe8874dabc`, path `opentype/common/SourceSans-VF.ttf`.
- Estedad variable, from the same revision and path
  `opentype/common/Estedad-VF.ttf`.
- Source Sans license: `licenses/SourceSans-OFL-1.1.txt`. Copyright 2010-2024
  Adobe, with Reserved Font Name "Source".
- Estedad license: `licenses/Estedad-OFL-1.1.txt`. Copyright 2022 The Estedad
  Project Authors, <https://github.com/aminabedi68/Estedad>.

`fonts/SourceSans-VF-MATH.ttf` is a test-only derivative of the Source Sans
fixture. A deterministic SFNT rewrite preserves every upstream table and adds
a minimal MATH table with axis height, a horizontal variant for glyph 1, and
zero-valued remaining constants. This gives the variable-outline, HVAR, MVAR,
and MATH-construction tests one small offline fixture. It is not a production
font and does not imply system-font discovery.

Only variable-font tests load these faces. `fonts/Estedad-VF.ttf` is byte-for-
byte upstream data and supplies a second real design axis (`wdth`) for generic
axis and canonical-order tests.

## Corpus Manifest

`manifests/corpora.json` records the exact loadable corpus paths and counts.
`manifest.sha256` records the SHA-256 of every imported file. Neither manifest
may be regenerated from implementation output.
