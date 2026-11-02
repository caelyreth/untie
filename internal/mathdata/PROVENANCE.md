# Provenance And Licensing

This package contains mechanically transformed metadata from pinned, permissively
licensed upstream projects. It does not contain code from those projects.

## Source Pins

| Project | Version / commit | License | Files consumed |
|---|---|---|---|
| KaTeX | 0.19.1, `65d1e725206dbbd278704df641491cf877413b6b` | MIT | `src/symbols.ts`, `src/functions/op.ts` |
| Typst | 0.15.1, `84cca406aafe7f830ed3f37f1cc6f0c8b6b65a03` | Apache-2.0 | `crates/typst-syntax/src/ast.rs`, `crates/typst-library/src/math/op.rs` |
| Typlet | 0.2.0, `c3ebcac2db54ee123faaab8c437c83e7f66ec0a5` | MIT with Apache-2.0 notices | `src/generated/math-class.ts`, `src/utils/math-class.ts`, `src/utils/styling.ts`, `src/generated/symbols.ts` |
| codex | 0.3.0, upstream Typst pin `2e43ab9` | Apache-2.0 | Transitive source of Typlet's generated Typst symbol table |
| unicode-math-class | 0.1.0, MathClass.txt revision 15 | MIT or Apache-2.0, used under MIT | Transitive source of Typlet's generated class runs |

The KaTeX and Typst commits exactly match `references/untie-plan/01-source-of-truth.md`.
The local checkout was used only as static text. No reference project was built
or executed.

## Exact Input Hashes

These SHA-256 values are also emitted into each generated file:

| File | SHA-256 |
|---|---|
| `KaTeX/src/symbols.ts` | `88bf41a79569d9c860292049e897cbd12019dd43f7e3e0044e39ca5deb356b84` |
| `KaTeX/src/functions/op.ts` | `a04557b2f14cccd7927b8ea60cdf3f7df649abe52ed3b81748145538deb25a9d` |
| `typst/crates/typst-syntax/src/ast.rs` | `595cea7702ab8dcc1272776f07564398527411157c8f2ebdd77b6b01c7b2b92e` |
| `typst/crates/typst-library/src/math/op.rs` | `8b4ee3ebcb085a4810b164d03f1eb0fbb4f2ce9b01224cd32ef4352d3631a1d3` |
| `Typlet/src/generated/math-class.ts` | `81697cfe65460d76fffcb834f87327c7e9f614aa7efff61effc318aab9f26bca` |
| `Typlet/src/utils/math-class.ts` | `c185f1bba6adc673ceff27ae5417b942767d2e2cd537a7a4e409870e2463c2e1` |
| `Typlet/src/utils/styling.ts` | `14fd8750bceb34b1b68d192b9e767e7e0af8db981e891b6fb15ce1548c971163` |
| `Typlet/src/generated/symbols.ts` | `229f1cb2b5892076ed14104be42adccbae055d72421c466063e150b0ee8a2d9d` |

The generated headers are the authoritative record for the files used by a
particular output.

## Transformations

The generator performs only representation-preserving transformations:

- KaTeX `defineSymbol(math, ...)` calls are selected as literal no-argument math
  symbols. JavaScript string escapes are decoded; commands are sorted.
- KaTeX's parser groups are retained in `TeXGroup`; normalized `Class` is a
  derived convenience mapping.
- KaTeX `type: "op"` function registrations with `numArgs: 0` are collected as
  operator metadata. Macro-specific symbol aliases and operands are not expanded.
- Typst `MathShorthand::LIST` entries are decoded and sorted.
- Typst's static `ops!` registrations are decoded and sorted.
- Typlet's generated class runs are decoded from its compact
  `(gap, length, class)` representation. Typst's explicit default-class override
  cases are decoded separately.
- Typlet's transcription of codex's named symbol variants is decoded from
  `name<TAB>=value<TAB>modifier=value...` records. Variant and binding
  deprecations are attached to the same generated records.
- Typlet's transcription of codex's alphanumeric ranges and exceptions is decoded
  into generated range data. Calligraphic and roundhand selectors are retained.

No spelling, code-point, deprecation, font-class, group, limit, or symbol value
is changed. Ordering is canonicalized for deterministic binary search.

## Excluded Data

- KaTeX text-mode symbols, macros, CSS, fonts, and metrics.
- Typst document semantics, evaluator behavior, arbitrary embedded code, fonts,
  package/import/file support, and layout data.
- Typst symbols outside the codex 0.3.0 data captured by pinned Typlet.
- Dynamic symbol modifiers, callable-symbol implementations, and runtime
  deprecation behavior.
- Unicode prose names and data not represented by the pinned tools.

Absence is not silently converted into support. Higher frontend layers must
report unsupported commands, symbols, and options.

## Required Notices

Full license texts are vendored under `licenses/`:

- `licenses/MIT-KaTeX.txt`
- `licenses/MIT-Typlet.txt`
- `licenses/Apache-2.0.txt`
- `licenses/NOTICE-Typst.txt`

Copyright and attribution notices:

- KaTeX: Copyright (c) 2013-2020 Khan Academy and other contributors.
- Typst: Copyright The Typst Project Developers.
- codex: Copyright The Typst Project Developers.
- Typlet: Copyright (c) 2026 dbcccc.

The Apache-2.0 material is used under the Apache License, Version 2.0. The
generated metadata is a modified representation and is not presented as an
unmodified Typst or codex distribution.
