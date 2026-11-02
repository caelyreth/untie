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

## Corpus Manifest

`manifests/corpora.json` records the exact loadable corpus paths and counts.
`manifest.sha256` records the SHA-256 of every imported file. Neither manifest
may be regenerated from implementation output.
