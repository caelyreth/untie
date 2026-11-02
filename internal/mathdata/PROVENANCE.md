# Generated Math Metadata

`internal/mathdata` contains mechanically transformed metadata from pinned,
permissively licensed upstream projects. It does not contain upstream runtime
code. Normal builds consume only the checked-in Go files.

## Source Pins

| Project | Version / commit | License | Files consumed |
|---|---|---|---|
| KaTeX | 0.19.1, `65d1e725206dbbd278704df641491cf877413b6b` | MIT | `src/symbols.ts`, `src/functions/op.ts`, `src/macros.ts` |
| Typst | 0.15.1, `84cca406aafe7f830ed3f37f1cc6f0c8b6b65a03` | Apache-2.0 | `crates/typst-syntax/src/ast.rs`, `crates/typst-library/src/math/op.rs` |
| Typlet | 0.2.0, `c3ebcac2db54ee123faaab8c437c83e7f66ec0a5` | MIT with Apache-2.0 notices | `src/generated/math-class.ts`, `src/utils/math-class.ts`, `src/utils/styling.ts`, `src/generated/symbols.ts` |
| codex | 0.3.0, upstream Typst pin `2e43ab9` | Apache-2.0 | Transitive source of Typlet's generated Typst symbol table |
| unicode-math-class | 0.1.0, MathClass.txt revision 15 | MIT or Apache-2.0 | Transitive source of Typlet's generated class runs |

The input files are read as static text. No upstream compiler or runtime is
invoked.

## Exact Input Hashes

| File | SHA-256 |
|---|---|
| `KaTeX/src/symbols.ts` | `88bf41a79569d9c860292049e897cbd12019dd43f7e3e0044e39ca5deb356b84` |
| `KaTeX/src/functions/op.ts` | `a04557b2f14cccd7927b8ea60cdf3f7df649abe52ed3b81748145538deb25a9d` |
| `KaTeX/src/macros.ts` | `13adf540091b6c4d4a31bcd65fe325b74beffc37f2ddb8017f52d024c73a1fdc` |
| `typst/crates/typst-syntax/src/ast.rs` | `595cea7702ab8dcc1272776f07564398527411157c8f2ebdd77b6b01c7b2b92e` |
| `typst/crates/typst-library/src/math/op.rs` | `8b4ee3ebcb085a4810b164d03f1eb0fbb4f2ce9b01224cd32ef4352d3631a1d3` |
| `Typlet/src/generated/math-class.ts` | `81697cfe65460d76fffcb834f87327c7e9f614aa7efff61effc318aab9f26bca` |
| `Typlet/src/utils/math-class.ts` | `c185f1bba6adc673ceff27ae5417b942767d2e2cd537a7a4e409870e2463c2e1` |
| `Typlet/src/utils/styling.ts` | `14fd8750bceb34b1b68d192b9e767e7e0af8db981e891b6fb15ce1548c971163` |
| `Typlet/src/generated/symbols.ts` | `229f1cb2b5892076ed14104be42adccbae055d72421c466063e150b0ee8a2d9d` |

Generated file headers repeat the exact source paths and hashes used for that
output.

## Transformations

The generator performs representation-preserving transformations:

- KaTeX literal no-argument math symbol registrations are decoded and sorted.
- Private-use KaTeX code points are excluded from public replacement text.
  Portable Unicode alternatives from `macros.ts` are retained where available.
- KaTeX zero-argument operator registrations are collected without expanding
  macros.
- Typst math shorthands and static operator registrations are decoded and
  sorted.
- Typlet class runs and Typst default-class overrides are decoded into compact
  Unicode ranges.
- Typlet symbol variant records, alphanumeric ranges, exceptions, and
  deprecation metadata are decoded and sorted.

No public spelling, portable code point, deprecation, class, or symbol value is
changed. Ordering is canonicalized for deterministic binary search.

## Reproduction

With a directory containing the pinned source trees:

```sh
go run ./internal/mathdata/gen \
  -sources /path/to/pinned-sources \
  -out internal/mathdata
```

The source root is an explicit maintainer input. Review the generated hash and
table diff before committing it. Do not update generated metadata merely to make
an unrelated parser or layout test pass.

## Attribution

Canonical upstream license and notice files are tracked beside the imported
fixtures under `testdata/upstream/licenses`.
