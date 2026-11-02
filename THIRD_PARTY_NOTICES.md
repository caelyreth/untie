# Third-Party Notices

Untie does not currently declare a project-wide license. This file does not
grant a license for Untie's original source, documentation, generated code, or
distribution. It records third-party material and points to the complete
licenses and notices that are tracked with that material.

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
```

`testdata/upstream/manifest.sha256` records the hashes of the fixture-side
license files and data. License files under `internal/mathdata` are covered by
the generated-data provenance and repository review.

## Owner Confirmation Required

The following facts require project-owner or legal review before public
distribution. This document does not resolve them:

1. Select a project-wide license and copyright holder, or explicitly decide to
   keep the original project unlicensed.
2. Decide the contribution licensing model, such as a Developer Certificate of
   Origin, CLA, or another written grant.
3. Confirm whether the Apache-2.0 NOTICE obligations are satisfied by the two
   tracked copies covering generated metadata and Typst-derived fixture output.
4. Track complete codex and unicode-math-class license texts, or document why
   their current provenance notices are sufficient.
5. Confirm GUST/LPPL obligations for distributing the exact NewCM font and its
   manifest. If a modified, subset, or embedded derived font is later shipped,
   re-review the GUST naming and source-offer requirements.
6. Confirm the `typst-assets` revision and asset licensing against the upstream
   repository, because that checkout is not available locally.
7. Confirm whether hand-transcribed KaTeX cases and normalized Typlet selections
   need any additional copyright or modification notice.
8. Confirm whether any RaTeX test input contains third-party TeX material that
   requires a separate attribution.
9. Decide whether documentation contributions and examples are under the same
   license as source, once a project license is selected.

Until those decisions are recorded, do not describe Untie as licensed under MIT,
Apache-2.0, GPL, or any other project-wide license.
