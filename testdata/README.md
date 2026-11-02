# Test Data

This directory contains reviewed inputs and expected results. It is not a
runtime dependency.

## Layout

- `daily-stem/` is the product-priority formula corpus. `manifest.json` pins
  case IDs, counts, review classifications, and a canonical content digest.
- `conformance/` contains reviewed TeX and Typst conformance policies and
  baselines.
- `crosscheck/` contains the reviewed paired TeX/Typst crosscheck baseline.
- `tex/` contains additional reviewed TeX parser cases.
- `upstream/` contains immutable reference corpora, frames, fonts, and
  attribution used by conformance tests.

Package-local `testdata/fuzz` directories remain beside their fuzz targets
because the Go fuzzing tool discovers them there.

## Review Rules

- Do not update an expected result merely to make a failing implementation pass.
- Keep declared counts, IDs, hashes, and source/frame pairs exact.
- Do not silently skip a case. Record a supported, diagnostic, unsupported, or
  reviewed-difference disposition.
- Avoid regenerating a golden from the implementation under test.
- Convert fixture points and string offsets only at the fixture boundary.

## Corpus Updates

After an intentional daily-corpus review:

```sh
go test ./internal/quality -run '^TestDailySTEMCorpus$' \
  -args -update-daily-manifest
```

Conformance and crosscheck baselines are reviewed in the same way:

```sh
go test ./internal/quality -run '^TestTeXConformance$' \
  -args -update-tex-baseline
go test ./internal/quality -run '^TestTypstConformance$' \
  -args -update-typst-baseline
go test ./internal/quality -run '^TestPairedCrosscheck$' \
  -args -update-crosscheck-baseline
```

Normal verification uses:

```sh
CGO_ENABLED=0 go test ./...
```
