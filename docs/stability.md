# API Stability Tiers

Untie is currently version `0.1.0` with layout revision `1`. This page defines
how these docs describe stability; it does not add a guarantee that is absent
from the code.

## Tier 1: Host Integration API

Primary packages:

- `github.com/caelyreth/untie`
- the host-facing types in `github.com/caelyreth/untie/display`
- `github.com/caelyreth/untie/font` for `Fonts`, `FontData`, and `FontSet`

Primary surface:

- `LoadFonts`, `NewEngine`, `NewEngine` results, `Engine.Layout`,
  `Engine.LayoutContext`;
- `Parse`, `ParseTeX`, `ParseTypst`, `Expr`;
- `Layout`, `LayoutContext`, `ClearLayoutEngineCache`;
- `Style`, `Box`, `Item`, `Paint`, `Hit`, `Break`, `LineBox`, `TagBox`, `Alt`;
- public error sentinels and diagnostic severities/codes.

Tier 1 is the recommended integration surface. Breaking changes are allowed
while the module is pre-1.0, but hosts should not need internal packages or
undocumented concrete layout logic.

## Tier 2: Advanced Data Contracts

Packages:

- `github.com/caelyreth/untie/ir`
- `github.com/caelyreth/untie/source`
- `github.com/caelyreth/untie/geometry`
- low-level introspection in `github.com/caelyreth/untie/font`

This tier supports:

- canonical IR inspection and caching;
- `RootSnapshot`, `ValidateSource`, `CloneFormula`, `WalkFormula`;
- `Engine.LayoutFormulaContext`;
- source ranges, diagnostic codes, and resource limits;
- exact glyph metadata, feature tags, MATH tables, and font identity.

These are public and intended to be usable, but they expose more of the data
model. Changes are more likely to require host code changes than a Tier 1
drawing integration.

## Tier 3: Diagnostic and Test Serialization

Functions:

- `ir.MarshalCanonical`;
- `display.Box.MarshalCanonical`;
- `display.Alt.MarshalCanonical`;
- corpus/report schemas under `testdata` and internal quality tooling.

The canonical serializers produce deterministic JSON for golden tests and
diagnostics. They are explicitly not stable cross-language protocols and not
the final public interchange format.

Use them for:

- regression tests;
- local debugging;
- deterministic snapshots.

Do not use them as an unversioned external wire protocol.

## Internal Packages

Anything under an `internal/` path is implementation detail:

- `internal/tex`;
- `internal/typst`;
- `internal/engine`;
- `internal/layout`;
- `internal/alt`;
- `internal/mathdata`;
- `internal/quality`.

Go's import rules prevent normal external modules from importing them. The
separate `examples/mygo` module is additionally checked to reject any Untie
internal import. Do not build a host against internal files even when working
inside the repository.

## Version and Cache Compatibility

`untie.Version()` combines a library version and layout revision:

```text
0.1.0+layout.1
```

The package-level engine cache includes that whole string. A layout behavior
change that invalidates prepared engine assumptions increments the layout
revision.

Font identity is separate and collision-resistant:

- `FontSet.KeySHA256()` for full identity;
- `FontSet.Key()` for a 64-bit convenience key.

Expression identity is convenience-only:

- `Expr.Key()` is 64-bit;
- it includes source, dialect, mode, version, limits, and TeX macros;
- it is not cryptographic.

## Diagnostic Codes

The source package documents diagnostic codes as stable semantic identifiers.
New codes may be added, but an existing code should not acquire a contradictory
meaning. Match with `Diagnostic.Code`, not message text.

Messages are human-readable and may improve. `Args` and `Hints` are supporting
metadata, not a stable localization format.

## Font IDs

Host-supplied face IDs are part of the public cache and glyph-reference
contract:

- IDs must be nonblank and unique within a `FontSet`;
- `ItemGlyphRef.FontID` is the exact ID supplied by the host;
- changing an ID changes the font-set SHA-256 identity.

Treat a face ID as a persisted cache identifier. Changing it intentionally
invalidates host caches.

## JSON and Report Schemas

Tracked corpus and report files have internal schema versions for repository
tests. Those versions do not automatically define a public application
protocol.

If a host persists canonical IR or reports, add its own application-level
version, migration, and validation layer.

## Compatibility Checklist

- Import only root and documented public packages.
- Use `errors.Is` for classified layout errors.
- Do not depend on diagnostic message spelling.
- Do not parse canonical JSON as a long-lived external format.
- Pin the Untie version used by persisted caches.
- Recompute font keys after changing bytes, IDs, indexes, or fallback order.
- Use `Version()` rather than assuming two binary versions share layout
  behavior.

## Evidence

- Version: [`version.go`](../version.go)
- Root public aliases: [`api.go`](../api.go)
- Public packages: [`go.mod`](../go.mod)
- Canonical serialization disclaimers:
  [`ir/canonical.go`](../ir/canonical.go),
  [`display/canonical.go`](../display/canonical.go)
- Diagnostic code contract: [`source/diagnostic.go`](../source/diagnostic.go)
- Architecture allowlist:
  [`internal/quality/architecture_test.go`](../internal/quality/architecture_test.go)

