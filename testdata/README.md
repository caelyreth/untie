# Test Data

This directory contains reviewed inputs and expected results. It is not a
runtime dependency.

## Layout

- `daily-stem/` is the product-priority formula corpus. `manifest.json` pins
  case IDs, counts, review classifications, and a canonical content digest.
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
go run ./cmd/dailyreport -update-manifest
```

Normal verification uses:

```sh
CGO_ENABLED=0 go test ./...
```
