# Contributing

Untie is a pure-Go math renderer with a strict separation between source
languages, neutral IR, layout, fonts, and UI output. Contributions must preserve
that separation and must not present unimplemented behavior as working.

## Development

The Nix development shell pins Go and sets `CGO_ENABLED=0`:

```sh
nix develop
go version
```

The module deliberately limits external Go dependencies to its pinned pure-Go
font and text substrate. Do not add cgo, native libraries, system font
discovery, a browser, a JavaScript runtime, or an image rasterizer to the core.

## Required Checks

Run these before handing off work:

```sh
gofmt -l .
CGO_ENABLED=0 go vet ./...
CGO_ENABLED=0 go test ./...
go test ./internal/quality
CGO_ENABLED=0 go test -shuffle=on -count=2 ./...
CGO_ENABLED=0 go test -tags=layout_tex_integration ./internal/layout
```

`gofmt -l .` must print nothing. CI runs the race detector on a cgo-capable
host, while the required compatibility gate remains `CGO_ENABLED=0`.

For parser changes, run the relevant fuzz target for a short smoke test:

```sh
CGO_ENABLED=0 go test ./internal/tex \
  -run=^$ -fuzz=FuzzParseNoPanicAndValidIR -fuzztime=10s
```

For platform-sensitive changes, cross-build the package tree:

```sh
for target in linux/amd64 linux/arm64 darwin/arm64 windows/amd64 js/wasm; do
  GOOS=${target%/*} GOARCH=${target#*/} CGO_ENABLED=0 go build ./...
done
```

## Architecture

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
- Unsupported behavior must produce a visible diagnostic or error.
- Prefer a refactor when a workaround crosses a package boundary or duplicates a
  semantic rule.
- Add tests at the lowest layer that can prove the behavior. Add integration or
  fixture tests when the boundary behavior matters.

## Fixtures And Generated Data

Tracked test inputs live under `testdata`. They are immutable evidence for
reviewed behavior, not outputs that may be changed to hide a regression.

- Keep corpus counts, IDs, hashes, and classifications explicit.
- Never silently skip a case. Record a supported, diagnostic, unsupported, or
  reviewed-difference disposition.
- Never regenerate a golden from the implementation being tested.
- Keep source corpora and frame fixtures paired by ID.
- Convert fixture points only at the fixture boundary.
- Do not edit `internal/mathdata/*_generated.go` by hand.

After an intentional corpus review:

```sh
go test ./internal/quality -run '^TestDailySTEMCorpus$' \
  -args -update-daily-manifest
```

Baseline changes must be reviewed as data changes separate from behavior
changes where practical.

## Commits

Use one-line Conventional Commit subjects:

```text
feat(tex): support aligned environments
fix(layout): keep italic correction out of advance twice
test(fixture): verify frame IDs match corpus IDs
refactor(ir): split script placement from style state
docs: describe host font ownership
```

Use an imperative, specific subject and keep one meaningful change per commit.
Do not mix source behavior, broad formatting, fixture regeneration, and
documentation updates without a reason.

## License

Untie is dual-licensed under MIT or Apache-2.0. By submitting a contribution,
you agree that it may be distributed under either project license unless you
explicitly state otherwise.
