# Performance

Untie's tracked performance target is the complete public geometry pipeline:
parse, validate, lay out, validate the output box, and return a drawable
display list. Accessibility text is generated only when
`Style.GenerateAlt` is set, so it is not part of the default benchmark.
Font loading and corpus loading are setup, not part of the timed operation.

Run benchmarks on an otherwise idle machine and compare repeated runs on the
same hardware and Go toolchain. Do not compare isolated numbers from different
processors or with different corpus revisions.

## Whole-Corpus Benchmark

```sh
CGO_ENABLED=0 go test ./internal/quality \
  -run '^$' -bench '^BenchmarkEndToEndDailyCorpus$' -benchmem
```

One benchmark operation is one pass over the tracked daily-STEM corpus:

- 92 formulas;
- 51 TeX and 41 Typst;
- 2,252 source bytes;
- four effective limit configurations.

The baseline reports both attempted cases and successful layouts:

- `corpora/s`: complete corpus passes per second;
- `cases/s`: attempted formulas per second;
- `items/success`, `hits/success`, and `path-seg/success`: output volume per
  successful layout;
- `items/case`, `hits/case`, and `path-seg/case`: output volume divided by all
  attempted formulas;
- `successes/case` and `successes/corpus`: successful-layout rate;
- `layout-errors/case`: expected font or output-limit failures.

The `/case` and `/success` denominators differ deliberately. A successful
layout count excludes reviewed layout failures, while throughput metrics still
need to account for the work spent attempting those cases.

## Engine Strategies

The benchmark compares:

- `parse-only`: parse and validate, without layout;
- `e2e/caller-engine`: engines prepared outside the timer and reused;
- `e2e/package-cache-warm`: the bounded package cache pre-filled outside the
  timer;
- `e2e/package-cache-cold`: package cache cleared at the start of each corpus
  pass;
- `e2e/no-cache`: a new engine constructed for every formula.

Use `caller-engine` for normal application performance. The package-cache rows
measure the convenience API, and `no-cache` is a worst-case comparison.

## Constrained Display Storage

```sh
CGO_ENABLED=0 go test ./internal/quality \
  -run '^$' -bench '^BenchmarkEndToEndConstrainedDailyCorpus$' -benchmem
```

This focused benchmark selects the daily corpus's constrained display cases
and distinguishes canonical storage from range references:

- `parent-*`: the canonical `Box.Items`/`Box.Hits` storage;
- `line-range-*` and `tag-range-*`: records selected from that canonical
  storage by ranges;
- `physical-*`: actual retained item and path storage;
- `item-storage-x` and `path-storage-x`: physical storage divided by canonical
  parent storage. The range-backed contract keeps both ratios at `1.0`.

Use this benchmark when reviewing constrained-layout or tag changes; timing
alone will not reveal accidental reintroduction of duplicate retained memory.

## Tracked Parser Corpora

```sh
CGO_ENABLED=0 go test ./internal/quality \
  -run '^$' -bench '^BenchmarkParseTrackedCorpora$' -benchmem
```

This benchmark runs every tracked upstream corpus through the public parse
boundary. Each sub-benchmark reports its exact case count, source bytes, and
reviewed diagnostics. The diagnostic counts matter: a parser benchmark is not
an all-clean workload merely because every case has a reviewed outcome.

## Interpreting Allocations

`B/op` and `allocs/op` include parser, validation, layout, and output
construction. A speed improvement that moves work outside the timed operation
or caches output across benchmark iterations is not comparable. Every corpus
pass must construct fresh expressions and boxes.

## Profiling

Generate CPU and allocation profiles around the caller-owned pipeline:

```sh
CGO_ENABLED=0 go test ./internal/quality \
  -run '^$' \
  -bench 'BenchmarkEndToEndDailyCorpus/e2e/caller-engine$' \
  -benchtime=500x -cpuprofile=/tmp/untie.cpu -memprofile=/tmp/untie.mem

go tool pprof -top /tmp/untie.cpu
go tool pprof -top -alloc_space /tmp/untie.mem
```

Profile changes against the same commit series and corpus. Treat a smaller
allocation count as useful only when corpus time is also unchanged or lower.
