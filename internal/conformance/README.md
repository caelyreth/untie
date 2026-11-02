# Conformance Reporting

This package evaluates the tracked TeX and Typst corpora against their frontend
adapters. It is test infrastructure and is not imported by the runtime rendering
pipeline.

The product-priority acceptance source is `testdata/daily-stem`, evaluated by
`internal/daily` and `cmd/dailyreport`. Upstream conformance is evidence for
syntax, diagnostics, and fidelity; it does not require implementing every rare
reference-project construct when the daily-STEM corpus does not need it.

## What A Run Checks

Every case is evaluated three times:

1. With `source.DefaultLimits()`.
2. Again with `source.DefaultLimits()` to prove deterministic canonical output
   and diagnostics.
3. With the distinct, tracked profile returned by `ExplicitTeXLimits()` or
   `ExplicitTypstLimits()`.

Every root must pass `ir.ValidateSource`, every source range must fall on UTF-8
code-point boundaries and inside the source, diagnostics must be valid and
deterministic, and the repeated/default/explicit runs must have identical
canonical IR and diagnostics. The default-limit observation determines the
reviewed status:

- `pass`: the case parses without error diagnostics.
- `diagnostic`: an upstream error case yields one of its reviewed diagnostic
  codes.
- `unsupported`: the frontend reports an explicit unsupported construct, or a
  reviewed scope exclusion classifies an otherwise out-of-scope diagnostic.
- `bug`: the frontend panics, times out, emits invalid IR, violates limit
  profile parity, recovers silently where a diagnostic is required, or rejects
  an in-scope formula.

Every result carries sorted source families such as `command:\frac`,
`environment:matrix`, `function:frac`, `field:arrow.r`, `feature:scripts`, and
`feature:alignment`.

## Reviewed Data

- `testdata/tex-policy.json` and `testdata/typst-policy.json` are reviewed input
  data. They define expected upstream diagnostic codes, explicit scope
  exclusions, and known frontend-gap reasons.
- `testdata/tex-baseline.json` and `testdata/typst-baseline.json` are generated
  baselines. They are committed so every behavior, diagnostic code, family,
  review disposition, detail, and count is visible in review.

The tests never rewrite these files. A changed case, an added corpus record, or
a changed diagnostic fails the comparison and requires an explicit review.
Unreviewed non-passing behavior also fails `Report.Validate`, and unused policy
rules are errors.

Current TeX baseline:

| Corpus | Cases |
|---|---:|
| RaTeX 0.1.15 core | 1,075 |
| Selected KaTeX 0.19.1 errors and parses | 36 |

The TeX baseline currently records 1,047 passes, 26 expected diagnostics, 38
unsupported cases, and 0 reviewed parser gaps. Unsupported cases are explicit
scope exclusions such as math font-size declarations, chemistry,
commutative diagrams, and document-layout commands; they are not relabeled
passes. All 16 explicit `\big...` delimiter commands are now in-scope:
standalone sized delimiters preserve their class and an absolute target policy
in neutral IR. The TeX adapter maps its four command tiers to 1.2em, 1.8em,
2.4em, and 3.0em; the neutral IR does not encode TeX tier names.

Current Typst baseline:

| Corpus | Cases |
|---|---:|
| Typlet 0.2.0 paired | 81 |
| Selected Typlet features | 29 |
| Selected Typlet code cases | 16 |
| Selected Typlet symbols | 30 |
| Selected Typlet syntax cases | 15 |

The Typst baseline records 156 passes, 3 expected upstream diagnostics, 4
explicit depth-limit scope exclusions, and 8 reviewed known gaps. Known gaps are
limited to unrestricted Typst code evaluation and parser-only syntax probes
outside the evaluable math subset. None are silently skipped; each has an exact
case rule and reason in `typst-policy.json`. Typst `lr(size:)` now lowers
default/100% to content sizing, non-default percentages to relative sizing, and
lengths to absolute sizing; the reviewed `big-sizes` record no longer reports a
frontend warning.

## Isolation

The baseline reports run every case in a fresh worker process with a deadline,
so an unexpected non-cooperative parser hang or fatal failure cannot poison the
suite. This isolation is defense in depth, not a substitute for termination
tests: the tracked TeX corpora are also parsed in-process with per-case deadlines
by `TestTeXCorpusTerminatesInProcess`. Benchmarks call the parser directly so
their throughput numbers describe parser work, not process startup.

## Commands

Run the package tests with the repository's Nix shell:

```sh
nix develop --command env CGO_ENABLED=0 go test ./internal/conformance/...
```

Run representative parser benchmarks:

```sh
nix develop --command go test ./internal/conformance -run '^$' -bench 'Parse(TeX|Typst)' -benchmem
```

Regenerate the TeX baseline only as a separate, explicit review action:

```sh
nix develop --command env CGO_ENABLED=0 go run ./internal/conformance/cmd/texreport \
  -root testdata/upstream \
  -policy internal/conformance/testdata/tex-policy.json \
  -case-timeout 500ms \
  -write internal/conformance/testdata/tex-baseline.json
```

Regenerate the Typst baseline only as a separate, explicit review action:

```sh
nix develop --command env CGO_ENABLED=0 go run ./internal/conformance/cmd/typstreport \
  -root testdata/upstream \
  -policy internal/conformance/testdata/typst-policy.json \
  -case-timeout 500ms \
  -write internal/conformance/testdata/typst-baseline.json
```

Both commands refuse to write when the report is invalid or contains
unexplained cases. Regeneration must be reviewed as a behavior change; passing
tests never regenerate a baseline.

This suite depends only on tracked `testdata/upstream` inputs. It does not need
the ignored `references/` checkout and does not compile or execute any upstream
reference project.
