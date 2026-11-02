# Current Status

This document records the implementation state and supported scope at the
documentation snapshot. It must be updated when behavior changes. Planned scope
is not a compatibility claim.

## Snapshot

- Repository commit: `b879aefd610d86f4e1fad3992d5d5b455fc847a2`
- Documentation date: 2026-11-02
- Go toolchain: 1.27.3, as pinned by `flake.nix`
- Required build mode: `CGO_ENABLED=0`
- Last documented verification: `CGO_ENABLED=0 go test ./...` passed

The public API and IR remain pre-1.0. Breaking changes are allowed while the
vertical slice is completed, but semantic behavior and data formats still need
tests and migration notes.

## Implemented

### Public Foundations

- UTF-8 byte ranges, sorted diagnostics, stable diagnostic codes, and bounded
  resource limits in `source`.
- Fixed-point em and DIP arithmetic in `geometry`.
- Sealed neutral IR nodes, constructors, walking, canonical test encoding, and
  structural validation in `ir`.
- Immutable display-list types, validation, cloning, paint variants, and
  canonical test encoding in `display`.
- Parsing facade with dialect selection, TeX macro options, Typst preamble
  options, diagnostics, stable parse keys, cancellation checks, and a panic
  containment boundary.

### TeX Frontend

The bounded TeX frontend currently includes:

- UTF-8 lexing with comments, control words, control symbols, groups, scripts,
  primes, alignment tabs, and source spans;
- scoped `\def`, `\gdef`, `\let`, `\global\def`, `\newcommand`,
  `\renewcommand`, and `\providecommand` for supported forms;
- expansion, token, call-depth, nesting, and node budgets;
- core symbols, Greek letters, named operators, character classes, spacing,
  fractions, roots, scripts, limits, accents, fences, lines, cancel, text, and
  style commands represented by the current IR;
- matrix, array, cases, aligned, gathered, split, alignedat, and alignat parsing
  into table or multiline IR;
- recovery diagnostics for malformed groups, missing arguments, invalid UTF-8,
  unmatched environments, recursion, and resource limits;
- static KaTeX-derived parser cases, unit tests, recovery tests, and a fuzz
  target.

Unsupported TeX constructs are diagnosed. Current explicit limitations include
`\edef`/`\xdef`, delimited macro parameters, optional macro defaults, explicit
delimiter sizing commands, and some text-mode math constructs.

### Shared Metadata

`internal/mathdata` contains checked-in, generated tables for:

- Unicode math classes and Typst default class overrides;
- mathematical alphanumeric variants;
- literal TeX symbols;
- Typst symbol families and variants;
- Typst math shorthands;
- TeX and Typst named operators.

The generator records its input paths and hashes. Generation requires the
ignored reference checkout; normal builds and tests consume only checked-in
tables.

### Fixture Infrastructure

- Pinned NewCM font bytes, Typlet formulas and static frames, RaTeX core TeX
  cases, and selected KaTeX cases are tracked under `testdata/upstream`.
- Corpus manifests declare exact counts.
- SHA-256 manifests cover imported files.
- The fixture package decodes formulas, frames, fonts, diagnostics, and JSON
  Lines; converts UTF-16 fixture spans to UTF-8 byte spans; converts fixture
  points at the test boundary; runs corpora; and provides golden and path
  inspection helpers.

### Internal Layout Foundation

`internal/layout` contains a deterministic fixed-point layout core with:

- display, text, script, and script-script styles, normal and cramped;
- neutral math classes and automatic class spacing;
- rows, explicit spaces, glyphs, scripts, fractions, radicals, fences, lines,
  styles, colors, placeholders, and basic text through narrow ports;
- optional table and stretch ports;
- fragment bounds, output quantization, overflow measurement, cancellation, and
  resource limits;
- flattening to `display.Box` using mocked font ports in tests.

This code is not yet connected to the public `LayoutContext` API.

## Not Yet Implemented

The following are genuine gaps, not hidden behavior:

- `LayoutContext` currently returns an empty `display.Box`; public end-to-end
  formula layout is unavailable.
- `font.Load` copies font bytes and computes a font-set key but does not parse
  SFNT/TTC, cmap, CFF or TrueType outlines, GSUB, GPOS, or OpenType MATH data.
- There is no production font adapter from `font.FontSet` to the internal layout
  ports.
- The Typst frontend is a stub that diagnoses all non-empty input.
- No public path produces glyph outlines from a real font.
- The internal table layout requires a host-supplied extension port and is not a
  complete built-in table implementation.
- Text shaping, MATH variants and assemblies, glyph fallback, font matching, and
  production glyph caching are not implemented.
- No renderer-produced layout golden has been compared with the imported Typlet
  frame fixtures.
- The imported corpora are not yet a complete conformance runner with a status
  for every case.
- Break placement, display wrapping, inline source hits, accessibility output,
  equation numbering, tags, links, MathML, rasterization, and SVG export are not
  product features. SVG support is test-only.
- There is no project-wide license. See `THIRD_PARTY_NOTICES.md`.

## Scope Boundary

Untie targets rendered math formula semantics. It does not aim to execute a TeX
distribution, a full Typst document, packages, imports, file or network access,
arbitrary embedded code, general page layout, HTML/CSS layout, or platform text
systems.

Planned compatibility work should remain focused on formula constructs that
KaTeX or Typst math users reasonably expect. A planned construct becomes
supported only when parsing, lowering, layout, display output, diagnostics when
relevant, and tests all agree.

## Status Changes

When behavior changes, update this document in the same change or a directly
linked change. Do not remove a gap without pointing to tests or code that
establish the new behavior.
