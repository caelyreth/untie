# Current Status

This document records the implementation state and supported scope at the
documentation snapshot. It must be updated when behavior changes. Planned scope
is not a compatibility claim.

## Snapshot

- Repository state: post engine hardening, generated/fixture import, and
  TeX/Typst conformance baselines
- Documentation date: 2026-11-02
- Go toolchain: 1.27.3, as pinned by `flake.nix`
- Required build mode: `CGO_ENABLED=0`
- Last documented verification: pure-Go tests, race tests, vet, lint, tagged TeX
  layout integration, fuzz smoke runs, and the cross-build matrix passed

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
- Immutable display-list types, validation, cloning, paint variants, source
  hit and break contracts, and canonical test encoding in `display`.
- Expression-level alternative text through `Expr.Alt` and `Expr.AltContext`,
  with deterministic plain text, speech, bounded validated fallbacks, and a
  public error contract. Public layout populates `display.Box.Alt` after
  geometry succeeds under the per-field stricter intersection of Engine and
  expression resource limits, so tighter Engine limits may produce a bounded
  fallback that differs from `Expr.Alt`; MathML remains unimplemented.
- Layout emits source-linked hit regions for atoms, groups, scripts, fraction
  parts, fences, and table cells from their actual placed fragment extents.
- Inline layout emits deterministic top-level relation, binary, and explicit
  break opportunities with post-operator glue folded into the preceding piece.
- Parsing facade with dialect selection, TeX macro options, Typst preamble
  options, diagnostics, stable parse keys, cancellation checks, and a panic
  containment boundary.
- Public end-to-end layout through `internal/engine`, with real font, IR,
  layout, and display-list validation.
- Caller-owned reusable `Engine` values and a bounded eight-entry convenience
  cache keyed by complete font identity, effective limits, version, and cache
  schema. `ClearLayoutEngineCache` releases convenience-path retention without
  affecting caller-owned engines.

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

### Font Backend

`font` provides:

- SFNT and TTC face selection from copied host bytes;
- cmap and Unicode variation-sequence mapping;
- OpenType MATH constants, italic corrections, accent attachments, corner
  kerns, variants, assemblies, and connector metadata in design units;
- `dtls`/`ssty` math shaping in canonical order;
- quadratic and cubic glyph outlines;
- checked malformed-input rejection and a concurrency-safe outline cache;
- exact conformance against the tracked NewCM font fixture, with a small,
  explicitly listed CFF legacy-composition exception.

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
- rows, explicit spaces, glyphs, scripts, fractions, radicals, fences, accents,
  cancel, lines, boxes, phantom, styles, colors, placeholders, and text through
  narrow ports;
- baseline-origin placement verified with real NewCM outputs;
- signed explicit spacing, binary lookahead/demotion, and MATH corner kerning;
- optional table and stretch ports;
- fragment bounds, output quantization, overflow measurement, cancellation, and
  resource limits;
- flattening to `display.Box` using real and mocked font ports;
- deterministic greedy display wrapping at top-level neutral break boundaries,
  with independently flattened, baseline-offset `LineBox` results.

`internal/engine` adapts `font.FontSet` to layout, performs rune/class-aware
variant selection, multi-glyph operator shaping, text tracking and fallback,
and MATH variant/assembly selection.

### Typst Frontend

The bounded Typst math frontend includes:

- UTF-8 byte-span lexing and a private AST;
- math identifiers, shorthands, scripts, primes, fractions, roots, alignment,
  rows, and math calls;
- named and spread arguments for core math functions;
- Typst's one-pair fraction operand deparenthesization, exact built-in spacing,
  source-glyph interword spaces, grouped-prime precomposition, structural
  spreader metadata, natural accent fitting, and contextual or display-only
  limit placement;
- matrices, vectors, cases, accents, lines, cancel, text, boxes, styles, and
  alphabets, including horizontal and vertical matrix augmentation lines;
- a deterministic evaluator subset for values required by math calls;
- source, token, nesting, node, call-depth, table-cell, and text limits;
- explicit diagnostics for unsupported code, parameters, and scope boundaries.

### Conformance Baselines

- 1,111 TeX cases from RaTeX and selected KaTeX inputs are classified with zero
  unexplained cases.
- 171 selected Typst cases are classified with zero unexplained cases; the
  current split is 156 pass, 3 expected diagnostic, 4 unsupported, and 8
  reviewed gaps.
- All 81 paired Typst/TeX formulas are classified with zero unexplained cases.
- The committed paired baseline records 22 full passes, 55 semantic gaps, 0
  layout gaps, 2 static-frame gaps, and 2 documented pair differences, with
  zero unexplained cases.

## Not Yet Implemented

The following are genuine gaps, not hidden behavior:

- The built-in table and multiline layout paths are implemented; custom table
  providers remain optional extensions.
- Paired Typst/TeX semantic and geometry conformance still has substantial
  reviewed gaps.
- TeX conformance records no reviewed parser gaps; remaining non-passes are
  reviewed upstream diagnostics or explicit unsupported scope exclusions.
- Typst conformance still records reviewed evaluator and parser-only syntax
  gaps.
- Some MATH assemblies and metric differences remain within documented fixture
  tolerances; comparison against every static Typlet frame is not yet green.
- Package-level convenience layout uses a bounded process-wide engine cache;
  callers that need explicit lifetime control should use `NewEngine`.
- Inline break metadata, alternative text, and width-constrained display line
  boxes are implemented. Re-layout split pieces, equation numbering, tags,
  links, MathML, rasterization, and SVG export are not product features. SVG
  support is test-only.
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
