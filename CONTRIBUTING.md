# Contributing

Untie is a pure-Go math renderer with a strict separation between source
languages, neutral IR, layout, fonts, and UI output. Contributions must preserve
that separation and must not present planned behavior as working behavior.

## Development Environment

The Nix development shell pins Go and sets `CGO_ENABLED=0`:

```sh
nix develop
go version
```

The module deliberately limits external Go dependencies to the pinned pure-Go
font/text substrate recorded in `go.mod` and `THIRD_PARTY_NOTICES.md`. Do not
add cgo, native libraries, system font discovery, a browser, a JavaScript
runtime, or an image rasterizer to the core.

## Required Checks

Run these from a clean checkout before handing off work:

```sh
gofmt -l .
CGO_ENABLED=0 go vet ./...
CGO_ENABLED=0 go test ./...
CGO_ENABLED=0 go test -shuffle=on -count=2 ./...
CGO_ENABLED=0 go test ./internal/fixture
CGO_ENABLED=0 go run ./cmd/dailyreport
CGO_ENABLED=0 go test -tags=layout_tex_integration ./internal/layout
```

`gofmt -l .` must print nothing. Formatting is not only a style preference; CI
fails when it is dirty.

Run the TeX fuzz target for parser changes:

```sh
CGO_ENABLED=0 go test ./internal/tex \
  -run=^$ -fuzz=FuzzParseNoPanicAndValidIR -fuzztime=10s
```

Run a cross-build matrix for platform-sensitive changes:

```sh
for target in \
  linux/amd64 linux/arm64 darwin/arm64 windows/amd64 js/wasm
do
  GOOS=${target%/*} GOARCH=${target#*/} CGO_ENABLED=0 go build ./...
done
```

`go test -race` can be run on a cgo-capable development host, but race mode is
not the pure-Go compatibility gate. The required build and test path is
`CGO_ENABLED=0`.

## Reference Projects

The ignored `references/` directory contains static checkouts used for source
study. Never compile, install, invoke, or run KaTeX, Typst, Typlet, or RaTeX as
part of development, generation, tests, or CI.

Allowed:

- reading source and tests with normal file tools;
- checking a checkout's `HEAD` or tag with Git;
- hashing pinned source files;
- decoding tracked JSON, JSON Lines, and font metadata;
- comparing manually transcribed behavior with pinned tests.

Not allowed:

- running `npm`, `pnpm`, `cargo`, `typst`, `node`, or an upstream test harness;
- generating new expected output with an upstream renderer;
- adding network access, a container image, or a submodule that runs an
  upstream project;
- treating a screenshot, DOM tree, or reference implementation helper as
  unconditional product truth.

If a behavior cannot be established statically, mark it unverified and add a
test only after a pinned textual source or fixture supports it.

## Fixture Policy

Fixtures under `testdata/upstream` are immutable external evidence. They are not
generated build products.

`testdata/daily-stem` is the product-priority acceptance corpus. Its manifest
pins exact IDs, counts, pair classifications, and tracked issues. Add or change
a daily case only with an intentional acceptance decision; do not weaken a case
to make an implementation pass.

Rules:

- Copy only the records needed by tests.
- Record project, tag, full commit, source path, transformation, and license.
- Add or update the SHA-256 manifest for every imported file.
- Keep declared corpus counts exact.
- Keep source corpus and frame fixture IDs paired.
- Never silently skip a case. Use an explicit classification such as
  supported, supported-with-diagnostic, unsupported-by-scope, invalid-by-spec,
  or known-difference.
- Never regenerate a golden from the implementation being tested.
- Never update a golden merely to make a failing implementation pass.
- Treat a fixture update as a reviewed data change, separate from a behavior
  change where practical.
- Convert Typlet UTF-16 offsets only in `internal/fixture`; product packages use
  UTF-8 byte spans.
- Convert fixture points only at the fixture boundary; product geometry is
  fixed-point ems internally and DIPs at the public display boundary.

After changing fixtures:

```sh
CGO_ENABLED=0 go test ./internal/fixture
CGO_ENABLED=0 go test ./...
```

Update `testdata/upstream/ATTRIBUTION.md`, the manifests, and
`THIRD_PARTY_NOTICES.md` when licenses or imported material change.

## Generated Data

Do not edit `internal/mathdata/*_generated.go` by hand. With the pinned ignored
checkout available:

```sh
nix develop --command go generate ./internal/mathdata
```

The generator reads only the files listed in
`internal/mathdata/PROVENANCE.md`. It must never execute an upstream project.
Review generated input hashes and semantic table diffs before committing.

Normal builds and CI consume checked-in tables and must not require
`references/`.

## Code And Architecture Rules

- Put source-language behavior in `internal/tex` or `internal/typst`.
- Put neutral semantics in `ir`; never add a dialect AST, glyph ID, or measured
  font metric there.
- Do not add a dialect switch to `internal/layout` or `display`.
- Keep font parsing and shaping behind narrow ports; layout must not parse font
  files.
- Keep host UI types, DOM, GPU, image, and native text dependencies outside the
  core.
- Use fixed-point arithmetic for internal geometry and quantize at the display
  boundary.
- Preserve UTF-8 byte ranges, source origins, cancellation checks, and resource
  limits.
- Unsupported behavior must produce a visible diagnostic or error. Silent
  approximation is not acceptable.
- Prefer a refactor when a workaround crosses a package boundary or duplicates a
  semantic rule.
- Add tests at the lowest layer that can prove the behavior. Add integration or
  fixture tests when the boundary behavior matters.

Pure-Go portability, deterministic output, and recoverable parsing are product
requirements, not optional cleanup.

## Commit Style

Use one-line Conventional Commit subjects:

```text
feat(tex): support aligned environments
fix(layout): keep italic correction out of advance twice
test(fixture): verify frame IDs match corpus IDs
docs: record upstream fixture provenance
refactor(ir): split script placement from style state
```

Use an imperative, specific subject. Keep one meaningful change per commit.
Avoid mixing source behavior, broad formatting, fixture regeneration, and
documentation updates in one commit.

Branch names use `codex/<topic>` by default. Merge work with a merge commit so
agent and worktree history is preserved:

```sh
git switch main
git merge --no-ff codex/<topic>
```

Do not squash or rebase a shared agent branch unless the owner explicitly asks.
Do not force-push a branch another worker may be using. Do not amend a commit
unless the owner requests it.

## Worktree And Agent Coordination

Use one worktree and one branch per independent workstream:

```sh
git worktree add ../untie-<topic> -b codex/<topic> <base-commit>
```

Before starting:

- record the base commit;
- declare owned paths;
- check `git status --short --branch`;
- read the relevant architecture and status documents.

While working:

- do not edit files outside the declared ownership without coordination;
- do not overwrite or discard another worker's changes;
- do not use `git reset --hard`, `git checkout --`, or a broad clean command to
  hide unexpected changes;
- if an unexplained change appears in an owned file, stop and inspect it;
- if an unexplained change appears outside ownership, leave it alone and report
  it;
- keep generated data, fixtures, and source changes attributable to one pin.

Before handoff, report:

- branch and base commit;
- commits;
- changed paths;
- exact commands run and results;
- known unsupported cases;
- fixture or license changes;
- any remaining risk or unverified assumption.

The integration owner verifies the branch independently, checks for concurrent
changes, and merges with history intact.

## Review Gates

Every non-trivial change must pass these gates.

| Gate | Required evidence |
|---|---|
| Scope | The change implements or documents an accepted construct; no planned feature is claimed as complete |
| Architecture | Import direction and package ownership remain valid; architecture test passes |
| Correctness | Focused tests plus `CGO_ENABLED=0 go test ./...`; malformed input is covered where relevant |
| Determinism | No map-order, environment, clock, host font, pointer, or uninitialized-state variation enters output |
| Limits | New expansion, recursion, node, table, outline, or display loops have bounded behavior and tests |
| Units | Design units, em, DIP, and fixture points cross named adapters only |
| Fixtures | Counts, IDs, hashes, classifications, and transformations are explicit |
| Licensing | New source, fixtures, fonts, generated data, and notices are recorded in `THIRD_PARTY_NOTICES.md` and tracked license paths |
| Documentation | `docs/status.md` remains accurate when support or gaps change |

Reviewers should reject:

- a new dialect branch in layout;
- a font ID or measured width in semantic IR;
- a frontend importing layout;
- an unscoped dependency in the module graph;
- a silent fallback for unsupported syntax;
- a test that compares implementation output to itself;
- a golden update without a reviewed upstream or compatibility reason;
- an "all tests pass" claim when the relevant corpus cases were skipped.

## Definition Of Done

A feature is complete only when all applicable layers agree:

1. The source syntax is accepted or rejected with the documented diagnostic.
2. Lowering produces valid neutral IR with correct spans and style semantics.
3. Layout uses font metrics through the defined ports.
4. Display output is valid and deterministic.
5. Tests cover success, failure, and limits.
6. Unsupported cases are classified rather than omitted.
7. Documentation and provenance are updated.

An implementation that only parses a construct is not full support.
