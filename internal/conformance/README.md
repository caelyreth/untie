# TeX Conformance Reporting

This package evaluates the tracked RaTeX and selected KaTeX corpora against
`internal/tex.Parse`. It is test infrastructure and is not imported by the
runtime rendering pipeline.

## What A Run Checks

Every case is parsed twice:

1. With `source.DefaultLimits()`.
2. With the distinct, tracked profile returned by `ExplicitTeXLimits()`.

Both roots must pass `ir.Validate`, have identical canonical IR, and produce
identical diagnostics. The default-limit observation determines the reviewed
status:

- `pass`: the case parses without error diagnostics.
- `diagnostic`: an upstream error case yields one of its reviewed diagnostic
  codes.
- `unsupported`: the frontend reports an explicit unsupported construct, or a
  reviewed scope exclusion classifies an otherwise out-of-scope diagnostic.
- `bug`: the frontend panics, times out, emits invalid IR, violates limit
  profile parity, recovers silently where a diagnostic is required, or rejects
  an in-scope formula.

Every result carries sorted source families such as `command:\frac`,
`environment:matrix`, `feature:scripts`, and `feature:alignment`.

## Reviewed Data

- `testdata/tex-policy.json` is reviewed input data. It defines expected KaTeX
  diagnostic codes, explicit scope exclusions, and known parser-gap reasons.
- `testdata/tex-baseline.json` is the generated baseline report. It is committed
  so every behavior, diagnostic code, family, review disposition, and count is
  visible in review.

The test never rewrites either file. A changed case, an added corpus record, or
a changed diagnostic fails the comparison and requires an explicit review.
Unreviewed non-passing behavior also fails `Report.Validate`.

Current baseline:

| Corpus | Cases |
|---|---:|
| RaTeX 0.1.15 core | 1,075 |
| Selected KaTeX 0.19.1 errors and parses | 36 |

The baseline currently records 535 passes, 18 expected diagnostics, 19
unsupported cases, and 539 reviewed parser gaps. The large gap count is
intentional: the report exposes incomplete TeX command and environment
coverage rather than relabeling in-scope omissions as unsupported.

Notable known gap families are unimplemented symbol and command commands,
unsupported table environments, explicit text-mode accents and commands that
nonterminate in the current frontend, silent recovery for selected KaTeX error
cases, and a few diagnostics emitted with the wrong semantic code.

## Isolation

The TeX frontend currently contains nonterminating text-mode inputs. A Go
goroutine cannot be forcibly stopped, so `ProcessFrontend` runs every case in a
fresh worker process with a deadline. This contains hangs and crashes without
weakening the conformance gate. Benchmarks call `tex.Parse` directly so their
throughput numbers describe parser work, not process startup.

## Commands

Run the package tests with the repository's Nix shell:

```sh
nix develop --command env CGO_ENABLED=0 go test ./internal/conformance/...
```

Run representative parser benchmarks:

```sh
nix develop --command go test ./internal/conformance -run '^$' -bench ParseTeX -benchmem
```

Regeneration is a separate, explicit review action:

```sh
nix develop --command env CGO_ENABLED=0 go run ./internal/conformance/cmd/texreport \
  -root testdata/upstream \
  -policy internal/conformance/testdata/tex-policy.json \
  -case-timeout 500ms \
  -write internal/conformance/testdata/tex-baseline.json
```

The command refuses to write when the report is invalid or contains
unexplained cases. Regeneration must be reviewed as a behavior change; passing
tests never regenerate the baseline.

This suite depends only on tracked `testdata/upstream` inputs. It does not need
the ignored `references/` checkout and does not compile or execute any upstream
reference project.
