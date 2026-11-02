# Third-Party Notices

Untie's original source and documentation are dual-licensed under the MIT
License or Apache License 2.0, at your option. See `LICENSE-MIT`,
`LICENSE-APACHE`, and `NOTICE`. Those licenses do not replace or relicense the
third-party material recorded here.

Third-party material remains under its own license. Do not remove or rewrite a
third-party notice when redistributing the repository or a binary that contains
the material.

## Summary

| Component | Use in this repository | Pin | License | Tracked notice |
|---|---|---|---|---|
| KaTeX | TeX behavior reference; generated symbol/operator metadata; selected parser cases | 0.19.1, `65d1e725206dbbd278704df641491cf877413b6b` | MIT | `testdata/upstream/licenses/KaTeX-MIT.txt`; `internal/mathdata/licenses/MIT-KaTeX.txt` |
| Typst | Typst behavior reference; generated metadata; source of Typlet fixture output | 0.15.1, `84cca406aafe7f830ed3f37f1cc6f0c8b6b65a03` | Apache-2.0 | `testdata/upstream/licenses/Typst-Apache-2.0.txt`; `testdata/upstream/licenses/Typst-NOTICE.txt`; `internal/mathdata/licenses/NOTICE-Typst.txt` |
| Typlet | Static formula, frame, and font-metric fixtures; generated metadata input | 0.2.0, `c3ebcac2db54ee123faaab8c437c83e7f66ec0a5` | MIT; generated Typst output also carries Apache-2.0 notices | `testdata/upstream/licenses/Typlet-MIT.txt`; `internal/mathdata/licenses/MIT-Typlet.txt` |
| RaTeX | Core TeX formula case inventory | 0.1.15, `f0ef2cf7c15d85c41743d4bd2df19759cf0dfee3` | MIT | `testdata/upstream/licenses/RaTeX-MIT.txt` |
| New Computer Modern Math | Exact NewCM Book font used by static frame fixtures | `typst-assets` `94dcb990edb36311ef85162baa26e4a8205e0bf0` | GUST Font License 1.0, based on LPPL 1.3c or later | `testdata/upstream/licenses/typst-assets-NOTICE.txt` |
| `typst-assets` | Source distribution of the tracked NewCM font and its notices | `94dcb990edb36311ef85162baa26e4a8205e0bf0` | Asset-specific; NewCM is under GUST 1.0 | `testdata/upstream/licenses/typst-assets-NOTICE.txt` |
| codex | Transitive source of generated Typst symbol metadata | 0.3.0, Typst revision `2e43ab9` | Apache-2.0 | No standalone full text tracked; see owner actions below |
| unicode-math-class | Transitive source of generated Unicode math-class runs | 0.1.0, MathClass.txt revision 15 | MIT or Apache-2.0; recorded use is MIT | No standalone full text tracked; see owner actions below |
| `go-opentype/opentype` | Pure-Go OpenType container, metadata, MATH, and GSUB parsing | pseudoversion `2c8922cb87c69d7555957e90370c19f890758b36` | BSD-3-Clause | `licenses/go-opentype-BSD-3-Clause.txt` |
| `go-text/typesetting` | Pure-Go Type 2/CFF and TrueType glyph outline extraction | `v0.3.5` | Unlicense OR BSD-3-Clause | `licenses/go-text-typesetting-Unlicense-OR-BSD-3-Clause.txt` |
| `dsnet/compress` | Transitive WOFF2/brotli support through `go-opentype` | `v0.0.2-0.20230904184137-39efe44ab707` | BSD-3-Clause | `licenses/dsnet-compress-BSD-3-Clause.txt` |
| `golang.org/x/image` | Transitive font geometry dependency | `v0.47.0` | BSD-3-Clause | `licenses/golang-x-image-BSD-3-Clause.txt`; `licenses/golang-x-image-PATENTS.txt` |

## Text, Metadata, And Fixtures

### KaTeX

Copyright (c) 2013-2020 Khan Academy and other contributors.

Untie uses pinned KaTeX behavior as the TeX semantic reference. Generated
metadata is mechanically derived from selected KaTeX symbol and operator
registrations. Tracked test cases are a normalized selection from KaTeX's
parser/error tests. The KaTeX DOM, CSS, JavaScript runtime, and font stack are
not product dependencies.

MIT license:

- `testdata/upstream/licenses/KaTeX-MIT.txt`
- `internal/mathdata/licenses/MIT-KaTeX.txt`

### Typst

Copyright The Typst Project Developers.

Untie uses pinned Typst behavior as the Typst math reference. Typst is also the
source of the frame output captured by Typlet's committed fixtures. Pinned
Typst syntax and operator data appear in generated metadata. The Apache-2.0
NOTICE must be retained with distributions that include this material.

Apache-2.0 license:

- `testdata/upstream/licenses/Typst-Apache-2.0.txt`
- `internal/mathdata/licenses/Apache-2.0.txt`

Notice:

- `testdata/upstream/licenses/Typst-NOTICE.txt`
- `internal/mathdata/licenses/NOTICE-Typst.txt`

### Typlet

Copyright (c) 2026 dbcccc.

Untie tracks a selected subset of Typlet's formula corpora, static frames, and
NewCM font metric dump. Typlet fixture output is generated from Typst, so the
Apache-2.0 Typst notices are retained in addition to Typlet's MIT license.
Generated Typlet class and symbol transcriptions are also represented in
`internal/mathdata`.

MIT license:

- `testdata/upstream/licenses/Typlet-MIT.txt`
- `internal/mathdata/licenses/MIT-Typlet.txt`

### RaTeX

Copyright erweixin.

Untie tracks RaTeX's 1,075-line core TeX test inventory. RaTeX chemistry,
proof-tree cases, PNG goldens, fonts, and other implementation files are not
imported.

MIT license:

- `testdata/upstream/licenses/RaTeX-MIT.txt`

### Selected Case Transformations

The KaTeX selection is hand-transcribed and normalized. The RaTeX core case file
is a byte-for-byte copy. Typlet selected records preserve original record order
and values; selection is recorded in
`testdata/upstream/manifests/typlet-selection.json`. Detailed transformations
are recorded in `testdata/upstream/ATTRIBUTION.md`.

## New Computer Modern Math

The tracked font is New Computer Modern Math Book:

- file: `testdata/upstream/fonts/NewCMMath-Book.otf`
- SHA-256:
  `2ea09ebc9167b1e1a66f31390dc917f2d4004ecfca72d51b28010e4ad6becd95`
- author attribution: Antonis Tsolomitis
- source: `typst-assets` revision
  `94dcb990edb36311ef85162baa26e4a8205e0bf0`

The GUST Font License permits distribution and modification under the LaTeX
Project Public License, version 1.3c or later, with the GUST conditions. The
GUST notice requests, but does not legally require, that derived font works be
renamed and accompanied by a manifest. The tracked file is presented as an
unmodified test asset from `typst-assets`.

The full GUST Font License text and the surrounding `typst-assets` notices are
in `testdata/upstream/licenses/typst-assets-NOTICE.txt`. NewCM is not
relicensed by this project.

## Generated Data Provenance

`internal/mathdata` contains generated, mechanically transformed metadata. It
contains data derived from KaTeX, Typst, Typlet, codex, and unicode-math-class.
It is not presented as an unmodified upstream release.

The authoritative record of exact input files, hashes, transformations,
exclusions, and copyright notices is:

- `internal/mathdata/PROVENANCE.md`
- generated file headers in `internal/mathdata/*_generated.go`

The generator source is `internal/mathdata/gen`. Normal builds and tests use the
checked-in tables and do not require the ignored `references/` checkout.

## Full License Locations

Complete tracked license texts:

```text
testdata/upstream/licenses/KaTeX-MIT.txt
testdata/upstream/licenses/RaTeX-MIT.txt
testdata/upstream/licenses/Typlet-MIT.txt
testdata/upstream/licenses/Typst-Apache-2.0.txt
testdata/upstream/licenses/Typst-NOTICE.txt
testdata/upstream/licenses/typst-assets-NOTICE.txt
internal/mathdata/licenses/Apache-2.0.txt
internal/mathdata/licenses/MIT-KaTeX.txt
internal/mathdata/licenses/MIT-Typlet.txt
internal/mathdata/licenses/NOTICE-Typst.txt
licenses/dsnet-compress-BSD-3-Clause.txt
licenses/go-opentype-BSD-3-Clause.txt
licenses/go-text-typesetting-Unlicense-OR-BSD-3-Clause.txt
licenses/golang-x-image-BSD-3-Clause.txt
licenses/golang-x-image-PATENTS.txt
```

`testdata/upstream/manifest.sha256` records the hashes of the fixture-side
license files and data. License files under `internal/mathdata` are covered by
the generated-data provenance and repository review. Files under `licenses/`
are verbatim copies from the pinned Go module cache; upgrades must refresh and
review them with the dependency version.

## Owner Confirmation Required

The following release-review items do not change the dual-license grant stated
in `README.md`. This document does not resolve them:

1. Decide the contribution licensing model, such as a Developer Certificate of
   Origin, CLA, or another written grant.
2. Confirm whether the Apache-2.0 NOTICE obligations are satisfied by the two
   tracked copies covering generated metadata and Typst-derived fixture output.
3. Track complete codex and unicode-math-class license texts, or document why
   their current provenance notices are sufficient.
4. Confirm GUST/LPPL obligations for distributing the exact NewCM font and its
   manifest. If a modified, subset, or embedded derived font is later shipped,
   re-review the GUST naming and source-offer requirements.
5. Confirm the `typst-assets` revision and asset licensing against the upstream
   repository, because that checkout is not available locally.
6. Confirm whether hand-transcribed KaTeX cases and normalized Typlet selections
   need any additional copyright or modification notice.
7. Confirm whether any RaTeX test input contains third-party TeX material that
   requires a separate attribution.
